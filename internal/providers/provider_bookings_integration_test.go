package providers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createDashboardProvider(
	t *testing.T,
	pool *pgxpool.Pool,
	service *Service,
) uuid.UUID {
	t.Helper()

	providerID := createProvidersTestUser(t, pool, "provider")

	_, err := service.CreateProfile(
		context.Background(),
		providerID,
		CreateProfileInput{
			DisplayName: "Dashboard Test Provider",
		},
	)
	if err != nil {
		t.Fatalf("create dashboard provider profile: %v", err)
	}

	return providerID
}

func createDashboardCustomer(
	t *testing.T,
	pool *pgxpool.Pool,
) uuid.UUID {
	t.Helper()

	customerID := createProvidersTestUser(t, pool, "customer")

	// This cleanup runs before the helper's user cleanup.
	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM bookings WHERE customer_id = $1`,
			customerID,
		)
	})

	return customerID
}

func createDashboardBooking(
	t *testing.T,
	pool *pgxpool.Pool,
	service *Service,
	providerID uuid.UUID,
	customerID uuid.UUID,
	start time.Time,
	status string,
) {
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
		t.Fatalf("create dashboard slot: %v", err)
	}

	_, err = pool.Exec(context.Background(), `
		INSERT INTO bookings (slot_id, customer_id, status)
		VALUES ($1, $2, $3)
	`, slot.ID, customerID, status)
	if err != nil {
		t.Fatalf("create dashboard booking: %v", err)
	}
}

func TestListProviderBookingsIsolation(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerA := createDashboardProvider(t, pool, service)
	providerB := createDashboardProvider(t, pool, service)

	customerA := createDashboardCustomer(t, pool)
	customerB := createDashboardCustomer(t, pool)

	base := time.Now().UTC().Add(96 * time.Hour).Truncate(time.Minute)

	createDashboardBooking(
		t, pool, service, providerA, customerA,
		base, "confirmed",
	)

	createDashboardBooking(
		t, pool, service, providerB, customerB,
		base.Add(time.Hour), "confirmed",
	)

	result, err := service.ListProviderBookings(
		context.Background(),
		providerA,
		ProviderBookingListInput{
			Limit:  20,
			Offset: 0,
		},
	)
	if err != nil {
		t.Fatalf("list provider bookings: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 booking, got %d", result.Total)
	}

	if len(result.Bookings) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Bookings))
	}

	if result.Bookings[0].CustomerID != customerA {
		t.Errorf("returned booking belonging to unexpected customer")
	}

	if result.Bookings[0].CustomerEmail == "" {
		t.Error("expected customer email in dashboard response")
	}
}

func TestListProviderBookingsFiltersAndPagination(t *testing.T) {
	pool := newProvidersTestPool(t)
	service := NewService(NewRepository(pool))

	providerID := createDashboardProvider(t, pool, service)
	customerID := createDashboardCustomer(t, pool)

	base := time.Now().UTC().Add(120 * time.Hour).Truncate(time.Minute)

	createDashboardBooking(
		t, pool, service, providerID, customerID,
		base, "confirmed",
	)

	createDashboardBooking(
		t, pool, service, providerID, customerID,
		base.Add(time.Hour), "cancelled",
	)

	createDashboardBooking(
		t, pool, service, providerID, customerID,
		base.Add(2*time.Hour), "confirmed",
	)

	// Filter by status.
	confirmed, err := service.ListProviderBookings(
		context.Background(),
		providerID,
		ProviderBookingListInput{
			Status: "confirmed",
			Limit:  20,
		},
	)
	if err != nil {
		t.Fatalf("filter confirmed bookings: %v", err)
	}

	if confirmed.Total != 2 || len(confirmed.Bookings) != 2 {
		t.Fatalf(
			"expected 2 confirmed bookings, total=%d returned=%d",
			confirmed.Total,
			len(confirmed.Bookings),
		)
	}

	// Filter by date range.
	from := base.Add(30 * time.Minute)
	to := base.Add(90 * time.Minute)

	dateFiltered, err := service.ListProviderBookings(
		context.Background(),
		providerID,
		ProviderBookingListInput{
			From:  &from,
			To:    &to,
			Limit: 20,
		},
	)
	if err != nil {
		t.Fatalf("filter bookings by date: %v", err)
	}

	if dateFiltered.Total != 1 {
		t.Fatalf("expected 1 date-filtered booking, got %d", dateFiltered.Total)
	}

	if dateFiltered.Bookings[0].Status != "cancelled" {
		t.Fatalf(
			"expected cancelled booking, got %q",
			dateFiltered.Bookings[0].Status,
		)
	}

	// Verify pagination and total count.
	page, err := service.ListProviderBookings(
		context.Background(),
		providerID,
		ProviderBookingListInput{
			Limit:  1,
			Offset: 1,
		},
	)
	if err != nil {
		t.Fatalf("paginate provider bookings: %v", err)
	}

	if page.Total != 3 {
		t.Fatalf("expected total 3, got %d", page.Total)
	}

	if len(page.Bookings) != 1 {
		t.Fatalf("expected 1 booking on page, got %d", len(page.Bookings))
	}

	if page.Limit != 1 || page.Offset != 1 {
		t.Errorf("unexpected pagination metadata: %+v", page)
	}
}
