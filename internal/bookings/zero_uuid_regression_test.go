package bookings

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Regression tests for the all-zero UUID (uuid.Nil): a valid but
// nonexistent booking ID must return not-found, the same as any other
// unknown ID, instead of a generic service error that surfaced as 500.

func TestGetBookingZeroUUIDNotFound(t *testing.T) {
	pool := newBookingTestPool(t)
	service := NewService(&Repository{db: pool})

	callerID := newBookingTestUser(t, pool, "customer")

	tests := []struct {
		name        string
		bookingID   uuid.UUID
		expectError error
	}{
		{
			name:        "all-zero UUID",
			bookingID:   uuid.Nil,
			expectError: pgx.ErrNoRows,
		},
		{
			name:        "random nonexistent UUID",
			bookingID:   uuid.New(),
			expectError: pgx.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.GetBooking(
				context.Background(),
				tt.bookingID,
				callerID,
				false,
			)

			if !errors.Is(err, tt.expectError) {
				t.Fatalf("expected %v, got %v", tt.expectError, err)
			}
		})
	}
}

func TestCancelBookingZeroUUIDNotFound(t *testing.T) {
	pool := newBookingTestPool(t)
	service := NewService(&Repository{db: pool})

	customerID := newBookingTestUser(t, pool, "customer")

	tests := []struct {
		name      string
		bookingID uuid.UUID
	}{
		{name: "all-zero UUID", bookingID: uuid.Nil},
		{name: "random nonexistent UUID", bookingID: uuid.New()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CancelBooking(
				context.Background(),
				tt.bookingID,
				customerID,
			)

			if !errors.Is(err, ErrBookingNotFound) {
				t.Fatalf("expected ErrBookingNotFound, got %v", err)
			}
		})
	}
}

func TestRescheduleBookingZeroUUIDNotFound(t *testing.T) {
	pool := newBookingTestPool(t)
	service := NewService(&Repository{db: pool})

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	targetSlot := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(48*time.Hour),
		30*time.Minute,
		"open",
	)

	t.Run("all-zero booking UUID", func(t *testing.T) {
		_, err := service.RescheduleBooking(
			context.Background(),
			uuid.Nil,
			customerID,
			targetSlot,
		)

		if !errors.Is(err, ErrBookingNotFound) {
			t.Fatalf("expected ErrBookingNotFound, got %v", err)
		}
	})

	t.Run("all-zero target slot UUID", func(t *testing.T) {
		_, err := service.RescheduleBooking(
			context.Background(),
			uuid.New(),
			customerID,
			uuid.Nil,
		)

		if !errors.Is(err, ErrSlotNotFound) {
			t.Fatalf("expected ErrSlotNotFound, got %v", err)
		}
	})
}

func TestZeroUUIDHandlersReturn404(t *testing.T) {
	pool := newBookingTestPool(t)
	service := NewService(&Repository{db: pool})

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	targetSlot := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(49*time.Hour),
		30*time.Minute,
		"open",
	)

	router := newBookingHandlerRouter(NewHandler(service))

	zero := uuid.Nil.String()

	tests := []struct {
		name      string
		method    string
		path      string
		payload   any
		wantError string
	}{
		{
			name:      "GET booking detail",
			method:    http.MethodGet,
			path:      "/bookings/" + zero,
			wantError: "booking not found",
		},
		{
			name:      "cancel booking",
			method:    http.MethodPatch,
			path:      "/bookings/" + zero + "/cancel",
			wantError: "booking not found",
		},
		{
			name:      "reschedule booking",
			method:    http.MethodPatch,
			path:      "/bookings/" + zero + "/reschedule",
			payload:   map[string]any{"slot_id": targetSlot.String()},
			wantError: "booking not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, body := doBookingRequest(
				t, router, tt.method, tt.path, tt.payload,
				customerID.String(), "customer",
			)

			if response.Code != http.StatusNotFound {
				t.Fatalf(
					"expected 404, got %d: %s",
					response.Code,
					response.Body.String(),
				)
			}

			if body["error"] != tt.wantError {
				t.Errorf("expected %q, got %v", tt.wantError, body["error"])
			}
		})
	}
}
