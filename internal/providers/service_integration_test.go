package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newProvidersTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// createProvidersTestUser creates a bare user (no profile yet) and
// cleans up the profile, slots, and dependent bookings afterwards.
func createProvidersTestUser(
	t *testing.T,
	pool *pgxpool.Pool,
	role string,
) uuid.UUID {
	t.Helper()

	ctx := context.Background()

	userID := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, is_active)
		VALUES ($1, $2, 'test-hash', $3, TRUE)
	`, userID, fmt.Sprintf("%s-%s@example.com", role, uuid.NewString()), role)
	if err != nil {
		t.Fatalf("create test %s: %v", role, err)
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
		`, userID)

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM availability_slots WHERE provider_id = $1
		`, userID)

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM provider_profiles WHERE user_id = $1
		`, userID)

		_, _ = pool.Exec(cleanupCtx, `
			DELETE FROM users WHERE id = $1
		`, userID)
	})

	return userID
}

func TestCreateProfileService(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	userID := createProvidersTestUser(t, pool, "provider")

	bio := "Board-certified test engineer"

	profile, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: "  Dr. Test  ",
		Bio:         &bio,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if profile.UserID != userID {
		t.Errorf("expected user %s, got %s", userID, profile.UserID)
	}

	// Whitespace is trimmed but the value is stored as given otherwise.
	if profile.DisplayName != "Dr. Test" {
		t.Errorf("expected trimmed display name, got %q", profile.DisplayName)
	}

	// Empty timezone defaults to UTC.
	if profile.Timezone != "UTC" {
		t.Errorf("expected default UTC timezone, got %q", profile.Timezone)
	}

	// A second profile for the same user is rejected.
	if _, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: "Second Profile",
	}); !errors.Is(err, ErrProfileExists) {
		t.Fatalf("expected ErrProfileExists, got %v", err)
	}
}

func TestCreateProfileServiceValidation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	userID := createProvidersTestUser(t, pool, "provider")

	if _, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: "a",
	}); err == nil {
		t.Error("expected a 1-character display name to be rejected")
	}

	if _, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: strings.Repeat("x", 151),
	}); err == nil {
		t.Error("expected a 151-character display name to be rejected")
	}

	if _, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: "Valid Name",
		Timezone:    "Mars/Olympus_Mons",
	}); err == nil {
		t.Error("expected an invalid timezone to be rejected")
	}

	if _, err := service.CreateProfile(context.Background(), uuid.Nil, CreateProfileInput{
		DisplayName: "Valid Name",
	}); err == nil {
		t.Error("expected a nil user ID to be rejected")
	}
}

func TestCreateProfileServiceExplicitTimezone(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	userID := createProvidersTestUser(t, pool, "provider")

	profile, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: "Dr. TimeZone",
		Timezone:    "Europe/Berlin",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if profile.Timezone != "Europe/Berlin" {
		t.Errorf("expected Europe/Berlin, got %q", profile.Timezone)
	}
}

func TestGetMyProfileService(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	userID := createProvidersTestUser(t, pool, "provider")

	if _, err := service.GetMyProfile(context.Background(), userID); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound before creation, got %v", err)
	}

	created, err := service.CreateProfile(context.Background(), userID, CreateProfileInput{
		DisplayName: "Dr. Found",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	found, err := service.GetMyProfile(context.Background(), userID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	if found.UserID != created.UserID || found.DisplayName != "Dr. Found" {
		t.Errorf("unexpected profile: %+v", found)
	}
}

func TestCreateSlotService(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createTestProvider(t, pool)

	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	slot, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start,
		EndsAt:   start.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	if slot.ID == uuid.Nil {
		t.Error("expected a slot ID")
	}

	if slot.ProviderID != providerID {
		t.Errorf("expected provider %s, got %s", providerID, slot.ProviderID)
	}

	if slot.Status != SlotStatusOpen {
		t.Errorf("expected open status, got %q", slot.Status)
	}

	// Adjacent slots share a boundary but do not overlap ([) ranges).
	adjacent, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start.Add(30 * time.Minute),
		EndsAt:   start.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("adjacent slot should be allowed: %v", err)
	}

	if adjacent.ID == slot.ID {
		t.Error("expected a new slot")
	}
}

func TestCreateSlotServiceValidation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createTestProvider(t, pool)

	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	cases := []struct {
		name string
		in   CreateSlotInput
	}{
		{
			"end before start",
			CreateSlotInput{StartsAt: start, EndsAt: start.Add(-time.Minute)},
		},
		{
			"end equals start",
			CreateSlotInput{StartsAt: start, EndsAt: start},
		},
	}

	for _, tc := range cases {
		if _, err := service.CreateSlot(context.Background(), providerID, tc.in); !errors.Is(err, ErrInvalidSlotTime) {
			t.Errorf("%s: expected ErrInvalidSlotTime, got %v", tc.name, err)
		}
	}

	// Overlapping with an existing open slot is rejected.
	_, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start,
		EndsAt:   start.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("seed slot: %v", err)
	}

	if _, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start.Add(10 * time.Minute),
		EndsAt:   start.Add(40 * time.Minute),
	}); !errors.Is(err, ErrSlotOverlap) {
		t.Fatalf("expected ErrSlotOverlap, got %v", err)
	}
}

