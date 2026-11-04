package bookings

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newBookingTestPool(t *testing.T) *pgxpool.Pool {
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

// newBookingTestUser creates a user with the given role and cleans it up.
func newBookingTestUser(
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

		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM bookings WHERE customer_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
	})

	return userID
}

// createBookingTestProvider creates a provider user with a profile and
// cleans up the profile, its slots, and dependent bookings.
func createBookingTestProvider(
	t *testing.T,
	pool *pgxpool.Pool,
) uuid.UUID {
	t.Helper()

	providerID := newBookingTestUser(t, pool, "provider")

	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO provider_profiles (user_id, display_name, timezone)
		VALUES ($1, $2, 'UTC')
	`, providerID, "Booking Test Provider "+uuid.NewString())
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
	})

	return providerID
}

// createTestAvailabilitySlot inserts a slot directly with the given status.
func createTestAvailabilitySlot(
	t *testing.T,
	pool *pgxpool.Pool,
	providerID uuid.UUID,
	start time.Time,
	duration time.Duration,
	status string,
) uuid.UUID {
	t.Helper()

	slotID := uuid.New()

	_, err := pool.Exec(context.Background(), `
		INSERT INTO availability_slots (
			id, provider_id, starts_at, ends_at, status
		)
		VALUES ($1, $2, $3, $4, $5)
	`, slotID, providerID, start, start.Add(duration), status)
	if err != nil {
		t.Fatalf("create test slot: %v", err)
	}

	return slotID
}
