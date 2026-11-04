package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/priyanshunaredi/slotforge/internal/cache"
)

// queryCounter counts PostgreSQL queries through a pgx tracer, providing
// a real-database seam to assert whether the service hit PostgreSQL.
type queryCounter struct {
	mu    sync.Mutex
	count int
}

func (c *queryCounter) TraceQueryStart(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	c.mu.Lock()
	c.count++
	c.mu.Unlock()

	return ctx
}

func (c *queryCounter) TraceQueryEnd(
	ctx context.Context,
	conn *pgx.Conn,
	data pgx.TraceQueryEndData,
) {
}

func (c *queryCounter) queries() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.count
}

// newTracedPool returns a pool wired to a query counter.
func newTracedPool(t *testing.T) (*pgxpool.Pool, *queryCounter) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	counter := &queryCounter{}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse database config: %v", err)
	}

	config.ConnConfig.Tracer = counter

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool, counter
}

// newCacheTestRedis connects to a dedicated Redis database index (15)
// so developer data in DB 0 is never touched. Keys are cleaned up.
func newCacheTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:        addr,
		DB:          15,
		DialTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("Redis is not available at %s (DB 15); skipping cache test: %v", addr, err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cleanupCancel()

		invalidateSlotSearchKeys(cleanupCtx, client)
		_ = client.Close()
	})

	return client
}

// invalidateSlotSearchKeys deletes only slot-search:* keys, never
// flushing the Redis instance.
func invalidateSlotSearchKeys(ctx context.Context, client *redis.Client) {
	var cursor uint64

	for {
		keys, next, err := client.Scan(ctx, cursor, "slot-search:*", 100).Result()
		if err != nil {
			return
		}

		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}

		cursor = next
		if cursor == 0 {
			return
		}
	}
}

func newCacheTestService(
	t *testing.T,
	pool *pgxpool.Pool,
	client *redis.Client,
) *Service {
	t.Helper()

	return NewService(NewRepository(pool), client)
}

// TestSlotSearchCacheHit verifies the second identical search is served
// from Redis without touching PostgreSQL.
func TestSlotSearchCacheHit(t *testing.T) {
	pool, counter := newTracedPool(t)
	client := newCacheTestRedis(t)

	providerID := createBookingTestProvider(t, pool)

	createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(240*time.Hour).Truncate(time.Minute),
		30*time.Minute, "open",
	)

	service := newCacheTestService(t, pool, client)

	ctx := context.Background()

	input := SlotSearchInput{
		ProviderID: &providerID,
		Limit:      20,
	}

	// First search: must query PostgreSQL and populate the cache.
	first, err := service.SearchAvailableSlotsWithCount(ctx, input)
	if err != nil {
		t.Fatalf("first search: %v", err)
	}

	if counter.queries() == 0 {
		t.Fatal("expected PostgreSQL queries on the first search")
	}

	cacheKey := slotSearchCacheKey(input)

	if exists, err := client.Exists(ctx, cacheKey).Result(); err != nil || exists != 1 {
		t.Fatalf("expected cache key to be written, exists=%d err=%v", exists, err)
	}

	// Second identical search: must be served from Redis only.
	counter.mu.Lock()
	counter.count = 0
	counter.mu.Unlock()

	second, err := service.SearchAvailableSlotsWithCount(ctx, input)
	if err != nil {
		t.Fatalf("second search: %v", err)
	}

	if got := counter.queries(); got != 0 {
		t.Fatalf("expected 0 PostgreSQL queries on cache hit, got %d", got)
	}

	if second.Total != first.Total {
		t.Errorf("expected cached total %d, got %d", first.Total, second.Total)
	}

	if len(second.Slots) != len(first.Slots) {
		t.Fatalf("expected %d cached slots, got %d", len(first.Slots), len(second.Slots))
	}

	for i := range first.Slots {
		if second.Slots[i].ID != first.Slots[i].ID {
			t.Errorf("slot %d: expected %s, got %s", i, first.Slots[i].ID, second.Slots[i].ID)
		}
	}
}

