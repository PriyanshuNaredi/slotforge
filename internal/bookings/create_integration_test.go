package bookings

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCreateBookingSuccess(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(24*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"open",
	)

	notes := "Please ring the doorbell"

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{
		SlotID: slotID,
		Notes:  &notes,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	if booking.ID == uuid.Nil {
		t.Error("expected a booking ID")
	}

	if booking.SlotID != slotID {
		t.Errorf("expected slot %s, got %s", slotID, booking.SlotID)
	}

	if booking.CustomerID != customerID {
		t.Errorf("expected customer %s, got %s", customerID, booking.CustomerID)
	}

	if booking.Status != StatusConfirmed {
		t.Errorf("expected confirmed status, got %q", booking.Status)
	}

	if booking.Notes == nil || *booking.Notes != notes {
		t.Errorf("expected notes %q, got %v", notes, booking.Notes)
	}
}

func TestCreateBookingWithoutNotes(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(24*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"open",
	)

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{
		SlotID: slotID,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	if booking.Notes != nil {
		t.Errorf("expected nil notes, got %q", *booking.Notes)
	}
}

func TestCreateBookingNonexistentSlot(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	customerID := newBookingTestUser(t, pool, "customer")

	_, err := repo.Create(context.Background(), customerID, CreateBookingInput{
		SlotID: uuid.New(),
	})

	if !errors.Is(err, ErrSlotNotFound) {
		t.Fatalf("expected ErrSlotNotFound, got %v", err)
	}
}

func TestCreateBookingClosedSlot(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(24*time.Hour).Truncate(time.Minute),
		30*time.Minute,
		"closed",
	)

	_, err := repo.Create(context.Background(), customerID, CreateBookingInput{
		SlotID: slotID,
	})

	if !errors.Is(err, ErrSlotClosed) {
		t.Fatalf("expected ErrSlotClosed, got %v", err)
	}

	// No booking row must exist for the failed attempt.
	var count int

	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM bookings WHERE slot_id = $1
	`, slotID).Scan(&count)
	if err != nil {
		t.Fatalf("count bookings: %v", err)
	}

	if count != 0 {
		t.Errorf("expected no bookings for closed slot, got %d", count)
	}
}

func TestCreateBookingAlreadyBooked(t *testing.T) {
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

	_, err := repo.Create(context.Background(), customerA, CreateBookingInput{
		SlotID: slotID,
	})
	if err != nil {
		t.Fatalf("first booking: %v", err)
	}

	_, err = repo.Create(context.Background(), customerB, CreateBookingInput{
		SlotID: slotID,
	})

	if !errors.Is(err, ErrSlotBooked) {
		t.Fatalf("expected ErrSlotBooked, got %v", err)
	}
}

func TestGetBookingByIDForUser(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customerA := newBookingTestUser(t, pool, "customer")
	customerB := newBookingTestUser(t, pool, "customer")
	adminID := newBookingTestUser(t, pool, "admin")

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

	// The owner can retrieve it.
	found, err := repo.GetByIDForUser(
		context.Background(),
		created.ID,
		customerA,
		false,
	)
	if err != nil {
		t.Fatalf("owner retrieval: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("expected booking %s, got %s", created.ID, found.ID)
	}

	// Another customer must not see it.
	_, err = repo.GetByIDForUser(
		context.Background(),
		created.ID,
		customerB,
		false,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for other customer, got %v", err)
	}

	// A random ID must not resolve either.
	_, err = repo.GetByIDForUser(
		context.Background(),
		uuid.New(),
		customerA,
		false,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for nonexistent booking, got %v", err)
	}

	// Admins can retrieve any booking.
	adminView, err := repo.GetByIDForUser(
		context.Background(),
		created.ID,
		adminID,
		true,
	)
	if err != nil {
		t.Fatalf("admin retrieval: %v", err)
	}

	if adminView.ID != created.ID {
		t.Errorf("expected booking %s, got %s", created.ID, adminView.ID)
	}

	// A non-admin flag with a foreign customer must stay hidden.
	_, err = repo.GetByIDForUser(
		context.Background(),
		created.ID,
		adminID,
		false,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected non-admin to be denied, got %v", err)
	}
}

// TestConcurrentBookingRepeated runs the same-slot race several times on
// fresh slots so intermittent scheduling issues surface across repeats.
func TestConcurrentBookingRepeated(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := NewRepository(pool)

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	const (
		iterations = 3
		workers    = 10
	)

	for iteration := 0; iteration < iterations; iteration++ {
		t.Run(fmt.Sprintf("iteration_%d", iteration), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			slotID := createTestAvailabilitySlot(
				t,
				pool,
				providerID,
				time.Now().UTC().
					Add(48*time.Hour).
					Add(time.Duration(iteration)*time.Hour).
					Truncate(time.Minute),
				30*time.Minute,
				"open",
			)

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

			close(start)
			wg.Wait()

			if got := successCount.Load(); got != 1 {
				t.Errorf("expected exactly 1 success, got %d", got)
			}

			if got := conflictCount.Load(); got != workers-1 {
				t.Errorf("expected %d conflicts, got %d", workers-1, got)
			}

			if got := errorCount.Load(); got != 0 {
				t.Errorf("expected 0 unexpected errors, got %d", got)
			}

			var activeBookings int

			err := pool.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM bookings
				WHERE slot_id = $1
				  AND status IN ('pending', 'confirmed')
			`, slotID).Scan(&activeBookings)
			if err != nil {
				t.Fatalf("verify active bookings: %v", err)
			}

			if activeBookings != 1 {
				t.Errorf("expected 1 active booking in database, got %d", activeBookings)
			}
		})
	}
}
