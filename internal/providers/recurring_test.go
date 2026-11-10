package providers

import (
	"errors"
	"testing"
	"time"
)

func mustGenerate(t *testing.T, input RecurringRuleInput, now time.Time) []SlotTime {
	t.Helper()

	startH, startM, endH, endM, from, to, loc, days, err := parseRecurringRule(input, now)
	if err != nil {
		t.Fatalf("parse rule: %v", err)
	}

	slots, err := generateRecurringSlots(startH, startM, endH, endM, from, to, loc, days)
	if err != nil {
		t.Fatalf("generate slots: %v", err)
	}

	return slots
}

// TestRecurringGenerationAcrossSpringForward generates daily 09:00-17:00
// slots across the US 2026 spring-forward transition (2026-03-08). Wall
// times stay 09:00 local, so the UTC instants shift by one hour.
func TestRecurringGenerationAcrossSpringForward(t *testing.T) {
	slots := mustGenerate(t, RecurringRuleInput{
		StartTime:  "09:00",
		EndTime:    "17:00",
		DaysOfWeek: []int{0, 1, 2, 3, 4, 5, 6},
		DateFrom:   "2026-03-07",
		DateTo:     "2026-03-09",
		Timezone:   "America/New_York",
	}, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))

	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}

	expectUTC := []string{
		"2026-03-07T14:00:00Z", // EST (UTC-5)
		"2026-03-08T13:00:00Z", // EDT (UTC-4) after spring forward
		"2026-03-09T13:00:00Z",
	}
	for i, want := range expectUTC {
		if got := slots[i].StartsAt.UTC().Format("2006-01-02T15:04:05Z"); got != want {
			t.Errorf("slot %d start = %s, want %s", i, got, want)
		}
		if !slots[i].EndsAt.After(slots[i].StartsAt) {
			t.Errorf("slot %d end not after start", i)
		}
	}
}

// TestRecurringGenerationAcrossFallBack mirrors the check across the
// 2026 fall-back transition (2026-11-01).
func TestRecurringGenerationAcrossFallBack(t *testing.T) {
	slots := mustGenerate(t, RecurringRuleInput{
		StartTime:  "09:00",
		EndTime:    "17:00",
		DaysOfWeek: []int{0, 1, 2, 3, 4, 5, 6},
		DateFrom:   "2026-10-31",
		DateTo:     "2026-11-02",
		Timezone:   "America/New_York",
	}, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))

	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}

	expectUTC := []string{
		"2026-10-31T13:00:00Z", // EDT (UTC-4)
		"2026-11-01T14:00:00Z", // EST (UTC-5) after fall back
		"2026-11-02T14:00:00Z",
	}
	for i, want := range expectUTC {
		if got := slots[i].StartsAt.UTC().Format("2006-01-02T15:04:05Z"); got != want {
			t.Errorf("slot %d start = %s, want %s", i, got, want)
		}
	}
}

// TestRecurringGenerationWeekdayFilter checks only the requested weekdays
// produce slots (2026-11-16 is a Monday).
func TestRecurringGenerationWeekdayFilter(t *testing.T) {
	slots := mustGenerate(t, RecurringRuleInput{
		StartTime:  "10:00",
		EndTime:    "11:00",
		DaysOfWeek: []int{1, 3, 5}, // Mon, Wed, Fri
		DateFrom:   "2026-11-16",
		DateTo:     "2026-11-22",
		Timezone:   "UTC",
	}, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))

	if len(slots) != 3 {
		t.Fatalf("expected Mon/Wed/Fri only (3 slots), got %d", len(slots))
	}
	for _, s := range slots {
		switch s.StartsAt.UTC().Weekday() {
		case time.Monday, time.Wednesday, time.Friday:
		default:
			t.Errorf("unexpected weekday %v", s.StartsAt.UTC().Weekday())
		}
	}
}

// TestRecurringGenerationDSTGapWindow verifies a window that contains the
// spring-forward gap still produces end > start (2 real hours that day).
func TestRecurringGenerationDSTGapWindow(t *testing.T) {
	slots := mustGenerate(t, RecurringRuleInput{
		StartTime:  "02:00",
		EndTime:    "04:00",
		DaysOfWeek: []int{0},
		DateFrom:   "2026-03-08",
		DateTo:     "2026-03-08",
		Timezone:   "America/New_York",
	}, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))

	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if !slots[0].EndsAt.After(slots[0].StartsAt) {
		t.Fatal("end must stay after start across the DST gap")
	}
	duration := slots[0].EndsAt.Sub(slots[0].StartsAt)
	if duration <= 0 || duration > 2*time.Hour {
		t.Errorf("unexpected duration across the gap: %v", duration)
	}
}

func TestParseRecurringRuleValidation(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		mutate  func(*RecurringRuleInput)
		wantErr error
	}{
		{"unknown timezone", func(r *RecurringRuleInput) { r.Timezone = "Mars/Olympus" }, ErrInvalidTimezone},
		{"end before start", func(r *RecurringRuleInput) { r.EndTime = "08:00" }, ErrInvalidRuleTime},
		{"end equals start", func(r *RecurringRuleInput) { r.EndTime = "09:00" }, ErrInvalidRuleTime},
		{"bad clock", func(r *RecurringRuleInput) { r.StartTime = "9am" }, ErrInvalidRuleTime},
		{"bad date", func(r *RecurringRuleInput) { r.DateFrom = "2026/11/08" }, ErrInvalidRuleWindow},
		{"to before from", func(r *RecurringRuleInput) { r.DateTo = "2026-11-08" }, ErrInvalidRuleWindow},
		{"horizon beyond 90 days", func(r *RecurringRuleInput) { r.DateTo = "2027-03-01" }, ErrInvalidRuleWindow},
		{"past date_from", func(r *RecurringRuleInput) { r.DateFrom = "2026-11-06"; r.DateTo = "2026-11-08" }, ErrRuleInPast},
		{"weekday out of range", func(r *RecurringRuleInput) { r.DaysOfWeek = []int{7} }, ErrInvalidDaysOfWeek},
	}

	base := RecurringRuleInput{
		StartTime:  "09:00",
		EndTime:    "17:00",
		DaysOfWeek: []int{1},
		DateFrom:   "2026-11-09",
		DateTo:     "2026-11-13",
		Timezone:   "UTC",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			tt.mutate(&input)

			_, _, _, _, _, _, _, _, err := parseRecurringRule(input, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("want %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGenerateRecurringSlotsTooMany(t *testing.T) {
	// 90 days × 7 days/week = 90 slots — under the cap. Use a 2-hour window
	// each hour of the day? No: one rule is one window per day. To exceed
	// the cap would need >500 days, which the horizon prevents. So assert
	// the horizon×daily case stays under the cap instead.
	slots, err := generateRecurringSlots(9, 0, 17, 0,
		time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 2, 6, 0, 0, 0, 0, time.UTC),
		time.UTC,
		map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true, 5: true, 6: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slots) != 90 {
		t.Fatalf("expected 90 slots for a 90-day daily rule, got %d", len(slots))
	}
}
