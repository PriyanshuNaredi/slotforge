package providers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// newProviderCacheRedis connects to a dedicated Redis database index
// (15) so developer data in DB 0 is never touched, and cleans up all
// slot-search keys afterwards.
func newProviderCacheRedis(t *testing.T) *redis.Client {
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

		var cursor uint64

		for {
			keys, next, err := client.Scan(
				cleanupCtx, cursor, "slot-search:*", 100,
			).Result()
			if err != nil {
				break
			}

			if len(keys) > 0 {
				_ = client.Del(cleanupCtx, keys...).Err()
			}

			cursor = next
			if cursor == 0 {
				break
			}
		}

		_ = client.Close()
	})

	return client
}

func seedSlotSearchKey(t *testing.T, client *redis.Client) string {
	t.Helper()

	key := "slot-search:v1:provider=" + uuid.NewString() +
		":from=none:to=none:limit=20:offset=0"

	if err := client.Set(context.Background(), key, "stale", time.Minute).Err(); err != nil {
		t.Fatalf("seed cache key: %v", err)
	}

	return key
}

func keyExists(t *testing.T, client *redis.Client, key string) bool {
	t.Helper()

	exists, err := client.Exists(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("check key existence: %v", err)
	}

	return exists == 1
}

// TestProviderMutationsInvalidateSlotSearchCache verifies slot
// mutations invalidate the public search cache, and failed mutations
// leave it alone.
func TestProviderMutationsInvalidateSlotSearchCache(t *testing.T) {
	pool := newProvidersTestPool(t)
	client := newProviderCacheRedis(t)

	service := NewService(NewRepository(pool), client)

	providerID := createTestProvider(t, pool)

	ctx := context.Background()

	start := time.Now().UTC().Add(260 * time.Hour).Truncate(time.Minute)

	t.Run("slot creation invalidates", func(t *testing.T) {
		key := seedSlotSearchKey(t, client)

		if _, err := service.CreateSlot(ctx, providerID, CreateSlotInput{
			StartsAt: start,
			EndsAt:   start.Add(30 * time.Minute),
		}); err != nil {
			t.Fatalf("create slot: %v", err)
		}

		if keyExists(t, client, key) {
			t.Error("expected cache invalidation after slot creation")
		}
	})

	t.Run("slot closure invalidates", func(t *testing.T) {
		slot, err := service.CreateSlot(ctx, providerID, CreateSlotInput{
			StartsAt: start.Add(time.Hour),
			EndsAt:   start.Add(90 * time.Minute),
		})
		if err != nil {
			t.Fatalf("create slot: %v", err)
		}

		key := seedSlotSearchKey(t, client)

		if err := service.CloseSlot(ctx, providerID, slot.ID); err != nil {
			t.Fatalf("close slot: %v", err)
		}

		if keyExists(t, client, key) {
			t.Error("expected cache invalidation after slot closure")
		}
	})

	t.Run("failed slot closure does not invalidate", func(t *testing.T) {
		key := seedSlotSearchKey(t, client)

		if err := service.CloseSlot(ctx, providerID, uuid.New()); err != ErrSlotNotFound {
			t.Fatalf("expected ErrSlotNotFound, got %v", err)
		}

		if !keyExists(t, client, key) {
			t.Error("failed mutation must keep the cache")
		}
	})

	t.Run("booking status update invalidates", func(t *testing.T) {
		slot, err := service.CreateSlot(ctx, providerID, CreateSlotInput{
			StartsAt: start.Add(2 * time.Hour),
			EndsAt:   start.Add(150 * time.Minute),
		})
		if err != nil {
			t.Fatalf("create slot: %v", err)
		}

		customerID := createProvidersTestUser(t, pool, "customer")

		var bookingID uuid.UUID

		err = pool.QueryRow(ctx, `
			INSERT INTO bookings (slot_id, customer_id, status)
			VALUES ($1, $2, 'pending')
			RETURNING id
		`, slot.ID, customerID).Scan(&bookingID)
		if err != nil {
			t.Fatalf("insert booking: %v", err)
		}

		key := seedSlotSearchKey(t, client)

		if _, err := service.UpdateProviderBookingStatus(
			ctx, providerID, bookingID, "confirmed",
		); err != nil {
			t.Fatalf("update booking status: %v", err)
		}

		if keyExists(t, client, key) {
			t.Error("expected cache invalidation after status update")
		}
	})
}
