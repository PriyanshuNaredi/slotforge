package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TestPublicSlotDiscovery seeds a provider with open, closed, and expired
// slots and verifies filtering, pagination, and counts end to end.
func TestPublicSlotDiscovery(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)

	ctx := context.Background()

	now := time.Now().UTC()

	// Future open slots that should be visible.
	futureA := createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(24*time.Hour).Truncate(time.Minute), 30*time.Minute, "open",
	)
	futureB := createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(25*time.Hour).Truncate(time.Minute), 30*time.Minute, "open",
	)

	// Started in the past: must never appear.
	createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(-time.Hour).Truncate(time.Minute), 30*time.Minute, "open",
	)

	// Closed: must never appear even in the future.
	createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(26*time.Hour).Truncate(time.Minute), 30*time.Minute, "closed",
	)

	// All assertions are scoped to this test's provider so the shared
	// dev database's other slots cannot affect the result.
	input := SlotSearchInput{Limit: 20, ProviderID: &providerID}

	slots, err := repo.SearchAvailableSlots(ctx, input)
	if err != nil {
		t.Fatalf("search slots: %v", err)
	}

	if len(slots) != 2 {
		t.Fatalf("expected 2 open future slots, got %d", len(slots))
	}

	ids := map[uuid.UUID]bool{futureA: true, futureB: true}

	for _, slot := range slots {
		if !ids[slot.ID] {
			t.Errorf("unexpected slot %s in results", slot.ID)
		}

		if slot.ProviderID != providerID {
			t.Errorf("slot %s belongs to another provider", slot.ID)
		}

		if slot.DisplayName == "" {
			t.Error("expected provider display name to be joined in")
		}

		if !slot.StartsAt.After(now) {
			t.Errorf("slot %s starts in the past", slot.ID)
		}
	}

	// Ordering must be ascending by start time.
	if !slots[0].StartsAt.Before(slots[1].StartsAt) {
		t.Error("expected slots ordered by starts_at ascending")
	}

	total, err := repo.CountAvailableSlots(ctx, input)
	if err != nil {
		t.Fatalf("count slots: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	// Provider filter with a foreign provider must return nothing.
	otherProvider := uuid.New()

	slots, err = repo.SearchAvailableSlots(ctx, SlotSearchInput{
		ProviderID: &otherProvider,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("search with foreign provider: %v", err)
	}

	if len(slots) != 0 {
		t.Errorf("expected no slots for unknown provider, got %d", len(slots))
	}

	// From/To window that only contains the second slot.
	from := now.Add(24*time.Hour + 45*time.Minute)
	to := now.Add(25*time.Hour + 30*time.Minute)

	slots, err = repo.SearchAvailableSlots(ctx, SlotSearchInput{
		ProviderID: &providerID,
		From:       &from,
		To:         &to,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("search with window: %v", err)
	}

	if len(slots) != 1 || slots[0].ID != futureB {
		t.Fatalf("expected only slot B in window, got %d slots", len(slots))
	}

	// Pagination must not change the total.
	firstPage, err := repo.SearchAvailableSlots(ctx, SlotSearchInput{
		ProviderID: &providerID, Limit: 1, Offset: 0,
	})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}

	if len(firstPage) != 1 {
		t.Fatalf("expected 1 slot on the first page, got %d", len(firstPage))
	}

	secondPage, err := repo.SearchAvailableSlots(ctx, SlotSearchInput{
		ProviderID: &providerID, Limit: 1, Offset: 1,
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}

	if len(secondPage) != 1 || secondPage[0].ID == firstPage[0].ID {
		t.Error("expected distinct slots across pages")
	}

	// An empty window returns an empty slice and zero total.
	past := now.Add(-24 * time.Hour)
	pastEnd := past.Add(time.Hour)

	slots, err = repo.SearchAvailableSlots(ctx, SlotSearchInput{
		ProviderID: &providerID,
		From:       &past,
		To:         &pastEnd,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("search in past window: %v", err)
	}

	if slots == nil {
		t.Error("expected a non-nil empty slice")
	}

	if len(slots) != 0 {
		t.Errorf("expected empty result set, got %d", len(slots))
	}
}

func TestPublicSlotDetail(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)

	ctx := context.Background()
	now := time.Now().UTC()

	openSlot := createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(24*time.Hour).Truncate(time.Minute), 30*time.Minute, "open",
	)
	closedSlot := createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(25*time.Hour).Truncate(time.Minute), 30*time.Minute, "closed",
	)
	expiredSlot := createTestAvailabilitySlot(
		t, pool, providerID,
		now.Add(-2*time.Hour).Truncate(time.Minute), 30*time.Minute, "open",
	)

	slot, err := repo.GetAvailableSlotByID(ctx, openSlot)
	if err != nil {
		t.Fatalf("open slot should be visible: %v", err)
	}

	if slot.ID != openSlot || slot.ProviderID != providerID {
		t.Errorf("unexpected slot data: %+v", slot)
	}

	for _, slotID := range []uuid.UUID{closedSlot, expiredSlot, uuid.New()} {
		if _, err := repo.GetAvailableSlotByID(ctx, slotID); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("slot %s: expected pgx.ErrNoRows, got %v", slotID, err)
		}
	}
}

// TestPublicDiscoveryHandlerDoesNotLeakProviderData runs the public
// listing handler against the real service and inspects the JSON body.
func TestPublicDiscoveryHandlerDoesNotLeakProviderData(t *testing.T) {
	pool := newBookingTestPool(t)

	providerID := createBookingTestProvider(t, pool)

	// Bio content that must never reach the response.
	_, err := pool.Exec(context.Background(), `
		UPDATE provider_profiles
		SET bio = 'SECRET-BIO-CONTENT'
		WHERE user_id = $1
	`, providerID)
	if err != nil {
		t.Fatalf("set bio: %v", err)
	}

	createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(24*time.Hour).Truncate(time.Minute),
		30*time.Minute, "open",
	)

	service := NewService(NewRepository(pool))

	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(service).RegisterPublicRoutes(router.Group(""))

	request := httptest.NewRequest(
		http.MethodGet,
		"/slots?provider_id="+providerID.String(),
		nil,
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	body := response.Body.String()

	for _, secret := range []string{
		"SECRET-BIO-CONTENT",
		"password_hash",
		"\"email\"",
	} {
		if strings.Contains(body, secret) {
			t.Errorf("public response leaks %q: %s", secret, body)
		}
	}

	var parsed struct {
		Slots []map[string]any `json:"slots"`
		Total int64            `json:"total"`
	}

	if err := json.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(parsed.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(parsed.Slots))
	}

	slot := parsed.Slots[0]

	for _, key := range []string{"id", "provider_id", "provider_name", "timezone", "starts_at", "ends_at"} {
		if _, exists := slot[key]; !exists {
			t.Errorf("expected key %q in slot JSON", key)
		}
	}

	if parsed.Total < 1 {
		t.Errorf("expected total >= 1, got %d", parsed.Total)
	}
}

// Compile-time guard that the real service satisfies the handler interface.
var _ BookingService = (*Service)(nil)
