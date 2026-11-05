package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createDiscoveryProvider inserts a provider user plus a public profile
// with a chosen display name, mirroring the cleanup of createTestProvider.
func createDiscoveryProvider(
	t *testing.T,
	pool *pgxpool.Pool,
	displayName string,
) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	suffix := uuid.NewString()

	var providerID uuid.UUID

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'test_hash', 'provider')
		RETURNING id
	`, fmt.Sprintf("provider-%s@test.com", suffix)).Scan(&providerID)

	if err != nil {
		t.Fatalf("create test provider: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO provider_profiles (user_id, display_name, timezone)
		VALUES ($1, $2, 'UTC')
	`, providerID, displayName)

	if err != nil {
		t.Fatalf("create provider profile: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer cancel()

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM bookings
			WHERE slot_id IN (
				SELECT id FROM availability_slots WHERE provider_id = $1
			)
		`, providerID)

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM availability_slots WHERE provider_id = $1
		`, providerID)

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM provider_profiles WHERE user_id = $1
		`, providerID)

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM users WHERE id = $1
		`, providerID)
	})

	return providerID
}

// createDiscoveryUser inserts a user without any profile row.
func createDiscoveryUser(
	t *testing.T,
	pool *pgxpool.Pool,
	role string,
) uuid.UUID {
	t.Helper()

	var userID uuid.UUID

	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'test_hash', $2)
		RETURNING id
	`, fmt.Sprintf("user-%s@test.com", uuid.NewString()), role).Scan(&userID)

	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE id = $1`, userID)
	})

	return userID
}

func discoveryName(prefix string) string {
	return prefix + "-" + strings.SplitN(uuid.NewString(), "-", 2)[0]
}

