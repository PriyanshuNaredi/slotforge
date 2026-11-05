package bookings

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRescheduleMovesBookingAndFreesOldSlot(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	slotA := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")
	slotB := createTestAvailabilitySlot(t, pool, providerID, start.Add(2*time.Hour), time.Hour, "open")

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{
		SlotID: slotA,
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	rescheduled, err := repo.RescheduleByCustomer(
		context.Background(), booking.ID, customerID, slotB,
	)
	if err != nil {
		t.Fatalf("reschedule booking: %v", err)
	}

	// The booking keeps its identity and history.
	if rescheduled.ID != booking.ID {
		t.Errorf("expected booking ID %s to be preserved, got %s", booking.ID, rescheduled.ID)
	}
	if rescheduled.SlotID != slotB {
		t.Errorf("expected slot %s, got %s", slotB, rescheduled.SlotID)
	}
	if rescheduled.Status != StatusConfirmed {
		t.Errorf("expected status confirmed, got %s", rescheduled.Status)
	}
	if rescheduled.CreatedAt != booking.CreatedAt {
		t.Error("expected created_at to be preserved")
	}
	if !rescheduled.UpdatedAt.After(booking.UpdatedAt) {
		t.Error("expected updated_at to advance")
	}

	// The database row points at the new slot.
	var dbSlotID uuid.UUID

	err = pool.QueryRow(context.Background(), `
		SELECT slot_id FROM bookings WHERE id = $1
	`, booking.ID).Scan(&dbSlotID)
	if err != nil {
		t.Fatalf("load booking: %v", err)
	}

	if dbSlotID != slotB {
		t.Errorf("expected stored slot_id %s, got %s", slotB, dbSlotID)
	}

	// The old slot is bookable again: no active booking references it.
	var activeOnOld bool

	err = pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE slot_id = $1 AND status IN ('pending', 'confirmed')
		)
	`, slotA).Scan(&activeOnOld)
	if err != nil {
		t.Fatalf("check old slot: %v", err)
	}

	if activeOnOld {
		t.Error("expected the old slot to be free after rescheduling")
	}
}

func TestRescheduleRejectsOtherCustomersBooking(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	owner := newBookingTestUser(t, pool, "customer")
	attacker := newBookingTestUser(t, pool, "customer")

	slotA := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(48*time.Hour), time.Hour, "open",
	)
	slotB := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(50*time.Hour), time.Hour, "open",
	)

	booking, err := repo.Create(context.Background(), owner, CreateBookingInput{SlotID: slotA})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	_, err = repo.RescheduleByCustomer(
		context.Background(), booking.ID, attacker, slotB,
	)
	if !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("expected ErrBookingNotFound for another customer's booking, got %v", err)
	}

	// The booking must be untouched.
	unchanged, err := repo.GetByIDForUser(context.Background(), booking.ID, owner, false)
	if err != nil {
		t.Fatalf("reload booking: %v", err)
	}

	if unchanged.SlotID != slotA {
		t.Errorf("expected slot %s, got %s", slotA, unchanged.SlotID)
	}
}

func TestRescheduleRejectsCrossProviderTargetSlot(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerA := createBookingTestProvider(t, pool)
	providerB := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotA := createTestAvailabilitySlot(
		t, pool, providerA,
		time.Now().UTC().Add(48*time.Hour), time.Hour, "open",
	)
	otherProviderSlot := createTestAvailabilitySlot(
		t, pool, providerB,
		time.Now().UTC().Add(50*time.Hour), time.Hour, "open",
	)

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{SlotID: slotA})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	_, err = repo.RescheduleByCustomer(
		context.Background(), booking.ID, customerID, otherProviderSlot,
	)
	if !errors.Is(err, ErrSlotCrossProvider) {
		t.Fatalf("expected ErrSlotCrossProvider, got %v", err)
	}
}

func TestRescheduleRejectsUnavailableTargetSlots(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")
	otherCustomer := newBookingTestUser(t, pool, "customer")

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	slotA := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{SlotID: slotA})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	t.Run("closed slot", func(t *testing.T) {
		closedSlot := createTestAvailabilitySlot(
			t, pool, providerID, start.Add(2*time.Hour), time.Hour, "closed",
		)

		if _, err := repo.RescheduleByCustomer(
			context.Background(), booking.ID, customerID, closedSlot,
		); !errors.Is(err, ErrSlotClosed) {
			t.Fatalf("expected ErrSlotClosed, got %v", err)
		}
	})

	t.Run("slot already booked by another customer", func(t *testing.T) {
		bookedSlot := createTestAvailabilitySlot(
			t, pool, providerID, start.Add(3*time.Hour), time.Hour, "open",
		)

		if _, err := repo.Create(context.Background(), otherCustomer, CreateBookingInput{
			SlotID: bookedSlot,
		}); err != nil {
			t.Fatalf("book target slot: %v", err)
		}

		if _, err := repo.RescheduleByCustomer(
			context.Background(), booking.ID, customerID, bookedSlot,
		); !errors.Is(err, ErrSlotBooked) {
			t.Fatalf("expected ErrSlotBooked, got %v", err)
		}
	})
}

func TestRescheduleRejectsNonConfirmedBookings(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)

	statuses := []BookingStatus{StatusCancelled, StatusCompleted, StatusNoShow}

	for index, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			// Stagger the windows: open slots of one provider may not
			// overlap (availability_no_overlap exclusion constraint).
			base := start.Add(time.Duration(index) * 24 * time.Hour)

			slotA := createTestAvailabilitySlot(t, pool, providerID, base, time.Hour, "open")
			slotB := createTestAvailabilitySlot(
				t, pool, providerID, base.Add(2*time.Hour), time.Hour, "open",
			)

			bookingID := insertCancellationTestBooking(t, pool, slotA, customerID, status)

			if _, err := repo.RescheduleByCustomer(
				context.Background(), bookingID, customerID, slotB,
			); !errors.Is(err, ErrBookingNotReschedulable) {
				t.Fatalf("expected ErrBookingNotReschedulable for %s, got %v", status, err)
			}
		})
	}
}

func TestRescheduleSameSlotIsIdempotent(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	slotA := createTestAvailabilitySlot(
		t, pool, providerID,
		time.Now().UTC().Add(48*time.Hour), time.Hour, "open",
	)

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{SlotID: slotA})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	var before int

	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM notifications WHERE booking_id = $1
	`, booking.ID).Scan(&before)
	if err != nil {
		t.Fatalf("count notifications: %v", err)
	}

	rescheduled, err := repo.RescheduleByCustomer(
		context.Background(), booking.ID, customerID, slotA,
	)
	if err != nil {
		t.Fatalf("same-slot reschedule must be an idempotent success, got %v", err)
	}

	if rescheduled.ID != booking.ID || rescheduled.SlotID != slotA {
		t.Errorf("expected the booking to be unchanged, got %+v", rescheduled)
	}

	var after int

	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM notifications WHERE booking_id = $1
	`, booking.ID).Scan(&after)
	if err != nil {
		t.Fatalf("count notifications: %v", err)
	}

	if after != before {
		t.Errorf("same-slot reschedule must not create notifications: %d -> %d", before, after)
	}
}

func TestRescheduleConcurrentTargetSlot(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customer1 := newBookingTestUser(t, pool, "customer")
	customer2 := newBookingTestUser(t, pool, "customer")

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	slot1 := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")
	slot2 := createTestAvailabilitySlot(t, pool, providerID, start.Add(2*time.Hour), time.Hour, "open")
	target := createTestAvailabilitySlot(t, pool, providerID, start.Add(4*time.Hour), time.Hour, "open")

	booking1, err := repo.Create(context.Background(), customer1, CreateBookingInput{SlotID: slot1})
	if err != nil {
		t.Fatalf("create booking 1: %v", err)
	}

	booking2, err := repo.Create(context.Background(), customer2, CreateBookingInput{SlotID: slot2})
	if err != nil {
		t.Fatalf("create booking 2: %v", err)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		rejected  int
		lastErr   error
		winner    uuid.UUID
	)

	for _, attempt := range []struct {
		booking  *Booking
		customer uuid.UUID
	}{
		{booking1, customer1},
		{booking2, customer2},
	} {
		wg.Add(1)

		go func(bookingID, customerID uuid.UUID) {
			defer wg.Done()

			_, err := repo.RescheduleByCustomer(
				context.Background(), bookingID, customerID, target,
			)

			mu.Lock()
			defer mu.Unlock()

			switch {
			case err == nil:
				succeeded++
				winner = bookingID
			case errors.Is(err, ErrSlotBooked):
				rejected++
				lastErr = err
			default:
				lastErr = err
			}
		}(attempt.booking.ID, attempt.customer)
	}

	wg.Wait()

	if succeeded != 1 || rejected != 1 {
		t.Fatalf("expected exactly one success and one rejection, got %d/%d (last error: %v)",
			succeeded, rejected, lastErr)
	}

	// Exactly one active booking ends up on the target slot.
	var activeOnTarget int

	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM bookings
		WHERE slot_id = $1 AND status IN ('pending', 'confirmed')
	`, target).Scan(&activeOnTarget)
	if err != nil {
		t.Fatalf("count target bookings: %v", err)
	}

	if activeOnTarget != 1 {
		t.Fatalf("expected exactly 1 active booking on the target slot, got %d", activeOnTarget)
	}

	// The loser keeps its original slot.
	loserID := booking1.ID
	if winner == booking1.ID {
		loserID = booking2.ID
	}

	// isAdmin=true lets the repository query bypass ownership scoping.
	loser, err := repo.GetByIDForUser(context.Background(), loserID, customer1, true)
	if err != nil {
		t.Fatalf("reload loser booking: %v", err)
	}

	if loser.SlotID != slot1 && loser.SlotID != slot2 {
		t.Errorf("expected the losing booking to keep its own slot, got %s", loser.SlotID)
	}
}

