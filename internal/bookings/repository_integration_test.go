package bookings

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConcurrentBooking(t *testing.T) {
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
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	repo := NewRepository(pool)

	// Create unique test records.
	suffix := uuid.NewString()

	var customerID, providerID, slotID uuid.UUID

	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'test_hash', 'customer')
		RETURNING id
	`, fmt.Sprintf("customer-%s@test.com", suffix)).Scan(&customerID)
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'test_hash', 'provider')
		RETURNING id
	`, fmt.Sprintf("provider-%s@test.com", suffix)).Scan(&providerID)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO provider_profiles (user_id, display_name)
		VALUES ($1, $2)
	`, providerID, "Concurrency Test Provider")
	if err != nil {
		t.Fatalf("create provider profile: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO availability_slots (provider_id, starts_at, ends_at)
		VALUES ($1, NOW() + INTERVAL '2 days',
			NOW() + INTERVAL '2 days 1 hour')
		RETURNING id
	`, providerID).Scan(&slotID)
	if err != nil {
		t.Fatalf("create availability slot: %v", err)
	}

	// Clean up records after the test.
	defer func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM bookings WHERE slot_id = $1`, slotID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM availability_slots WHERE id = $1`, slotID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM provider_profiles WHERE user_id = $1`, providerID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE id IN ($1, $2)`, customerID, providerID)
	}()

	const workers = 20

	var (
		successCount  atomic.Int32
		conflictCount atomic.Int32
		errorCount    atomic.Int32
	)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			// All goroutines wait here until released.
			<-start

			_, err := repo.Create(ctx, customerID, CreateBookingInput{
				SlotID: slotID,
			})

			switch {
			case err == nil:
				successCount.Add(1)

			case errors.Is(err, ErrSlotBooked):
				conflictCount.Add(1)

			default:
				errorCount.Add(1)
				t.Errorf("unexpected booking error: %v", err)
			}
		}()
	}

	// Release all 20 goroutines at once.
	close(start)
	wg.Wait()

	successes := successCount.Load()
	conflicts := conflictCount.Load()
	failures := errorCount.Load()

	t.Logf("Total requests: %d", workers)
	t.Logf("Successful bookings: %d", successes)
	t.Logf("Conflicts: %d", conflicts)
	t.Logf("Unexpected errors: %d", failures)

	if successes != 1 {
		t.Errorf("expected exactly 1 successful booking, got %d", successes)
	}

	if conflicts != workers-1 {
		t.Errorf("expected %d conflicts, got %d", workers-1, conflicts)
	}

	if failures != 0 {
		t.Errorf("expected 0 unexpected errors, got %d", failures)
	}

	// Independently verify the database state.
	var activeBookings int

	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bookings
		WHERE slot_id = $1
		  AND status IN ('pending', 'confirmed')
	`, slotID).Scan(&activeBookings)

	if err != nil {
		t.Fatalf("verify active bookings: %v", err)
	}

	if activeBookings != 1 {
		t.Errorf("expected exactly 1 active booking in database, got %d",
			activeBookings)
	}
}
