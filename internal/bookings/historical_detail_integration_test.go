package bookings

import (
	"context"
	"testing"
	"time"
)

// TestHistoricalBookingDetailSurvivesPastAndClosedSlots pins the Phase 15
// contract: GET-equivalent detail for a booking whose slot is in the past
// and/or closed still resolves through the slot join — historical bookings
// must not depend on public availability endpoints.
func TestHistoricalBookingDetailSurvivesPastAndClosedSlots(t *testing.T) {
	pool := newBookingTestPool(t)
	providerID := createBookingTestProvider(t, pool)
	customerID := newBookingTestUser(t, pool, "customer")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bookRepo := NewRepository(pool)

	// A slot that both started in the past and is closed afterwards.
	past := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	pastSlotID := createTestAvailabilitySlot(t, pool, providerID, past, time.Hour, "open")

	booking, err := bookRepo.Create(ctx, customerID, CreateBookingInput{SlotID: pastSlotID})
	if err != nil {
		t.Fatalf("create booking on past slot: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE availability_slots SET status = 'closed' WHERE id = $1`, pastSlotID,
	); err != nil {
		t.Fatalf("close past slot: %v", err)
	}

	detail, err := bookRepo.GetByIDForUser(ctx, booking.ID, customerID, false)
	if err != nil {
		t.Fatalf("historical detail must resolve for closed past slot: %v", err)
	}
	if detail.ID != booking.ID || detail.SlotID != pastSlotID {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if detail.ProviderID != providerID || detail.ProviderName == "" {
		t.Fatalf("detail must include provider join fields: %+v", detail)
	}

	// The provider can read it too.
	if _, err := bookRepo.GetByIDForUser(ctx, booking.ID, providerID, false); err != nil {
		t.Fatalf("provider historical detail must resolve: %v", err)
	}
}
