package providers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestConcurrentOverlappingBlockCreation hammers the same window with
// concurrent block requests: every attempt must either succeed or fail with
// a deliberate, retryable sentinel — never an opaque transaction error
// (deadlock/serialization). Locks are taken in deterministic id order, so
// overlapping windows must serialize, not deadlock.
func TestConcurrentOverlappingBlockCreation(t *testing.T) {
	repo, pool := newIntegrationRepository(t)
	providerID := createTestProvider(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Three slots the windows all overlap, forcing full contention.
	base := time.Date(2032, 3, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, err := repo.CreateSlot(ctx, providerID, CreateSlotInput{
			StartsAt: base.Add(time.Duration(i) * time.Hour),
			EndsAt:   base.Add(time.Duration(i+1) * time.Hour),
		}); err != nil {
			t.Fatalf("seed slot %d: %v", i, err)
		}
	}

	const workers = 8
	start := base.Add(-30 * time.Minute)
	end := base.Add(4 * time.Hour)

	var wg sync.WaitGroup
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := repo.CreateUnavailablePeriod(ctx, providerID,
				CreateUnavailablePeriodInput{StartsAt: start, EndsAt: end})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	for err := range results {
		if err == nil {
			continue
		}
		if errors.Is(err, ErrUnavailableLocked) {
			continue // deliberate retryable outcome, not an opaque 500
		}
		t.Fatalf("unexpected error from concurrent block creation: %v", err)
	}

	// No request was rejected for bookings (there are none), so every
	// attempt either created a block or was transiently locked.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*)::int FROM provider_unavailable_periods WHERE provider_id = $1`,
		providerID,
	).Scan(&count); err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if count < 1 || count > workers {
		t.Fatalf("expected between 1 and %d blocks, got %d", workers, count)
	}
}
