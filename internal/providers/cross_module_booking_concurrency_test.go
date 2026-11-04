package providers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	bookingdomain "github.com/priyanshunaredi/slotforge/internal/bookings"
)

func TestConcurrentCustomerCancellationAndProviderCompletion(t *testing.T) {
	pool := newProvidersTestPool(t)

	providerService := NewService(NewRepository(pool))
	customerService := bookingdomain.NewService(
		bookingdomain.NewRepository(pool),
	)

	providerID := createDashboardProvider(t, pool, providerService)
	customerID := createDashboardCustomer(t, pool)

	bookingID := createStatusTestBooking(
		t,
		pool,
		providerService,
		providerID,
		customerID,
		"confirmed",
		time.Now().UTC().Add(240*time.Hour).Truncate(time.Minute),
	)

	start := make(chan struct{})
	var wg sync.WaitGroup

	type operationResult struct {
		name string
		err  error
	}

	results := make(chan operationResult, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start

		_, err := customerService.CancelBooking(
			context.Background(),
			bookingID,
			customerID,
		)

		results <- operationResult{
			name: "customer cancellation",
			err:  err,
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start

		_, err := providerService.UpdateProviderBookingStatus(
			context.Background(),
			providerID,
			bookingID,
			"completed",
		)

		results <- operationResult{
			name: "provider completion",
			err:  err,
		}
	}()

	close(start)

	wg.Wait()
	close(results)

	var successCount int
	var cancellationRejected bool
	var providerTransitionRejected bool

	for result := range results {
		switch {
		case result.err == nil:
			successCount++

		case errors.Is(result.err, bookingdomain.ErrBookingNotCancellable):
			if result.name != "customer cancellation" {
				t.Errorf("unexpected ErrBookingNotCancellable from %s", result.name)
			}
			cancellationRejected = true

		case errors.Is(result.err, ErrInvalidBookingTransition):
			if result.name != "provider completion" {
				t.Errorf("unexpected ErrInvalidBookingTransition from %s", result.name)
			}
			providerTransitionRejected = true

		default:
			t.Errorf("%s returned unexpected error: %v", result.name, result.err)
		}
	}

	if successCount != 1 {
		t.Fatalf("expected exactly one successful operation, got %d", successCount)
	}

	// Exactly one operation must lose the race.
	if cancellationRejected == providerTransitionRejected {
		t.Fatal("expected exactly one operation to be rejected")
	}

	finalStatus := getBookingStatus(t, pool, bookingID)

	switch finalStatus {
	case "cancelled":
		if !providerTransitionRejected {
			t.Fatal("booking is cancelled, but provider transition was not rejected")
		}

	case "completed":
		if !cancellationRejected {
			t.Fatal("booking is completed, but customer cancellation was not rejected")
		}

	default:
		t.Fatalf("unexpected final booking status: %q", finalStatus)
	}

	t.Logf("Cross-module race passed: final status = %s", finalStatus)
}

// Compile-time reference to ensure the test helper uses the expected pool type.
var _ *pgxpool.Pool
