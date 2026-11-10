package bookings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/priyanshunaredi/slotforge/internal/providers"
)

// TestBookingBlockedByUnavailablePeriod verifies the full enforcement loop:
// a provider block stops new bookings, stops reschedules, never cancels an
// existing booking (blocking is refused instead), and unblocking restores
// bookability.
func TestBookingBlockedByUnavailablePeriod(t *testing.T) {
	pool := newBookingTestPool(t)
	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provRepo := providers.NewRepository(pool)
	bookRepo := NewRepository(pool)

	start := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	slotID := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")

	// Sanity: booking works before any block.
	if _, err := bookRepo.Create(ctx, customerID, CreateBookingInput{SlotID: slotID}); err != nil {
		t.Fatalf("baseline booking failed: %v", err)
	}

	// Blocking a window that overlaps the active booking must be refused —
	// existing bookings are never silently cancelled.
	_, err := provRepo.CreateUnavailablePeriod(ctx, providerID, providers.CreateUnavailablePeriodInput{
		StartsAt: start.Add(-15 * time.Minute),
		EndsAt:   start.Add(15 * time.Minute),
	})
	if !errors.Is(err, providers.ErrUnavailableHasBookings) {
		t.Fatalf("expected ErrUnavailableHasBookings, got %v", err)
	}

	// Block a different slot's window.
	otherStart := start.Add(24 * time.Hour)
	otherSlot := createTestAvailabilitySlot(t, pool, providerID, otherStart, time.Hour, "open")

	period, err := provRepo.CreateUnavailablePeriod(ctx, providerID, providers.CreateUnavailablePeriodInput{
		StartsAt: otherStart.Add(-time.Minute),
		EndsAt:   otherStart.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create unavailable period: %v", err)
	}

	// Booking into the blocked window fails with ErrSlotBlocked.
	if _, err := bookRepo.Create(ctx, customerID, CreateBookingInput{SlotID: otherSlot}); !errors.Is(err, ErrSlotBlocked) {
		t.Fatalf("expected ErrSlotBlocked, got %v", err)
	}

	// Discovery must not advertise slots inside blocked windows.
	found, err := bookRepo.SearchAvailableSlots(ctx, SlotSearchInput{ProviderID: &providerID})
	if err != nil {
		t.Fatalf("search slots: %v", err)
	}
	for _, s := range found {
		if s.ID == otherSlot {
			t.Fatal("blocked slot must not appear in discovery")
		}
	}

	// Availability creation overlapping the block is skipped by the batch
	// path and rejected by the single-slot path.
	created, skipped, err := provRepo.CreateSlotsBatch(ctx, providerID, []providers.SlotTime{
		{StartsAt: otherStart.Add(2 * time.Hour), EndsAt: otherStart.Add(3 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("batch outside block failed: %v", err)
	}
	if created != 1 || skipped != 0 {
		t.Fatalf("batch outside block: created=%d skipped=%d, want 1/0", created, skipped)
	}

	created, skipped, err = provRepo.CreateSlotsBatch(ctx, providerID, []providers.SlotTime{
		{StartsAt: otherStart.Add(30 * time.Minute), EndsAt: otherStart.Add(90 * time.Minute)},
	})
	if err != nil {
		t.Fatalf("batch inside block failed: %v", err)
	}
	if created != 0 || skipped != 1 {
		t.Fatalf("batch inside block: created=%d skipped=%d, want 0/1", created, skipped)
	}

	if _, err := provRepo.CreateSlot(ctx, providerID, providers.CreateSlotInput{
		StartsAt: otherStart.Add(30 * time.Minute),
		EndsAt:   otherStart.Add(90 * time.Minute),
	}); !errors.Is(err, providers.ErrSlotBlockedByUnavailability) {
		t.Fatalf("expected ErrSlotBlockedByUnavailability, got %v", err)
	}

	// Unblocking restores bookability.
	if err := provRepo.DeleteUnavailablePeriod(ctx, providerID, period.ID); err != nil {
		t.Fatalf("delete unavailable period: %v", err)
	}
	if _, err := bookRepo.Create(ctx, customerID, CreateBookingInput{SlotID: otherSlot}); err != nil {
		t.Fatalf("booking after unblock failed: %v", err)
	}

	// Deleting a period the provider does not own looks like a miss.
	stranger := createBookingTestProvider(t, pool)
	if err := provRepo.DeleteUnavailablePeriod(ctx, stranger, period.ID); !errors.Is(err, providers.ErrUnavailableNotFound) {
		t.Fatalf("expected ErrUnavailableNotFound for foreign delete, got %v", err)
	}
}

// TestRescheduleBlockedByUnavailablePeriod verifies rescheduling into a
// blocked window fails without touching the booking.
func TestRescheduleBlockedByUnavailablePeriod(t *testing.T) {
	pool := newBookingTestPool(t)
	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provRepo := providers.NewRepository(pool)
	bookRepo := NewRepository(pool)

	start := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	fromSlot := createTestAvailabilitySlot(t, pool, providerID, start, time.Hour, "open")
	toSlot := createTestAvailabilitySlot(t, pool, providerID, start.Add(48*time.Hour), time.Hour, "open")

	booking, err := bookRepo.Create(ctx, customerID, CreateBookingInput{SlotID: fromSlot})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}

	if _, err := provRepo.CreateUnavailablePeriod(ctx, providerID, providers.CreateUnavailablePeriodInput{
		StartsAt: start.Add(48 * time.Hour).Add(-time.Minute),
		EndsAt:   start.Add(49 * time.Hour),
	}); err != nil {
		t.Fatalf("create block: %v", err)
	}

	if _, err := bookRepo.RescheduleByCustomer(ctx, booking.ID, customerID, toSlot); !errors.Is(err, ErrSlotBlocked) {
		t.Fatalf("expected ErrSlotBlocked on reschedule, got %v", err)
	}

	// The booking is untouched (still on the original slot).
	after, err := bookRepo.GetByIDForUser(ctx, booking.ID, customerID, false)
	if err != nil {
		t.Fatalf("reload booking: %v", err)
	}
	if after.SlotID != fromSlot {
		t.Fatalf("booking slot changed: got %s, want %s", after.SlotID, fromSlot)
	}
}
