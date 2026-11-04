package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListProviderBookingsValidation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	ctx := context.Background()

	from := time.Now().UTC().Add(150 * time.Hour)
	to := from.Add(time.Hour)

	cases := []struct {
		name     string
		provider uuid.UUID
		input    ProviderBookingListInput
		expected error
	}{
		{
			"nil provider ID",
			uuid.Nil,
			ProviderBookingListInput{Limit: 20},
			nil, // asserted by message below
		},
		{
			"invalid status",
			providerID,
			ProviderBookingListInput{Status: "shipped", Limit: 20},
			ErrInvalidBookingStatus,
		},
		{
			"negative limit",
			providerID,
			ProviderBookingListInput{Limit: -5},
			ErrInvalidLimit,
		},
		{
			"limit above 100",
			providerID,
			ProviderBookingListInput{Limit: 101},
			ErrInvalidLimit,
		},
		{
			"negative offset",
			providerID,
			ProviderBookingListInput{Limit: 20, Offset: -1},
			ErrInvalidOffset,
		},
		{
			"from equals to",
			providerID,
			ProviderBookingListInput{From: &from, To: &from, Limit: 20},
			ErrInvalidDateRange,
		},
		{
			"from after to",
			providerID,
			ProviderBookingListInput{From: &to, To: &from, Limit: 20},
			ErrInvalidDateRange,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.ListProviderBookings(ctx, tc.provider, tc.input)

			if tc.expected == nil {
				if err == nil || err.Error() != "invalid provider ID" {
					t.Fatalf("expected invalid provider ID error, got %v", err)
				}
				return
			}

			if !errors.Is(err, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, err)
			}
		})
	}
}

func TestListProviderBookingsZeroLimitDefaultsTo20(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)

	result, err := service.ListProviderBookings(
		context.Background(),
		providerID,
		ProviderBookingListInput{Limit: 0},
	)
	if err != nil {
		t.Fatalf("list with zero limit: %v", err)
	}

	if result.Limit != 20 {
		t.Errorf("expected limit to default to 20, got %d", result.Limit)
	}

	// Empty provider: empty slice, not nil, and zero total.
	if result.Bookings == nil {
		t.Fatal("expected non-nil empty bookings slice")
	}

	if len(result.Bookings) != 0 || result.Total != 0 {
		t.Errorf("expected empty result, got %d bookings total=%d",
			len(result.Bookings), result.Total)
	}
}

func TestListProviderBookingsValidCombinations(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	base := time.Now().UTC().Add(200 * time.Hour).Truncate(time.Minute)

	// Booking A starts exactly at base, booking B one hour later.
	createDashboardBooking(t, pool, service, providerID, customerID, base, "cancelled")
	createDashboardBooking(t, pool, service, providerID, customerID, base.Add(time.Hour), "confirmed")

	ctx := context.Background()

	// Cancelled bookings must remain visible when requested.
	cancelled, err := service.ListProviderBookings(ctx, providerID, ProviderBookingListInput{
		Status: "cancelled",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("filter cancelled: %v", err)
	}

	if cancelled.Total != 1 || cancelled.Bookings[0].Status != "cancelled" {
		t.Fatalf("expected 1 cancelled booking, got total=%d", cancelled.Total)
	}

	// From is inclusive: a slot starting exactly at `from` is included.
	toExclusive := base.Add(time.Hour) // To is exclusive: excludes booking B.

	inclusive, err := service.ListProviderBookings(ctx, providerID, ProviderBookingListInput{
		From:  &base,
		To:    &toExclusive,
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("filter by boundary dates: %v", err)
	}

	if inclusive.Total != 1 || !inclusive.Bookings[0].SlotStartsAt.Equal(base) {
		t.Fatalf("expected exactly the booking at `from`, got total=%d", inclusive.Total)
	}

	// Status and date range combine with AND semantics.
	rangeEnd := base.Add(2 * time.Hour)

	combined, err := service.ListProviderBookings(ctx, providerID, ProviderBookingListInput{
		Status: "confirmed",
		From:   &base,
		To:     &rangeEnd,
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("combined filters: %v", err)
	}

	if combined.Total != 1 || combined.Bookings[0].Status != "confirmed" {
		t.Fatalf("expected 1 confirmed booking in range, got total=%d", combined.Total)
	}
}
