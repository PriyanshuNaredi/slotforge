package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newIntegrationRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return NewRepository(pool), pool
}

func createTestProvider(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
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
	`, providerID, "Integration Test Provider")

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

func TestCreateOverlappingSlot(t *testing.T) {
	repo, pool := newIntegrationRepository(t)
	providerID := createTestProvider(t, pool)

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)

	_, err := repo.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start,
		EndsAt:   start.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create first slot: %v", err)
	}

	_, err = repo.CreateSlot(context.Background(), providerID, CreateSlotInput{
		StartsAt: start.Add(15 * time.Minute),
		EndsAt:   start.Add(45 * time.Minute),
	})

	if !errors.Is(err, ErrSlotOverlap) {
		t.Fatalf("expected ErrSlotOverlap, got %v", err)
	}
}

func TestCloseSlotWithActiveBooking(t *testing.T) {
	repo, pool := newIntegrationRepository(t)
	providerID := createTestProvider(t, pool)

	ctx := context.Background()
	suffix := uuid.NewString()

	var customerID uuid.UUID

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'test_hash', 'customer')
		RETURNING id
	`, fmt.Sprintf("customer-%s@test.com", suffix)).Scan(&customerID)

	if err != nil {
		t.Fatalf("create test customer: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE id = $1`, customerID)
	})

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)

	slot, err := repo.CreateSlot(ctx, providerID, CreateSlotInput{
		StartsAt: start,
		EndsAt:   start.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("create availability slot: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO bookings (slot_id, customer_id, status)
		VALUES ($1, $2, 'confirmed')
	`, slot.ID, customerID)

	if err != nil {
		t.Fatalf("create active booking: %v", err)
	}

	err = repo.CloseSlot(ctx, providerID, slot.ID)

	if !errors.Is(err, ErrSlotHasActiveBooking) {
		t.Fatalf("expected ErrSlotHasActiveBooking, got %v", err)
	}

	var status string

	err = pool.QueryRow(ctx, `
		SELECT status FROM availability_slots WHERE id = $1
	`, slot.ID).Scan(&status)

	if err != nil {
		t.Fatalf("check slot status: %v", err)
	}

	if status != SlotStatusOpen {
		t.Errorf("expected slot to remain open, got %q", status)
	}
}
