package bookings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertCancellationTestBooking(
	t *testing.T,
	pool *pgxpool.Pool,
	slotID uuid.UUID,
	customerID uuid.UUID,
	status BookingStatus,
) uuid.UUID {
	t.Helper()

	bookingID := uuid.New()

	_, err := pool.Exec(context.Background(), `
		INSERT INTO bookings (id, slot_id, customer_id, status)
		VALUES ($1, $2, $3, $4)
	`, bookingID, slotID, customerID, status)
	if err != nil {
		t.Fatalf("insert test booking: %v", err)
	}

	return bookingID
}

func TestCancelBookingByCustomer(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(48*time.Hour),
		30*time.Minute,
		"open",
	)

	bookingID := insertCancellationTestBooking(
		t, pool, slotID, customerID, StatusConfirmed,
	)

	booking, err := repo.CancelByCustomer(
		context.Background(), bookingID, customerID,
	)
	if err != nil {
		t.Fatalf("cancel booking: %v", err)
	}

	if booking.Status != StatusCancelled {
		t.Fatalf("expected cancelled status, got %q", booking.Status)
	}

	if booking.ID != bookingID {
		t.Fatalf("expected booking ID %s, got %s", bookingID, booking.ID)
	}
}

func TestCancelBookingCannotCancelAnotherCustomersBooking(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	ownerID := newBookingTestUser(t, pool, "customer")
	otherCustomerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(48*time.Hour),
		30*time.Minute,
		"open",
	)

	bookingID := insertCancellationTestBooking(
		t, pool, slotID, ownerID, StatusConfirmed,
	)

	_, err := repo.CancelByCustomer(
		context.Background(), bookingID, otherCustomerID,
	)

	if !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("expected ErrBookingNotFound, got %v", err)
	}
}

func TestCancelBookingRejectsNonConfirmedStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status BookingStatus
	}{
		{name: "already cancelled", status: StatusCancelled},
		{name: "completed", status: StatusCompleted},
		{name: "no show", status: StatusNoShow},
		{name: "pending", status: StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newBookingTestPool(t)
			repo := &Repository{db: pool}

			providerID := createBookingTestProvider(t, pool)
			customerID := newBookingTestUser(t, pool, "customer")

			slotID := createTestAvailabilitySlot(
				t, pool, providerID,
				time.Now().UTC().Add(48*time.Hour),
				30*time.Minute,
				"open",
			)

			bookingID := insertCancellationTestBooking(
				t, pool, slotID, customerID, tt.status,
			)

			_, err := repo.CancelByCustomer(
				context.Background(), bookingID, customerID,
			)

			if !errors.Is(err, ErrBookingNotCancellable) {
				t.Fatalf("expected ErrBookingNotCancellable, got %v", err)
			}
		})
	}
}

func TestCancelledSlotCanBeBookedByAnotherCustomer(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	firstCustomerID := newBookingTestUser(t, pool, "customer")
	secondCustomerID := newBookingTestUser(t, pool, "customer")

	slotID := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(48*time.Hour),
		30*time.Minute,
		"open",
	)

	bookingID := insertCancellationTestBooking(
		t, pool, slotID, firstCustomerID, StatusConfirmed,
	)

	_, err := repo.CancelByCustomer(
		context.Background(), bookingID, firstCustomerID,
	)
	if err != nil {
		t.Fatalf("cancel first booking: %v", err)
	}

	newBooking, err := repo.Create(
		context.Background(),
		secondCustomerID,
		CreateBookingInput{SlotID: slotID},
	)
	if err != nil {
		t.Fatalf("book previously cancelled slot: %v", err)
	}

	if newBooking.Status != StatusConfirmed {
		t.Fatalf("expected confirmed status, got %q", newBooking.Status)
	}

	if newBooking.CustomerID != secondCustomerID {
		t.Fatalf("expected customer %s, got %s",
			secondCustomerID, newBooking.CustomerID)
	}
}

func TestCancelBookingRacesWithConcurrentBookings(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	firstCustomerID := newBookingTestUser(t, pool, "customer")

	const concurrentAttempts = 20

	slotID := createTestAvailabilitySlot(
		t,
		pool,
		providerID,
		time.Now().UTC().Add(48*time.Hour),
		30*time.Minute,
		"open",
	)

	originalBookingID := insertCancellationTestBooking(
		t,
		pool,
		slotID,
		firstCustomerID,
		StatusConfirmed,
	)

	type result struct {
		booking *Booking
		err     error
	}

	start := make(chan struct{})
	results := make(chan result, concurrentAttempts)
	cancelResult := make(chan error, 1)

	// Start cancellation concurrently with the new booking attempts.
	go func() {
		<-start

		_, err := repo.CancelByCustomer(
			context.Background(),
			originalBookingID,
			firstCustomerID,
		)
		cancelResult <- err
	}()

	// Start concurrent attempts from different customers.
	for i := 0; i < concurrentAttempts; i++ {
		customerID := newBookingTestUser(t, pool, "customer")

		go func(customerID uuid.UUID) {
			<-start

			booking, err := repo.Create(
				context.Background(),
				customerID,
				CreateBookingInput{SlotID: slotID},
			)

			results <- result{
				booking: booking,
				err:     err,
			}
		}(customerID)
	}

	// Release all goroutines at approximately the same time.
	close(start)

	var successfulBookings int
	var expectedBookingFailures int

	for i := 0; i < concurrentAttempts; i++ {
		res := <-results

		if res.err == nil {
			successfulBookings++
			continue
		}

		if errors.Is(res.err, ErrSlotBooked) {
			expectedBookingFailures++
			continue
		}

		t.Errorf("unexpected booking error: %v", res.err)
	}

	if err := <-cancelResult; err != nil {
		t.Fatalf("cancellation failed: %v", err)
	}

	if successfulBookings > 1 {
		t.Fatalf(
			"expected at most one successful booking, got %d",
			successfulBookings,
		)
	}

	if successfulBookings+expectedBookingFailures != concurrentAttempts {
		t.Fatalf(
			"expected %d booking outcomes, got %d",
			concurrentAttempts,
			successfulBookings+expectedBookingFailures,
		)
	}

	// Verify the final database state.
	var activeCount int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM bookings
		WHERE slot_id = $1
		  AND status IN ('pending', 'confirmed')
	`, slotID).Scan(&activeCount)

	if err != nil {
		t.Fatalf("count active bookings: %v", err)
	}

	if activeCount != 1 {
		t.Fatalf("expected exactly one active booking, got %d", activeCount)
	}

	// Verify the original booking was cancelled.
	var originalStatus BookingStatus
	err = pool.QueryRow(context.Background(), `
		SELECT status
		FROM bookings
		WHERE id = $1
	`, originalBookingID).Scan(&originalStatus)

	if err != nil {
		t.Fatalf("read original booking status: %v", err)
	}

	if originalStatus != StatusCancelled {
		t.Fatalf(
			"expected original booking to be cancelled, got %q",
			originalStatus,
		)
	}

	t.Logf(
		"Concurrent attempts: %d, successful: %d, rejected: %d",
		concurrentAttempts,
		successfulBookings,
		expectedBookingFailures,
	)
}