func TestListAndRetrieveSlots(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createTestProvider(t, pool)

	base := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	first, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: base.Add(time.Hour),
		EndsAt:   base.Add(90 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create first slot: %v", err)
	}

	second, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: base,
		EndsAt:   base.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create second slot: %v", err)
	}

	slots, err := service.ListSlots(context.Background(), providerID)
	if err != nil {
		t.Fatalf("list slots: %v", err)
	}

	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}

	// Ordered by start time ascending.
	if slots[0].ID != second.ID || slots[1].ID != first.ID {
		t.Error("expected slots ordered by starts_at ascending")
	}

	// Ownership: the provider can read their own slot.
	got, err := service.GetSlot(context.Background(), providerID, first.ID)
	if err != nil {
		t.Fatalf("get own slot: %v", err)
	}

	if got.ID != first.ID {
		t.Errorf("expected slot %s, got %s", first.ID, got.ID)
	}

	// Another provider must not see it (404 semantics, not 403).
	otherProvider := createTestProvider(t, pool)

	if _, err := service.GetSlot(
		context.Background(), otherProvider, first.ID,
	); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("expected ErrSlotNotFound for foreign slot, got %v", err)
	}

	if _, err := service.GetSlot(
		context.Background(), providerID, uuid.New(),
	); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("expected ErrSlotNotFound for unknown slot, got %v", err)
	}
}

func TestCloseSlotService(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createTestProvider(t, pool)

	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	slot, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start,
		EndsAt:   start.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	// Closing succeeds and updates the status.
	if err := service.CloseSlot(context.Background(), providerID, slot.ID); err != nil {
		t.Fatalf("close slot: %v", err)
	}

	closed, err := service.GetSlot(context.Background(), providerID, slot.ID)
	if err != nil {
		t.Fatalf("get slot: %v", err)
	}

	if closed.Status != SlotStatusClosed {
		t.Errorf("expected closed status, got %q", closed.Status)
	}

	// Closing an already-closed slot is idempotent.
	if err := service.CloseSlot(context.Background(), providerID, slot.ID); err != nil {
		t.Fatalf("second close should be a no-op, got %v", err)
	}

	// Foreign provider cannot close someone else's slot.
	otherProvider := createTestProvider(t, pool)

	if err := service.CloseSlot(
		context.Background(), otherProvider, slot.ID,
	); !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("expected ErrSlotNotFound, got %v", err)
	}
}

func TestCloseSlotServiceWithActiveBooking(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createTestProvider(t, pool)

	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	slot, err := service.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start,
		EndsAt:   start.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	customerID := createProvidersTestUser(t, pool, "customer")

	_, err = pool.Exec(context.Background(), `
		INSERT INTO bookings (slot_id, customer_id, status)
		VALUES ($1, $2, 'confirmed')
	`, slot.ID, customerID)
	if err != nil {
		t.Fatalf("insert booking: %v", err)
	}

	if err := service.CloseSlot(context.Background(), providerID, slot.ID); !errors.Is(err, ErrSlotHasActiveBooking) {
		t.Fatalf("expected ErrSlotHasActiveBooking, got %v", err)
	}

	var status string

	err = pool.QueryRow(context.Background(), `
		SELECT status FROM availability_slots WHERE id = $1
	`, slot.ID).Scan(&status)
	if err != nil {
		t.Fatalf("load slot status: %v", err)
	}

	if status != SlotStatusOpen {
		t.Errorf("slot must stay open when closing fails, got %q", status)
	}
}
