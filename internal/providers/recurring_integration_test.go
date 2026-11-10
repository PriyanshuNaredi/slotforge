package providers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestCreateSlotsBatchIdempotentAndOverlap verifies the batch insert
// semantics: exact duplicates are skipped, a genuine overlap rejects the
// whole batch, and a rejected batch leaves no partial rows.
func TestCreateSlotsBatchIdempotentAndOverlap(t *testing.T) {
	repo, pool := newIntegrationRepository(t)
	providerID := createTestProvider(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	base := time.Date(2030, 5, 4, 9, 0, 0, 0, time.UTC) // a far-future Saturday
	slots := []SlotTime{
		{StartsAt: base, EndsAt: base.Add(time.Hour)},
		{StartsAt: base.Add(2 * time.Hour), EndsAt: base.Add(3 * time.Hour)},
	}

	created, skipped, err := repo.CreateSlotsBatch(ctx, providerID, slots)
	if err != nil {
		t.Fatalf("first batch: %v", err)
	}
	if created != 2 || skipped != 0 {
		t.Fatalf("first batch: created=%d skipped=%d, want 2/0", created, skipped)
	}

	// Re-running the same rule skips both (idempotent generation).
	created, skipped, err = repo.CreateSlotsBatch(ctx, providerID, slots)
	if err != nil {
		t.Fatalf("duplicate batch: %v", err)
	}
	if created != 0 || skipped != 2 {
		t.Fatalf("duplicate batch: created=%d skipped=%d, want 0/2", created, skipped)
	}

	// A slot overlapping the first one (9:30 start) must reject the whole
	// batch, including the non-overlapping third slot.
	overlap := []SlotTime{
		{StartsAt: base.Add(30 * time.Minute), EndsAt: base.Add(90 * time.Minute)},
		{StartsAt: base.Add(5 * time.Hour), EndsAt: base.Add(6 * time.Hour)},
	}
	_, _, err = repo.CreateSlotsBatch(ctx, providerID, overlap)
	if !errors.Is(err, ErrSlotOverlap) {
		t.Fatalf("expected ErrSlotOverlap, got %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*)::int FROM availability_slots WHERE provider_id = $1 AND starts_at = $2`,
		providerID, base.Add(5*time.Hour),
	).Scan(&count); err != nil {
		t.Fatalf("count check: %v", err)
	}
	if count != 0 {
		t.Fatal("rejected batch must leave no partial rows")
	}
}

// TestCreateSlotsBatchRolledBackOnOverlapUniqueStart verifies a slot whose
// start matches an existing row (but different end) is skipped, not rejected.
func TestCreateSlotsBatchRolledBackOnOverlapUniqueStart(t *testing.T) {
	repo, pool := newIntegrationRepository(t)
	providerID := createTestProvider(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Date(2031, 1, 2, 8, 0, 0, 0, time.UTC)
	if _, _, err := repo.CreateSlotsBatch(ctx, providerID, []SlotTime{
		{StartsAt: start, EndsAt: start.Add(time.Hour)},
	}); err != nil {
		t.Fatalf("seed slot: %v", err)
	}

	// Same start, different end → duplicate skip, no overlap error.
	created, skipped, err := repo.CreateSlotsBatch(ctx, providerID, []SlotTime{
		{StartsAt: start, EndsAt: start.Add(2 * time.Hour)},
	})
	if err != nil {
		t.Fatalf("same-start batch: %v", err)
	}
	if created != 0 || skipped != 1 {
		t.Fatalf("same-start batch: created=%d skipped=%d, want 0/1", created, skipped)
	}
}