func TestRescheduleCreatesNotificationsForBothParties(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	slotA := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")
	slotB := createTestAvailabilitySlot(t, pool, providerID, start.Add(2*time.Hour), time.Hour, "open")

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{SlotID: slotA})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	if _, err := repo.RescheduleByCustomer(
		context.Background(), booking.ID, customerID, slotB,
	); err != nil {
		t.Fatalf("reschedule booking: %v", err)
	}

	type notificationRow struct {
		UserID uuid.UUID
		Type   string
		Email  *string
	}

	rows, err := pool.Query(context.Background(), `
		SELECT n.user_id, n.type, d.recipient_email
		FROM notifications n
		JOIN notification_deliveries d ON d.notification_id = n.id
		WHERE n.booking_id = $1
		ORDER BY n.created_at ASC
	`, booking.ID)
	if err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	defer rows.Close()

	var notifications []notificationRow

	for rows.Next() {
		var row notificationRow

		if err := rows.Scan(&row.UserID, &row.Type, &row.Email); err != nil {
			t.Fatalf("scan notification: %v", err)
		}

		notifications = append(notifications, row)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate notifications: %v", err)
	}

	// Two from booking creation, two from rescheduling — all with a queued
	// email delivery.
	if len(notifications) != 4 {
		t.Fatalf("expected 4 notifications with deliveries, got %d: %+v", len(notifications), notifications)
	}

	var customerReschedule, providerReschedule bool

	for _, row := range notifications {
		switch row.Type {
		case "booking_rescheduled":
			if row.UserID == customerID && row.Email != nil {
				customerReschedule = true
			}
		case "customer_rescheduled":
			if row.UserID == providerID && row.Email != nil {
				providerReschedule = true
			}
		}
	}

	if !customerReschedule {
		t.Error("expected a booking_rescheduled notification with delivery for the customer")
	}
	if !providerReschedule {
		t.Error("expected a customer_rescheduled notification with delivery for the provider")
	}
}