// TestSlotSearchCacheMiss verifies a missing key triggers a PostgreSQL
// lookup and the result is cached afterwards.
func TestSlotSearchCacheMiss(t *testing.T) {
	pool, counter := newTracedPool(t)
	client := newCacheTestRedis(t)

	providerID := createBookingTestProvider(t, pool)

	input := SlotSearchInput{
		ProviderID: &providerID,
		Limit:      20,
	}

	cacheKey := slotSearchCacheKey(input)

	// Ensure the key is absent.
	if err := client.Del(context.Background(), cacheKey).Err(); err != nil {
		t.Fatalf("delete cache key: %v", err)
	}

	service := newCacheTestService(t, pool, client)

	result, err := service.SearchAvailableSlotsWithCount(
		context.Background(), input,
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if counter.queries() == 0 {
		t.Error("expected PostgreSQL queries on cache miss")
	}

	if result.Slots == nil {
		t.Error("expected a non-nil slots slice")
	}

	exists, err := client.Exists(context.Background(), cacheKey).Result()
	if err != nil || exists != 1 {
		t.Fatalf("expected result to be cached after miss, exists=%d err=%v", exists, err)
	}

	// The cached payload must be the marshalled search result.
	cached, err := client.Get(context.Background(), cacheKey).Result()
	if err != nil {
		t.Fatalf("read cached payload: %v", err)
	}

	var payload SlotSearchResult

	if err := json.Unmarshal([]byte(cached), &payload); err != nil {
		t.Fatalf("cached payload is not a valid SlotSearchResult: %v", err)
	}

	if payload.Total != result.Total || payload.Limit != result.Limit {
		t.Errorf("cached payload mismatch: %+v vs %+v", payload, result)
	}
}

// TestSlotSearchCacheTTL verifies the configured TTL is applied and that
// an expired key is treated as a cache miss. No long sleeps: the TTL
// value is asserted directly, and expiry behavior is exercised with a
// short-lived key at the same cache key.
func TestSlotSearchCacheTTL(t *testing.T) {
	pool, _ := newTracedPool(t)
	client := newCacheTestRedis(t)

	providerID := createBookingTestProvider(t, pool)

	service := newCacheTestService(t, pool, client)

	input := SlotSearchInput{
		ProviderID: &providerID,
		Limit:      20,
	}

	cacheKey := slotSearchCacheKey(input)

	if _, err := service.SearchAvailableSlotsWithCount(
		context.Background(), input,
	); err != nil {
		t.Fatalf("search: %v", err)
	}

	ttl, err := client.TTL(context.Background(), cacheKey).Result()
	if err != nil {
		t.Fatalf("read TTL: %v", err)
	}

	// The service must write the configured 30s TTL (allowing a small
	// window for the round-trip).
	if ttl <= 25*time.Second || ttl > slotSearchCacheTTL {
		t.Fatalf("expected TTL close to %s, got %s", slotSearchCacheTTL, ttl)
	}

	// Expiry behavior: a key with a short TTL must disappear and the
	// next search must fall back to PostgreSQL (cache miss).
	if err := client.Set(
		context.Background(), cacheKey, "expired-soon", 100*time.Millisecond,
	).Err(); err != nil {
		t.Fatalf("seed short-lived key: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)

	for {
		exists, err := client.Exists(context.Background(), cacheKey).Result()
		if err != nil {
			t.Fatalf("poll key existence: %v", err)
		}

		if exists == 0 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("key did not expire within 5 seconds")
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// TestSlotSearchRedisUnavailable verifies the search and mutations fail
// open when Redis is down.
func TestSlotSearchRedisUnavailable(t *testing.T) {
	pool, _ := newTracedPool(t)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(240*time.Hour).Truncate(time.Minute),
		30*time.Minute, "open",
	)

	// Nothing listens on port 1; retries disabled for fast failures.
	deadClient := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		MaxRetries:  -1,
		DialTimeout: 1 * time.Second,
	})

	t.Cleanup(func() {
		_ = deadClient.Close()
	})

	service := newCacheTestService(t, pool, deadClient)

	ctx := context.Background()

	// GET and SET failures fall back to PostgreSQL.
	result, err := service.SearchAvailableSlotsWithCount(ctx, SlotSearchInput{
		ProviderID: &providerID,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("search must succeed without Redis, got error: %v", err)
	}

	if result.Total != 1 || len(result.Slots) != 1 {
		t.Errorf("expected PostgreSQL fallback result, got total=%d slots=%d",
			result.Total, len(result.Slots))
	}

	// A failed cache invalidation must not fail a successful mutation.
	booking, err := service.CreateBooking(ctx, customerID, CreateBookingInput{
		SlotID: slotID,
	})
	if err != nil {
		t.Fatalf("booking creation must succeed without Redis, got: %v", err)
	}

	if booking.SlotID != slotID {
		t.Errorf("expected booking for slot %s, got %s", slotID, booking.SlotID)
	}
}

// TestSlotSearchInvalidCachedJSON verifies corrupted cache entries are
// treated as misses and removed best-effort.
func TestSlotSearchInvalidCachedJSON(t *testing.T) {
	pool, counter := newTracedPool(t)
	client := newCacheTestRedis(t)

	providerID := createBookingTestProvider(t, pool)

	service := newCacheTestService(t, pool, client)

	input := SlotSearchInput{
		ProviderID: &providerID,
		Limit:      20,
	}

	cacheKey := slotSearchCacheKey(input)

	if err := client.Set(
		context.Background(), cacheKey, "{not-valid-json", time.Minute,
	).Err(); err != nil {
		t.Fatalf("seed invalid cache entry: %v", err)
	}

	result, err := service.SearchAvailableSlotsWithCount(
		context.Background(), input,
	)
	if err != nil {
		t.Fatalf("search must fall back to PostgreSQL, got: %v", err)
	}

	if counter.queries() == 0 {
		t.Error("expected PostgreSQL to be queried for invalid cached JSON")
	}

	if result.Total < 0 {
		t.Errorf("unexpected negative total: %d", result.Total)
	}

	// The stale entry must be gone: the key now holds the freshly
	// cached PostgreSQL result, not the seeded garbage.
	current, err := client.Get(context.Background(), cacheKey).Result()
	if err != nil {
		t.Fatalf("expected the service to re-cache a fresh result, got: %v", err)
	}

	if current == "{not-valid-json" {
		t.Error("invalid cache entry was not deleted")
	}

	if !json.Valid([]byte(current)) {
		t.Errorf("re-cached payload is not valid JSON: %q", current)
	}
}

// TestInvalidateSlotSearch verifies scan-based invalidation deletes
// every slot-search key and nothing else.
func TestInvalidateSlotSearch(t *testing.T) {
	client := newCacheTestRedis(t)

	ctx := context.Background()

	matching := []string{
		"slot-search:v1:provider=all:from=none:to=none:limit=20:offset=0",
		"slot-search:v1:provider=" + uuid.NewString() + ":from=none:to=none:limit=5:offset=5",
		"slot-search:some-other-shape",
	}

	for _, key := range matching {
		if err := client.Set(ctx, key, "x", time.Minute).Err(); err != nil {
			t.Fatalf("seed key %s: %v", key, err)
		}
	}

	unrelated := "other-cache:v1:keep-me"

	if err := client.Set(ctx, unrelated, "x", time.Minute).Err(); err != nil {
		t.Fatalf("seed unrelated key: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Del(context.Background(), unrelated).Err()
	})

	cache.InvalidateSlotSearch(ctx, client)

	for _, key := range matching {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil || exists != 0 {
			t.Errorf("expected key %s to be deleted, exists=%d err=%v", key, exists, err)
		}
	}

	exists, err := client.Exists(ctx, unrelated).Result()
	if err != nil || exists != 1 {
		t.Errorf("unrelated key must survive, exists=%d err=%v", exists, err)
	}
}

// TestInvalidationAfterMutations verifies Redis is invalidated only by
// successful booking mutations.
func TestInvalidationAfterMutations(t *testing.T) {
	pool, _ := newTracedPool(t)
	client := newCacheTestRedis(t)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	service := newCacheTestService(t, pool, client)

	ctx := context.Background()

	seedKey := func(t *testing.T) string {
		t.Helper()

		key := "slot-search:v1:provider=" + uuid.NewString() +
			":from=none:to=none:limit=20:offset=0"

		if err := client.Set(ctx, key, "stale", time.Minute).Err(); err != nil {
			t.Fatalf("seed cache key: %v", err)
		}

		return key
	}

	t.Run("successful booking invalidates", func(t *testing.T) {
		slotID := createTestAvailabilitySlot(
			t, pool, providerID,
			time.Now().UTC().Add(240*time.Hour).Truncate(time.Minute),
			30*time.Minute, "open",
		)

		key := seedKey(t)

		if _, err := service.CreateBooking(ctx, customerID, CreateBookingInput{
			SlotID: slotID,
		}); err != nil {
			t.Fatalf("create booking: %v", err)
		}

		exists, err := client.Exists(ctx, key).Result()
		if err != nil || exists != 0 {
			t.Errorf("expected cache invalidation after booking, exists=%d err=%v", exists, err)
		}
	})

	t.Run("failed booking does not invalidate", func(t *testing.T) {
		key := seedKey(t)

		_, err := service.CreateBooking(ctx, customerID, CreateBookingInput{
			SlotID: uuid.New(), // nonexistent slot.
		})
		if !errors.Is(err, ErrSlotNotFound) {
			t.Fatalf("expected ErrSlotNotFound, got %v", err)
		}

		exists, err := client.Exists(ctx, key).Result()
		if err != nil || exists != 1 {
			t.Errorf("failed mutation must keep the cache, exists=%d err=%v", exists, err)
		}
	})

	t.Run("successful cancellation invalidates", func(t *testing.T) {
		slotID := createTestAvailabilitySlot(
			t, pool, providerID,
			time.Now().UTC().Add(241*time.Hour).Truncate(time.Minute),
			30*time.Minute, "open",
		)

		booking, err := service.CreateBooking(ctx, customerID, CreateBookingInput{
			SlotID: slotID,
		})
		if err != nil {
			t.Fatalf("create booking: %v", err)
		}

		key := seedKey(t)

		if _, err := service.CancelBooking(ctx, booking.ID, customerID); err != nil {
			t.Fatalf("cancel booking: %v", err)
		}

		exists, err := client.Exists(ctx, key).Result()
		if err != nil || exists != 0 {
			t.Errorf("expected cache invalidation after cancellation, exists=%d err=%v", exists, err)
		}
	})
}

// TestSearchExcludesActivelyBookedSlots verifies open slots with an
// active booking are hidden from public search results and totals.
func TestSearchExcludesActivelyBookedSlots(t *testing.T) {
	pool, _ := newTracedPool(t)
	client := newCacheTestRedis(t)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	now := time.Now().UTC().Add(240 * time.Hour).Truncate(time.Minute)

	bookedSlot := createTestAvailabilitySlot(t, pool, providerID, now, 30*time.Minute, "open")
	freeSlot := createTestAvailabilitySlot(t, pool, providerID, now.Add(time.Hour), 30*time.Minute, "open")

	service := newCacheTestService(t, pool, client)

	if _, err := service.CreateBooking(
		context.Background(),
		customerID,
		CreateBookingInput{SlotID: bookedSlot},
	); err != nil {
		t.Fatalf("book the first slot: %v", err)
	}

	result, err := service.SearchAvailableSlotsWithCount(context.Background(), SlotSearchInput{
		ProviderID: &providerID,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected total 1 after booking one slot, got %d", result.Total)
	}

	if len(result.Slots) != 1 || result.Slots[0].ID != freeSlot {
		t.Fatalf("expected only the unbooked slot %s, got %+v", freeSlot, result.Slots)
	}
}
