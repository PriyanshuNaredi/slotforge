package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// insertDirectBooking creates a booking row straight in the database, which
// is the only way to attach one to a past or closed slot: repo.Create
// rejects both.
func insertDirectBooking(
	t *testing.T,
	pool *pgxpool.Pool,
	slotID uuid.UUID,
	customerID uuid.UUID,
	notes string,
) uuid.UUID {
	t.Helper()

	bookingID := uuid.New()

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO bookings (id, slot_id, customer_id, status, notes)
		VALUES ($1, $2, $3, 'confirmed', $4)
	`, bookingID, slotID, customerID, notes); err != nil {
		t.Fatalf("insert direct booking: %v", err)
	}

	return bookingID
}

// testUserEmail reads back the generated email of a test user.
func testUserEmail(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
) string {
	t.Helper()

	var email string

	if err := pool.QueryRow(
		context.Background(),
		`SELECT email FROM users WHERE id = $1`,
		userID,
	).Scan(&email); err != nil {
		t.Fatalf("read test user email: %v", err)
	}

	return email
}

func TestGetBookingDetailCustomerAuthorization(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customerA := newBookingTestUser(t, pool, "customer")
	customerB := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(24*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"open",
	)

	created, err := repo.Create(context.Background(), customerA, CreateBookingInput{
		SlotID: slotID,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	// The owning customer gets the full detail.
	detail, err := repo.GetByIDForUser(context.Background(), created.ID, customerA, false)
	if err != nil {
		t.Fatalf("owner retrieval: %v", err)
	}

	if detail.ID != created.ID || detail.SlotID != slotID || detail.CustomerID != customerA {
		t.Errorf("detail identity mismatch: %+v", detail)
	}

	if detail.Status != StatusConfirmed {
		t.Errorf("expected status confirmed, got %s", detail.Status)
	}

	if detail.ProviderID != providerID {
		t.Errorf("expected provider %s, got %s", providerID, detail.ProviderID)
	}

	if !strings.HasPrefix(detail.ProviderName, "Booking Test Provider ") {
		t.Errorf("expected joined provider display name, got %q", detail.ProviderName)
	}

	if detail.ProviderTimezone != "UTC" {
		t.Errorf("expected provider timezone UTC, got %q", detail.ProviderTimezone)
	}

	slotStart := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Minute)
	if !detail.StartsAt.Equal(slotStart) {
		t.Errorf("expected starts_at %s, got %s", slotStart, detail.StartsAt)
	}

	if !detail.EndsAt.Equal(slotStart.Add(30 * time.Minute)) {
		t.Errorf("expected ends_at %s, got %s", slotStart.Add(30*time.Minute), detail.EndsAt)
	}

	if detail.SlotStatus != "open" {
		t.Errorf("expected slot status open, got %q", detail.SlotStatus)
	}

	if want := testUserEmail(t, pool, customerA); detail.CustomerEmail != want {
		t.Errorf("expected customer email %q, got %q", want, detail.CustomerEmail)
	}

	// Another customer gets a 404-equivalent, not a rejection detail.
	_, err = repo.GetByIDForUser(context.Background(), created.ID, customerB, false)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for other customer, got %v", err)
	}

	// A missing booking behaves the same way.
	_, err = repo.GetByIDForUser(context.Background(), uuid.New(), customerA, false)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for missing booking, got %v", err)
	}
}

func TestGetBookingDetailProviderAuthorization(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerA := createBookingTestProvider(t, pool)
	providerB := createBookingTestProvider(t, pool)
	customer := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerA,
		time.Now().UTC().Add(48*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"open",
	)

	created, err := repo.Create(context.Background(), customer, CreateBookingInput{
		SlotID: slotID,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	// The provider who owns the slot can read the booking.
	detail, err := repo.GetByIDForUser(context.Background(), created.ID, providerA, false)
	if err != nil {
		t.Fatalf("owning provider retrieval: %v", err)
	}

	if detail.ProviderID != providerA || detail.CustomerID != customer {
		t.Errorf("detail identity mismatch: %+v", detail)
	}

	if want := testUserEmail(t, pool, customer); detail.CustomerEmail != want {
		t.Errorf("expected customer email %q, got %q", want, detail.CustomerEmail)
	}

	// A different provider is denied, same as a missing booking.
	_, err = repo.GetByIDForUser(context.Background(), created.ID, providerB, false)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for other provider, got %v", err)
	}
}

func TestGetBookingDetailHistoricalSlots(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customer := newBookingTestUser(t, pool, "customer")

	// A slot entirely in the past.
	pastSlot := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(-24*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"open",
	)

	pastBooking := insertDirectBooking(t, pool, pastSlot, customer, "past appointment")

	detail, err := repo.GetByIDForUser(context.Background(), pastBooking, customer, false)
	if err != nil {
		t.Fatalf("past appointment retrieval: %v", err)
	}

	if !detail.StartsAt.Before(time.Now()) {
		t.Errorf("expected a past starts_at, got %s", detail.StartsAt)
	}

	if detail.SlotStatus != "open" {
		t.Errorf("expected slot status open, got %q", detail.SlotStatus)
	}

	// A closed slot in the future.
	closedSlot := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(72*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"closed",
	)

	closedBooking := insertDirectBooking(t, pool, closedSlot, customer, "closed slot booking")

	detail, err = repo.GetByIDForUser(context.Background(), closedBooking, customer, false)
	if err != nil {
		t.Fatalf("closed slot retrieval: %v", err)
	}

	if detail.SlotStatus != "closed" {
		t.Errorf("expected slot status closed, got %q", detail.SlotStatus)
	}

	if detail.Notes == nil || *detail.Notes != "closed slot booking" {
		t.Errorf("expected notes to round-trip, got %v", detail.Notes)
	}
}

func TestGetBookingDetailSensitiveFields(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customer := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(96*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"open",
	)

	created, err := repo.Create(context.Background(), customer, CreateBookingInput{
		SlotID: slotID,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	detail, err := repo.GetByIDForUser(context.Background(), created.ID, customer, false)
	if err != nil {
		t.Fatalf("owner retrieval: %v", err)
	}

	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}

	for _, forbidden := range []string{
		"password_hash", "role", "is_active", "email",
	} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("response must not contain %q", forbidden)
		}
	}

	providerEmail := testUserEmail(t, pool, providerID)
	if strings.Contains(string(raw), providerEmail) {
		t.Error("provider email must not appear in the response")
	}

	if strings.Contains(string(raw), "test-hash") {
		t.Error("password hash material must not appear in the response")
	}

	for _, required := range []string{
		"id", "status", "created_at", "starts_at", "ends_at",
		"provider_display_name", "provider_timezone", "customer_email",
	} {
		if _, ok := payload[required]; !ok {
			t.Errorf("response must contain %q", required)
		}
	}
}

func TestGetBookingDetailRouteAuth(t *testing.T) {
	router, manager := newWiredRouter(t, &mockBookingService{})

	// Unauthenticated requests are rejected by the middleware chain.
	response, _ := doWiredRequest(
		t, router, http.MethodGet,
		"/api/v1/bookings/"+uuid.NewString(),
		nil,
		"",
	)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", response.Code)
	}

	// A malformed booking ID is a 400, not a 404.
	token, err := manager.GenerateAccessToken(uuid.New(), "customer")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	response, _ = doWiredRequest(
		t, router, http.MethodGet, "/api/v1/bookings/not-a-uuid",
		nil,
		token,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid booking ID, got %d", response.Code)
	}
}
