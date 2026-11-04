package providers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createStatusTestBooking(
	t *testing.T,
	pool *pgxpool.Pool,
	service *Service,
	providerID uuid.UUID,
	customerID uuid.UUID,
	status string,
	start time.Time,
) uuid.UUID {
	t.Helper()

	slot, err := service.CreateSlot(
		context.Background(),
		providerID,
		CreateSlotInput{
			StartsAt: start,
			EndsAt:   start.Add(30 * time.Minute),
		},
	)
	if err != nil {
		t.Fatalf("create status test slot: %v", err)
	}

	var bookingID uuid.UUID

	err = pool.QueryRow(context.Background(), `
        INSERT INTO bookings (slot_id, customer_id, status)
        VALUES ($1, $2, $3)
        RETURNING id
    `, slot.ID, customerID, status).Scan(&bookingID)

	if err != nil {
		t.Fatalf("create status test booking: %v", err)
	}

	return bookingID
}

func getBookingStatus(
	t *testing.T,
	pool *pgxpool.Pool,
	bookingID uuid.UUID,
) string {
	t.Helper()

	var status string

	err := pool.QueryRow(
		context.Background(),
		`SELECT status FROM bookings WHERE id = $1`,
		bookingID,
	).Scan(&status)

	if err != nil {
		t.Fatalf("get booking status: %v", err)
	}

	return status
}

func TestUpdateProviderBookingStatusTransitions(t *testing.T) {

	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	tests := []struct {
		name        string
		current     string
		next        string
		expectError error
	}{
		{
			name:    "confirmed to completed",
			current: "confirmed",
			next:    "completed",
		},
		{
			name:    "confirmed to cancelled",
			current: "confirmed",
			next:    "cancelled",
		},
		{
			name:    "confirmed to no_show",
			current: "confirmed",
			next:    "no_show",
		},
		{
			name:    "pending to confirmed",
			current: "pending",
			next:    "confirmed",
		},
		{
			name:    "pending to cancelled",
			current: "pending",
			next:    "cancelled",
		},
		{
			name:        "completed is terminal",
			current:     "completed",
			next:        "cancelled",
			expectError: ErrInvalidBookingTransition,
		},
		{
			name:        "cancelled is terminal",
			current:     "cancelled",
			next:        "confirmed",
			expectError: ErrInvalidBookingTransition,
		},
		{
			name:        "no_show is terminal",
			current:     "no_show",
			next:        "completed",
			expectError: ErrInvalidBookingTransition,
		},
		{
			name:        "confirmed cannot return to pending",
			current:     "confirmed",
			next:        "pending",
			expectError: ErrInvalidBookingStatus,
		},
	}
	base := time.Now().UTC().Add(240 * time.Hour).Truncate(time.Minute)

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookingID := createStatusTestBooking(
				t,
				pool,
				service,
				providerID,
				customerID,
				tt.current,
				base.Add(time.Duration(i)*time.Hour),
			)

			// Keep the rest of your existing test code here.

			booking, err := service.UpdateProviderBookingStatus(
				context.Background(),
				providerID,
				bookingID,
				tt.next,
			)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectError,
						err,
					)
				}

				if got := getBookingStatus(t, pool, bookingID); got != tt.current {
					t.Fatalf(
						"booking status changed unexpectedly: expected %q, got %q",
						tt.current,
						got,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("update booking status: %v", err)
			}

			if booking == nil {
				t.Fatal("expected updated booking")
			}

			if booking.Status != tt.next {
				t.Errorf(
					"expected returned status %q, got %q",
					tt.next,
					booking.Status,
				)
			}

			if got := getBookingStatus(t, pool, bookingID); got != tt.next {
				t.Errorf(
					"expected database status %q, got %q",
					tt.next,
					got,
				)
			}
		})
	}
}

func TestUpdateProviderBookingStatusIsolation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerA := createDashboardProvider(t, pool, service)
	providerB := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	bookingID := createStatusTestBooking(
		t,
		pool,
		service,
		providerA,
		customerID,
		"confirmed",
		time.Now().UTC().Add(240*time.Hour).Truncate(time.Minute),
	)

	_, err := service.UpdateProviderBookingStatus(
		context.Background(),
		providerB,
		bookingID,
		"completed",
	)

	if !errors.Is(err, ErrProviderBookingNotFound) {
		t.Fatalf(
			"expected ErrProviderBookingNotFound, got %v",
			err,
		)
	}

	if got := getBookingStatus(t, pool, bookingID); got != "confirmed" {
		t.Fatalf("booking was modified by another provider: %q", got)
	}
}

func TestUpdateProviderBookingStatusNotFound(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	_, err := service.UpdateProviderBookingStatus(
		context.Background(),
		providerID,
		uuid.New(),
		"completed",
	)

	if !errors.Is(err, ErrProviderBookingNotFound) {
		t.Fatalf(
			"expected ErrProviderBookingNotFound, got %v",
			err,
		)
	}
}

func TestUpdateProviderBookingStatusInvalidInput(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	_, err := service.UpdateProviderBookingStatus(
		context.Background(),
		uuid.Nil,
		uuid.New(),
		"completed",
	)

	if err == nil {
		t.Fatal("expected invalid provider ID error")
	}

	_, err = service.UpdateProviderBookingStatus(
		context.Background(),
		providerID,
		uuid.Nil,
		"completed",
	)

	if err == nil {
		t.Fatal("expected invalid booking ID error")
	}

	_, err = service.UpdateProviderBookingStatus(
		context.Background(),
		providerID,
		uuid.New(),
		"invalid_status",
	)

	if !errors.Is(err, ErrInvalidBookingStatus) {
		t.Fatalf(
			"expected ErrInvalidBookingStatus, got %v",
			err,
		)
	}
}

func TestConcurrentProviderBookingStatusUpdates(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	bookingID := createStatusTestBooking(
		t,
		pool,
		service,
		providerID,
		customerID,
		"confirmed",
		time.Now().UTC().Add(240*time.Hour).Truncate(time.Minute),
	)

	statuses := []string{"completed", "cancelled"}

	var wg sync.WaitGroup
	errs := make(chan error, len(statuses))

	for _, status := range statuses {
		wg.Add(1)

		go func(nextStatus string) {
			defer wg.Done()

			_, err := service.UpdateProviderBookingStatus(
				context.Background(),
				providerID,
				bookingID,
				nextStatus,
			)

			errs <- err
		}(status)
	}

	wg.Wait()
	close(errs)

	successCount := 0
	transitionErrorCount := 0

	for err := range errs {
		switch {
		case err == nil:
			successCount++

		case errors.Is(err, ErrInvalidBookingTransition):
			transitionErrorCount++

		default:
			t.Errorf("unexpected concurrent update error: %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful update, got %d", successCount)
	}

	if transitionErrorCount != 1 {
		t.Errorf(
			"expected exactly 1 invalid transition, got %d",
			transitionErrorCount,
		)
	}

	finalStatus := getBookingStatus(t, pool, bookingID)

	if finalStatus != "completed" && finalStatus != "cancelled" {
		t.Errorf("unexpected final booking status: %q", finalStatus)
	}
}

// Compile-time reference to ensure the test helper uses the expected pool type.
var _ *pgxpool.Pool