func TestListPublicProvidersPaginationAndTotal(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	prefix := discoveryName("disc")
	alpha := createDiscoveryProvider(t, pool, prefix+" Alpha")
	beta := createDiscoveryProvider(t, pool, prefix+" Beta")
	gamma := createDiscoveryProvider(t, pool, prefix+" Gamma")

	ctx := context.Background()

	list := func(limit, offset int) *ProviderListResult {
		t.Helper()

		items, err := repo.ListPublicProviders(ctx, ProviderListInput{
			Search: prefix,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			t.Fatalf("list public providers: %v", err)
		}

		total, err := repo.CountPublicProviders(ctx, ProviderListInput{
			Search: prefix,
		})
		if err != nil {
			t.Fatalf("count public providers: %v", err)
		}

		return &ProviderListResult{
			Items:  items,
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	page := list(2, 0)
	if page.Total != 3 {
		t.Fatalf("expected total 3, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items with limit 2, got %d", len(page.Items))
	}

	full := list(100, 0)
	if full.Total != page.Total {
		t.Fatalf("total must not depend on page size: %d vs %d",
			full.Total, page.Total)
	}
	if len(full.Items) != 3 {
		t.Fatalf("expected 3 items with limit 100, got %d", len(full.Items))
	}

	last := list(2, 2)
	if len(last.Items) != 1 {
		t.Fatalf("expected 1 item on the last page, got %d", len(last.Items))
	}

	seen := map[uuid.UUID]bool{
		page.Items[0].ID: true,
		page.Items[1].ID: true,
		last.Items[0].ID: true,
	}
	for _, id := range []uuid.UUID{alpha, beta, gamma} {
		if !seen[id] {
			t.Fatalf("provider %s missing from paginated pages", id)
		}
	}

	// Two queries must return the same order (stable ordering).
	again := list(100, 0)
	for i := range again.Items {
		if again.Items[i].ID != full.Items[i].ID {
			t.Fatal("expected stable ordering between identical queries")
		}
	}
}

func TestListPublicProvidersOrdersNewestProfileFirst(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	prefix := discoveryName("order")
	old := createDiscoveryProvider(t, pool, prefix+" Old")
	newest := createDiscoveryProvider(t, pool, prefix+" New")

	// Make profile creation times deterministic instead of relying on
	// clock precision between two inserts.
	_, err := pool.Exec(context.Background(), `
		UPDATE provider_profiles
		SET created_at = NOW() - INTERVAL '2 days'
		WHERE user_id = $1
	`, old)
	if err != nil {
		t.Fatalf("backdate old profile: %v", err)
	}

	_, err = pool.Exec(context.Background(), `
		UPDATE provider_profiles
		SET created_at = NOW() - INTERVAL '1 day'
		WHERE user_id = $1
	`, newest)
	if err != nil {
		t.Fatalf("backdate new profile: %v", err)
	}

	items, err := repo.ListPublicProviders(context.Background(), ProviderListInput{
		Search: prefix,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list public providers: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != newest || items[1].ID != old {
		t.Fatal("expected newest profile first with deterministic ordering")
	}
}

func TestListPublicProvidersSearchCaseInsensitive(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	prefix := discoveryName("case")
	alpha := createDiscoveryProvider(t, pool, prefix+" ALPHA Specialist")
	beta := createDiscoveryProvider(t, pool, prefix+" Beta Specialist")

	ctx := context.Background()

	lowercase, err := repo.ListPublicProviders(ctx, ProviderListInput{
		Search: strings.ToLower(prefix + " alpha"),
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list with lowercase search: %v", err)
	}

	if len(lowercase) != 1 || lowercase[0].ID != alpha {
		t.Fatalf("expected only ALPHA provider for lowercase search, got %d items",
			len(lowercase))
	}

	mixed, err := repo.ListPublicProviders(ctx, ProviderListInput{
		Search: prefix + " beta SPECIALIST",
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list with mixed-case search: %v", err)
	}

	if len(mixed) != 1 || mixed[0].ID != beta {
		t.Fatalf("expected only Beta provider for mixed-case search, got %d items",
			len(mixed))
	}
}

func TestListPublicProvidersSearchEscapesWildcards(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	prefix := discoveryName("escape")
	createDiscoveryProvider(t, pool, prefix+" Ordinary")

	ctx := context.Background()

	// A bare '%' or '_' must match nothing: unescaped, both would match
	// every provider name.
	for _, wildcard := range []string{"%", "_"} {
		result, err := repo.ListPublicProviders(ctx, ProviderListInput{
			Search: wildcard,
			Limit:  10,
			Offset: 0,
		})
		if err != nil {
			t.Fatalf("list with wildcard search %q: %v", wildcard, err)
		}

		if len(result) != 0 {
			t.Fatalf("wildcard %q must not act as a pattern, matched %d providers",
				wildcard, len(result))
		}
	}
}

func TestGetPublicProviderDetail(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	name := discoveryName("detail")
	providerID := createDiscoveryProvider(t, pool, name)

	bio := "Public bio text"
	_, err := pool.Exec(context.Background(), `
		UPDATE provider_profiles SET bio = $2 WHERE user_id = $1
	`, providerID, bio)
	if err != nil {
		t.Fatalf("set bio: %v", err)
	}

	provider, err := repo.GetPublicProvider(context.Background(), providerID)
	if err != nil {
		t.Fatalf("get public provider: %v", err)
	}

	if provider.ID != providerID ||
		provider.DisplayName != name ||
		provider.Bio == nil || *provider.Bio != bio ||
		provider.Timezone != "UTC" {
		t.Fatalf("unexpected public profile: %+v", provider)
	}

	// The public DTO must not carry any private user fields.
	wire, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("marshal public provider: %v", err)
	}

	for _, leaked := range []string{"email", "password", "role", "is_active", "updated_at"} {
		if strings.Contains(string(wire), leaked) {
			t.Errorf("public provider JSON must not contain %q: %s", leaked, wire)
		}
	}
}

func TestGetPublicProviderMissingAndProfileless(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	profileless := createDiscoveryUser(t, pool, "provider")

	ctx := context.Background()

	if _, err := repo.GetPublicProvider(ctx, uuid.New()); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound for unknown ID, got %v", err)
	}

	if _, err := repo.GetPublicProvider(
		ctx,
		profileless,
	); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound for provider without profile, got %v", err)
	}

	// A provider without a profile must never appear in the directory.
	prefix := discoveryName("profileless")
	withProfile := createDiscoveryProvider(t, pool, prefix+" Visible")

	items, err := repo.ListPublicProviders(ctx, ProviderListInput{
		Search: prefix,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list public providers: %v", err)
	}

	for _, item := range items {
		if item.ID == profileless {
			t.Fatal("profileless provider must not be listed")
		}
	}

	if len(items) != 1 || items[0].ID != withProfile {
		t.Fatalf("expected only the profiled provider, got %d items", len(items))
	}
}

func TestListPublicProvidersAvailabilityFilter(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	prefix := discoveryName("avail")

	// A: two overlapping-with-window slots; must appear exactly once.
	inWindow := createDiscoveryProvider(t, pool, prefix+" In Window")
	outWindow := createDiscoveryProvider(t, pool, prefix+" Out Window")
	booked := createDiscoveryProvider(t, pool, prefix+" Booked")

	windowFrom := time.Now().UTC().Add(47 * time.Hour).Truncate(time.Minute)
	windowTo := time.Now().UTC().Add(49 * time.Hour).Truncate(time.Minute)

	slotBase := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)

	ctx := context.Background()

	for _, slot := range []struct {
		provider uuid.UUID
		start    time.Time
		end      time.Time
	}{
		{inWindow, slotBase, slotBase.Add(30 * time.Minute)},
		{inWindow, slotBase.Add(30 * time.Minute), slotBase.Add(60 * time.Minute)},
		{outWindow, slotBase.Add(2 * time.Hour), slotBase.Add(150 * time.Minute)},
		{booked, slotBase.Add(3 * time.Hour), slotBase.Add(210 * time.Minute)},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO availability_slots (provider_id, starts_at, ends_at)
			VALUES ($1, $2, $3)
		`, slot.provider, slot.start, slot.end); err != nil {
			t.Fatalf("insert availability slot: %v", err)
		}
	}

	// Book the booked provider's only slot.
	customerID := createDiscoveryUser(t, pool, "customer")

	var bookedSlotID uuid.UUID

	err := pool.QueryRow(ctx, `
		SELECT id FROM availability_slots WHERE provider_id = $1
	`, booked).Scan(&bookedSlotID)
	if err != nil {
		t.Fatalf("load booked slot: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO bookings (slot_id, customer_id, status)
		VALUES ($1, $2, 'confirmed')
	`, bookedSlotID, customerID); err != nil {
		t.Fatalf("insert booking: %v", err)
	}

	input := ProviderListInput{
		Search:        prefix,
		AvailableFrom: &windowFrom,
		AvailableTo:   &windowTo,
		Limit:         10,
		Offset:        0,
	}

	items, err := repo.ListPublicProviders(ctx, input)
	if err != nil {
		t.Fatalf("list with availability filter: %v", err)
	}

	if len(items) != 1 || items[0].ID != inWindow {
		t.Fatalf("expected only the in-window provider, got %d items", len(items))
	}

	total, err := repo.CountPublicProviders(ctx, input)
	if err != nil {
		t.Fatalf("count with availability filter: %v", err)
	}

	if total != 1 {
		t.Fatalf("expected filtered total 1, got %d", total)
	}

	// Without the window all three providers are listed.
	unfiltered, err := repo.ListPublicProviders(ctx, ProviderListInput{
		Search: prefix,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list without availability filter: %v", err)
	}

	if len(unfiltered) != 3 {
		t.Fatalf("expected 3 providers without the filter, got %d", len(unfiltered))
	}
}

func TestInactiveProvidersNotExposed(t *testing.T) {
	repo, pool := newIntegrationRepository(t)

	prefix := discoveryName("inactive")
	active := createDiscoveryProvider(t, pool, prefix+" Active")
	deactivated := createDiscoveryProvider(t, pool, prefix+" Deactivated")

	ctx := context.Background()

	// Give the deactivated provider a slot inside the test window too, so
	// the availability-filtered query path is exercised as well.
	slotBase := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	if _, err := pool.Exec(ctx, `
		INSERT INTO availability_slots (provider_id, starts_at, ends_at)
		VALUES ($1, $2, $3)
	`, deactivated, slotBase, slotBase.Add(30*time.Minute)); err != nil {
		t.Fatalf("insert availability slot: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE users SET is_active = FALSE WHERE id = $1
	`, deactivated); err != nil {
		t.Fatalf("deactivate provider: %v", err)
	}

	// General directory excludes the deactivated provider.
	items, err := repo.ListPublicProviders(ctx, ProviderListInput{
		Search: prefix,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("list public providers: %v", err)
	}

	if len(items) != 1 || items[0].ID != active {
		t.Fatalf("expected only the active provider, got %d items", len(items))
	}

	// Availability-filtered results apply the same rule.
	windowFrom := slotBase.Add(-1 * time.Hour)
	windowTo := slotBase.Add(1 * time.Hour)

	filtered, err := repo.ListPublicProviders(ctx, ProviderListInput{
		Search:        prefix,
		AvailableFrom: &windowFrom,
		AvailableTo:   &windowTo,
		Limit:         10,
		Offset:        0,
	})
	if err != nil {
		t.Fatalf("list with availability filter: %v", err)
	}

	if len(filtered) != 0 {
		t.Fatalf("expected the deactivated provider to be filtered out, got %d items", len(filtered))
	}

	// Detail returns the same 404 as an unknown provider.
	if _, err := repo.GetPublicProvider(
		ctx,
		deactivated,
	); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound for deactivated provider, got %v", err)
	}
}

// newPublicDiscoveryRouter builds the public discovery routes plus the
// static /providers/... route tree so gin conflicts between
// /providers/:id and /providers/profile are caught by registration.
func newPublicDiscoveryRouter(
	t *testing.T,
) (*gin.Engine, *Service, *pgxpool.Pool) {
	t.Helper()

	repo, pool := newIntegrationRepository(t)

	service := NewService(repo)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	api := router.Group("/api/v1")

	handler := NewHandler(service)
	handler.RegisterPublicRoutes(api)
	handler.RegisterRoutes(api.Group(""))

	return router, service, pool
}

func publicGet(
	t *testing.T,
	router *gin.Engine,
	path string,
) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	var body map[string]any

	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}

	return response, body
}

func TestPublicProviderHandlerDetail(t *testing.T) {
	router, _, pool := newPublicDiscoveryRouter(t)

	providerID := createDiscoveryProvider(t, pool, discoveryName("handler"))

	response, body := publicGet(
		t, router,
		"/api/v1/providers/"+providerID.String(),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body["id"] != providerID.String() {
		t.Fatalf("expected id %s, got %v", providerID, body["id"])
	}

	wire := response.Body.String()
	for _, leaked := range []string{"email", "password", "token"} {
		if strings.Contains(wire, leaked) {
			t.Errorf("public detail must not contain %q: %s", leaked, wire)
		}
	}

	missing, _ := publicGet(
		t, router,
		"/api/v1/providers/"+uuid.New().String(),
	)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown provider, got %d", missing.Code)
	}

	invalid, _ := publicGet(t, router, "/api/v1/providers/not-a-uuid")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid UUID, got %d", invalid.Code)
	}
}

func TestPublicProviderHandlerListing(t *testing.T) {
	router, _, pool := newPublicDiscoveryRouter(t)

	prefix := discoveryName("hlist")
	createDiscoveryProvider(t, pool, prefix+" One")
	createDiscoveryProvider(t, pool, prefix+" Two")

	response, body := publicGet(
		t, router,
		"/api/v1/providers?search="+prefix+"&limit=1&offset=0",
	)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body["total"] != float64(2) || body["limit"] != float64(1) || body["offset"] != float64(0) {
		t.Fatalf("unexpected pagination envelope: %s", response.Body.String())
	}

	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 item with limit 1, got %s", response.Body.String())
	}

	// The envelope must be items/total/limit/offset exactly.
	for key := range body {
		switch key {
		case "items", "total", "limit", "offset":
		default:
			t.Errorf("unexpected response key %q", key)
		}
	}
}

func TestPublicProviderHandlerValidation(t *testing.T) {
	router, _, _ := newPublicDiscoveryRouter(t)

	cases := []struct {
		name string
		path string
	}{
		{"limit zero", "/api/v1/providers?limit=0"},
		{"limit too high", "/api/v1/providers?limit=101"},
		{"limit not a number", "/api/v1/providers?limit=abc"},
		{"offset negative", "/api/v1/providers?offset=-1"},
		{"offset not a number", "/api/v1/providers?offset=x"},
		{"from without to", "/api/v1/providers?available_from=2026-01-01T00:00:00Z"},
		{"to without from", "/api/v1/providers?available_to=2026-01-02T00:00:00Z"},
		{"to before from", "/api/v1/providers?available_from=2026-01-02T00:00:00Z&available_to=2026-01-01T00:00:00Z"},
		{"from equal to", "/api/v1/providers?available_from=2026-01-01T00:00:00Z&available_to=2026-01-01T00:00:00Z"},
		{"bad from format", "/api/v1/providers?available_from=yesterday&available_to=2026-01-01T00:00:00Z"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response, _ := publicGet(t, router, testCase.path)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s",
					testCase.path, response.Code, response.Body.String())
			}
		})
	}

	valid, _ := publicGet(t, router, "/api/v1/providers")
	if valid.Code != http.StatusOK {
		t.Fatalf("expected 200 for plain listing, got %d", valid.Code)
	}
}

func TestPublicProviderHandlerAvailabilityFilter(t *testing.T) {
	router, _, pool := newPublicDiscoveryRouter(t)

	prefix := discoveryName("havail")
	providerID := createDiscoveryProvider(t, pool, prefix+" Available")

	slotBase := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO availability_slots (provider_id, starts_at, ends_at)
		VALUES ($1, $2, $3)
	`, providerID, slotBase, slotBase.Add(30*time.Minute)); err != nil {
		t.Fatalf("insert availability slot: %v", err)
	}

	from := slotBase.Add(-1 * time.Hour).Format(time.RFC3339)
	to := slotBase.Add(1 * time.Hour).Format(time.RFC3339)

	response, body := publicGet(
		t,
		router,
		fmt.Sprintf(
			"/api/v1/providers?search=%s&available_from=%s&available_to=%s",
			prefix, from, to,
		),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if body["total"] != float64(1) {
		t.Fatalf("expected total 1 with availability window, got %s", response.Body.String())
	}
}