func TestRescheduleRollsBackWhenNotificationFails(t *testing.T) {
	pool := newBookingTestPool(t)
	repo := &Repository{db: pool}

	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	slotA := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")
	slotB := createTestAvailabilitySlot(t, pool, providerID, start.Add(2*time.Hour), time.Hour, "open")

	booking, err := repo.Create(context.Background(), customerID, CreateBookingInput{SlotID: slotA})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	// Hold an exclusive lock on notifications so the reschedule transaction
	// blocks at its first notification insert, then cancel its context: the
	// failure happens after the slot/booking locks and the booking UPDATE.
	lockConn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire lock connection: %v", err)
	}
	defer lockConn.Release()

	lockTx, err := lockConn.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin lock transaction: %v", err)
	}

	if _, err := lockTx.Exec(context.Background(), `
		LOCK TABLE notifications IN ACCESS EXCLUSIVE MODE
	`); err != nil {
		t.Fatalf("lock notifications table: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type rescheduleResult struct {
		err error
	}

	done := make(chan rescheduleResult, 1)

	go func() {
		_, err := repo.RescheduleByCustomer(ctx, booking.ID, customerID, slotB)
		done <- rescheduleResult{err: err}
	}()

	// Give the reschedule time to take its locks and update the booking
	// before it blocks on the notifications insert.
	time.Sleep(400 * time.Millisecond)
	cancel()

	result := <-done
	if result.err == nil {
		t.Fatal("expected the reschedule to fail when notification creation is interrupted")
	}

	// Release the table lock.
	_ = lockTx.Rollback(context.Background())

	// Nothing may have been persisted: the booking stays on its old slot
	// and no reschedule notifications exist.
	stored, err := repo.GetByIDForUser(context.Background(), booking.ID, customerID, false)
	if err != nil {
		t.Fatalf("reload booking: %v", err)
	}

	if stored.SlotID != slotA {
		t.Errorf("expected rollback to keep slot %s, got %s", slotA, stored.SlotID)
	}

	if stored.Status != StatusConfirmed {
		t.Errorf("expected status confirmed after rollback, got %s", stored.Status)
	}

	var rescheduleNotifications int

	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM notifications
		WHERE booking_id = $1
		  AND type IN ('booking_rescheduled', 'customer_rescheduled')
	`, booking.ID).Scan(&rescheduleNotifications)
	if err != nil {
		t.Fatalf("count reschedule notifications: %v", err)
	}

	if rescheduleNotifications != 0 {
		t.Errorf("expected no reschedule notifications after rollback, got %d", rescheduleNotifications)
	}
}
