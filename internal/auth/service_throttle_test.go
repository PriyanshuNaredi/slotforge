package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLoginThrottleBlocksAfterLimit(t *testing.T) {
	s := NewService(nil, nil)
	email := "victim@example.com"

	for i := 0; i < loginAttemptLimit; i++ {
		if s.loginThrottled(email) {
			t.Fatalf("throttled early at attempt %d", i+1)
		}
		s.recordLoginFailure(email)
	}

	if !s.loginThrottled(email) {
		t.Fatal("expected throttle after limit")
	}
}

func TestLoginThrottleIsPerAccount(t *testing.T) {
	s := NewService(nil, nil)
	email := "victim2@example.com"

	for i := 0; i < loginAttemptLimit; i++ {
		s.recordLoginFailure(email)
	}

	if !s.loginThrottled(email) {
		t.Fatal("expected throttle for attacked account")
	}
	if s.loginThrottled("other@example.com") {
		t.Fatal("other account must not be throttled")
	}
}

func TestLoginThrottleResetsOnSuccess(t *testing.T) {
	s := NewService(nil, nil)
	email := "victim3@example.com"

	for i := 0; i < loginAttemptLimit; i++ {
		s.recordLoginFailure(email)
	}
	s.recordLoginSuccess(email)

	if s.loginThrottled(email) {
		t.Fatal("expected counter reset after successful login")
	}
}

func TestLoginThrottleWindowExpiry(t *testing.T) {
	s := NewService(nil, nil)
	email := "victim4@example.com"

	for i := 0; i < loginAttemptLimit; i++ {
		s.recordLoginFailure(email)
	}

	// Simulate the window elapsing.
	s.fails[email].start = time.Now().Add(-loginAttemptWindow - time.Second)

	if s.loginThrottled(email) {
		t.Fatal("expected window expiry to clear throttle")
	}
}

// TestLoginThrottleMapBoundedGrowth verifies that cycling distinct email
// addresses cannot grow the failure map beyond its cap, and that a sweep
// reclaims space once windows expire.
func TestLoginThrottleMapBoundedGrowth(t *testing.T) {
	s := NewService(nil, nil)

	over := 200
	for i := 0; i < maxTrackedLoginWindows+over; i++ {
		s.recordLoginFailure(fmt.Sprintf("spray-%d@example.com", i))
	}

	if got := len(s.fails); got > maxTrackedLoginWindows {
		t.Fatalf("map grew to %d entries, cap is %d", got, maxTrackedLoginWindows)
	}

	// Expire every entry, then record one more failure: the sweep must
	// reclaim space and track the new address.
	now := time.Now()
	for _, w := range s.fails {
		w.start = now.Add(-loginAttemptWindow - time.Second)
	}
	s.recordLoginFailure("fresh@example.com")

	if _, ok := s.fails["fresh@example.com"]; !ok {
		t.Fatal("expected sweep to reclaim space for new entries")
	}
}

// TestLoginThrottleConcurrentRecording race-checks the failure recorder:
// run under -race in CI-capable environments; without -race it still
// exercises the mutex path for lost updates.
func TestLoginThrottleConcurrentRecording(t *testing.T) {
	s := NewService(nil, nil)
	email := "racy@example.com"

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.recordLoginFailure(email)
			s.loginThrottled(email)
		}()
	}
	wg.Wait()

	if got := s.fails[email].count; got != 50 {
		t.Fatalf("lost updates under concurrency: count=%d, want 50", got)
	}
}
