package bookings

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createBookingTestCustomer(
	t *testing.T,
	pool *pgxpool.Pool,
) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("%s@example.com", uuid.NewString())

	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, password_hash, role, is_active)
		VALUES ($1, $2, 'test-hash', 'customer', TRUE)
	`, userID, email)

	if err != nil {
		t.Fatalf("create test customer: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM bookings WHERE customer_id = $1`, userID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE id = $1`, userID)
	})

	return userID
}

func TestListByCustomer(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := NewRepository(pool)

	// Create two independent customers.
	customerA := createBookingTestCustomer(t, pool)
	customerB := createBookingTestCustomer(t, pool)

	// Create a provider and profile for test slots.
	providerID := uuid.New()
	providerEmail := fmt.Sprintf("%s@example.com", uuid.NewString())

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, is_active)
		VALUES ($1, $2, 'test-hash', 'provider', TRUE)
	`, providerID, providerEmail)
	if err != nil {
		t.Fatalf("create test provider: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO provider_profiles (
			user_id, display_name, timezone
		)
		VALUES ($1, 'Integration Test Provider', 'UTC')
	`, providerID)
	if err != nil {
		t.Fatalf("create provider profile: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `
			DELETE FROM bookings
			WHERE slot_id IN (
				SELECT id FROM availability_slots WHERE provider_id = $1
			)
		`, providerID)

		_, _ = pool.Exec(ctx,
			`DELETE FROM availability_slots WHERE provider_id = $1`,
			providerID)

		_, _ = pool.Exec(ctx,
			`DELETE FROM provider_profiles WHERE user_id = $1`,
			providerID)

		_, _ = pool.Exec(ctx,
			`DELETE FROM users WHERE id = $1`,
			providerID)
	})

	// Create three independent slots.
	slotIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for i, slotID := range slotIDs {
		start := time.Now().UTC().Add(time.Duration(i+1) * time.Hour)
		end := start.Add(30 * time.Minute)

		_, err := pool.Exec(ctx, `
			INSERT INTO availability_slots (
				id, provider_id, starts_at, ends_at, status
			)
			VALUES ($1, $2, $3, $4, 'open')
		`, slotID, providerID, start, end)

		if err != nil {
			t.Fatalf("create test slot: %v", err)
		}
	}

	// Customer A receives two bookings.
	// Customer B receives one booking.
	insertBooking := func(slotID, customerID uuid.UUID, status string) {
		t.Helper()

		_, err := pool.Exec(ctx, `
			INSERT INTO bookings (slot_id, customer_id, status)
			VALUES ($1, $2, $3)
		`, slotID, customerID, status)

		if err != nil {
			t.Fatalf("insert test booking: %v", err)
		}
	}

	insertBooking(slotIDs[0], customerA, "confirmed")
	insertBooking(slotIDs[1], customerA, "cancelled")
	insertBooking(slotIDs[2], customerB, "confirmed")

	// Test 1: Customer A should only see their own two bookings.
	input := BookingListInput{
		Limit:  20,
		Offset: 0,
	}

	bookings, err := repo.ListByCustomer(ctx, customerA, input)
	if err != nil {
		t.Fatalf("list customer bookings: %v", err)
	}

	if len(bookings) != 2 {
		t.Fatalf("expected 2 bookings for customer A, got %d", len(bookings))
	}

	for _, booking := range bookings {
		if booking.CustomerID != customerA {
			t.Errorf("customer A received another customer's booking")
		}
	}

	// Test 2: Count should also respect customer ownership.
	total, err := repo.CountByCustomer(ctx, customerA, input)
	if err != nil {
		t.Fatalf("count customer bookings: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	// Test 3: Status filtering.
	confirmed := StatusConfirmed
	input.Status = &confirmed

	bookings, err = repo.ListByCustomer(ctx, customerA, input)
	if err != nil {
		t.Fatalf("list confirmed bookings: %v", err)
	}

	if len(bookings) != 1 || bookings[0].Status != StatusConfirmed {
		t.Errorf("expected exactly one confirmed booking, got %d", len(bookings))
	}

	// Test 4: Pagination.
	input.Status = nil
	input.Limit = 1
	input.Offset = 0

	bookings, err = repo.ListByCustomer(ctx, customerA, input)
	if err != nil {
		t.Fatalf("list paginated bookings: %v", err)
	}

	if len(bookings) != 1 {
		t.Errorf("expected 1 booking on first page, got %d", len(bookings))
	}

	// Test 5: Customer B must only see their own booking.
	customerBBookings, err := repo.ListByCustomer(
		ctx,
		customerB,
		BookingListInput{Limit: 20},
	)
	if err != nil {
		t.Fatalf("list customer B bookings: %v", err)
	}

	if len(customerBBookings) != 1 {
		t.Errorf("expected 1 booking for customer B, got %d", len(customerBBookings))
	}

	for _, booking := range customerBBookings {
		if booking.CustomerID != customerB {
			t.Errorf("customer B received another customer's booking")
		}
	}

	_ = pgx.ErrNoRows // Keep pgx available for future repository tests.
}
