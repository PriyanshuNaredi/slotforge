package providers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/priyanshunaredi/slotforge/internal/bookings"
)

func TestConcurrentBookingAndSlotClosing(t *testing.T) {
	repo, pool := newIntegrationRepository(t)
	bookingRepo := bookings.NewRepository(pool)

	providerID := createTestProvider(t, pool)
	ctx := context.Background()

	// Create a customer shared across the independent booking attempts.
	var customerID uuid.UUID

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, 'test_hash', 'customer')
		RETURNING id
	`, fmt.Sprintf("race-customer-%s@test.com", uuid.NewString())).
		Scan(&customerID)

	if err != nil {
		t.Fatalf("create test customer: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer cancel()

		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM bookings WHERE customer_id = $1`, customerID)

		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM users WHERE id = $1`, customerID)
	})

	const races = 20

	// Create 20 separate, non-overlapping slots.
	slotIDs := make([]uuid.UUID, races)

	baseTime := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Minute)

	for i := 0; i < races; i++ {
		start := baseTime.Add(time.Duration(i) * time.Hour)

		slot, err := repo.CreateSlot(ctx, providerID, CreateSlotInput{
			StartsAt: start,
			EndsAt:   start.Add(30 * time.Minute),
		})
		if err != nil {
			t.Fatalf("create test slot %d: %v", i, err)
		}

		slotIDs[i] = slot.ID
	}

	type operationResult struct {
		index     int
		operation string
		err       error
	}

	startSignal := make(chan struct{})
	results := make(chan operationResult, races*2)

	var wg sync.WaitGroup

	// Launch one booking request and one close request per slot.
	for i, slotID := range slotIDs {
		wg.Add(2)

		go func(index int, id uuid.UUID) {
			defer wg.Done()

			// Wait until all 40 operations are ready.
			<-startSignal

			_, err := bookingRepo.Create(
				ctx,
				customerID,
				bookings.CreateBookingInput{
					SlotID: id,
				},
			)

			results <- operationResult{
				index:     index,
				operation: "booking",
				err:       err,
			}
		}(i, slotID)

		go func(index int, id uuid.UUID) {
			defer wg.Done()

			<-startSignal

			err := repo.CloseSlot(ctx, providerID, id)

			results <- operationResult{
				index:     index,
				operation: "close",
				err:       err,
			}
		}(i, slotID)
	}

	// Release all 40 operations simultaneously.
	close(startSignal)

	wg.Wait()
	close(results)

	bookingErrors := make([]error, races)
	closeErrors := make([]error, races)

	for result := range results {
		switch result.operation {
		case "booking":
			bookingErrors[result.index] = result.err
		case "close":
			closeErrors[result.index] = result.err
		}
	}

	var bookingWins, closeWins int

	// Verify the outcome of every independent race.
	for i, slotID := range slotIDs {
		bookingErr := bookingErrors[i]
		closeErr := closeErrors[i]

		var status string

		err := pool.QueryRow(ctx, `
			SELECT status
			FROM availability_slots
			WHERE id = $1
		`, slotID).Scan(&status)

		if err != nil {
			t.Fatalf("read slot %d status: %v", i, err)
		}

		var activeBookings int

		err = pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM bookings
			WHERE slot_id = $1
			  AND status IN ('pending', 'confirmed')
		`, slotID).Scan(&activeBookings)

		if err != nil {
			t.Fatalf("count active bookings for slot %d: %v", i, err)
		}

		switch {
		case bookingErr == nil:
			// Booking acquired the lock first.
			if !errors.Is(closeErr, ErrSlotHasActiveBooking) {
				t.Errorf(
					"race %d: expected close to fail with ErrSlotHasActiveBooking, got %v",
					i, closeErr,
				)
			}

			if status != SlotStatusOpen {
				t.Errorf("race %d: expected open slot, got %q", i, status)
			}

			if activeBookings != 1 {
				t.Errorf("race %d: expected 1 active booking, got %d", i, activeBookings)
			}

			bookingWins++

		case errors.Is(bookingErr, bookings.ErrSlotClosed):
			// Provider acquired the lock first.
			if closeErr != nil {
				t.Errorf("race %d: expected close to succeed, got %v", i, closeErr)
			}

			if status != SlotStatusClosed {
				t.Errorf("race %d: expected closed slot, got %q", i, status)
			}

			if activeBookings != 0 {
				t.Errorf("race %d: expected 0 active bookings, got %d", i, activeBookings)
			}

			closeWins++

		default:
			t.Errorf("race %d: unexpected booking error: %v", i, bookingErr)
		}
	}

	t.Logf("Total races: %d", races)
	t.Logf("Booking operations won: %d", bookingWins)
	t.Logf("Slot closing operations won: %d", closeWins)

	if bookingWins+closeWins != races {
		t.Errorf(
			"expected %d valid race outcomes, got %d",
			races, bookingWins+closeWins,
		)
	}
}
