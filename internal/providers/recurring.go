package providers

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidTimezone    = errors.New("unknown timezone")
	ErrInvalidRuleWindow  = errors.New("invalid recurrence window: date_to must be on or after date_from, and no more than 90 days out")
	ErrInvalidRuleTime    = errors.New("invalid recurrence rule: end_time must be after start_time on the same day")
	ErrInvalidDaysOfWeek  = errors.New("days_of_week must contain values from 0 (Sunday) to 6 (Saturday)")
	ErrRuleTooManySlots   = errors.New("recurrence range generates too many slots; shorten the window or the days list")
	ErrRuleInPast         = errors.New("date_from must not be in the past")
	ErrRuleNoMatchingDays = errors.New("the window contains none of the requested weekdays")
)

const (
	// maxRecurringHorizonDays bounds how far one rule may generate.
	maxRecurringHorizonDays = 90
	// maxRecurringSlots bounds the number of slots a single rule creates.
	maxRecurringSlots = 500
)

// RecurringRuleInput describes weekly recurring availability.
//
// Times are wall-clock "HH:MM" in the provider's timezone; dates are
// "YYYY-MM-DD" in the same timezone. DaysOfWeek follows Go's time.Weekday:
// 0 = Sunday … 6 = Saturday.
type RecurringRuleInput struct {
	StartTime  string `json:"start_time" binding:"required"`
	EndTime    string `json:"end_time" binding:"required"`
	DaysOfWeek []int  `json:"days_of_week" binding:"required,min=1,max=7"`
	DateFrom   string `json:"date_from" binding:"required,datetime=2006-01-02"`
	DateTo     string `json:"date_to" binding:"required,datetime=2006-01-02"`
	Timezone   string `json:"timezone" binding:"required"`
}

// SlotTime is one concrete generated slot window.
type SlotTime struct {
	StartsAt time.Time
	EndsAt   time.Time
}

// parseRecurringRule validates the rule fields and returns the parsed
// pieces. All errors are sentinel errors the handler maps to 400.
func parseRecurringRule(input RecurringRuleInput, now time.Time) (
	startH, startM, endH, endM int,
	from, to time.Time,
	loc *time.Location,
	days map[int]bool,
	err error,
) {
	loc, err = time.LoadLocation(strings.TrimSpace(input.Timezone))
	if err != nil {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidTimezone
	}

	startH, startM, err = parseClock(input.StartTime)
	if err != nil {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidRuleTime
	}
	endH, endM, err = parseClock(input.EndTime)
	if err != nil {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidRuleTime
	}
	if endH*60+endM <= startH*60+startM {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidRuleTime
	}

	from, err = time.ParseInLocation("2006-01-02", input.DateFrom, loc)
	if err != nil {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidRuleWindow
	}
	to, err = time.ParseInLocation("2006-01-02", input.DateTo, loc)
	if err != nil {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidRuleWindow
	}

	if days := len(input.DaysOfWeek); days == 0 || days > 7 {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidDaysOfWeek
	}
	days = make(map[int]bool, len(input.DaysOfWeek))
	for _, d := range input.DaysOfWeek {
		if d < 0 || d > 6 {
			return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidDaysOfWeek
		}
		days[d] = true
	}

	// The window is interpreted in the provider's timezone.
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	if from.Before(today) {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrRuleInPast
	}
	if to.Before(from) || to.After(today.AddDate(0, 0, maxRecurringHorizonDays)) {
		return 0, 0, 0, 0, time.Time{}, time.Time{}, nil, nil, ErrInvalidRuleWindow
	}

	return startH, startM, endH, endM, from, to, loc, days, nil
}

func parseClock(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid clock %q", value)
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", value)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", value)
	}
	return h, m, nil
}

// generateRecurringSlots materializes concrete slot windows for the rule.
//
// DST behaviour follows Go's time.Date rules: a wall-clock time inside a
// spring-forward gap resolves to the instant after the gap; a wall-clock
// time repeated by a fall-back resolves to the first occurrence. The
// resulting instants are deterministic and stored as UTC timestamptz.
func generateRecurringSlots(
	startH, startM, endH, endM int,
	from, to time.Time,
	loc *time.Location,
	days map[int]bool,
) ([]SlotTime, error) {
	out := make([]SlotTime, 0)

	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		if !days[int(day.Weekday())] {
			continue
		}

		startsAt := time.Date(day.Year(), day.Month(), day.Day(), startH, startM, 0, 0, loc)
		endsAt := time.Date(day.Year(), day.Month(), day.Day(), endH, endM, 0, 0, loc)
		if !endsAt.After(startsAt) {
			// A DST gap can fold the window; keep the invariant end > start.
			return nil, ErrInvalidRuleTime
		}

		out = append(out, SlotTime{StartsAt: startsAt.UTC(), EndsAt: endsAt.UTC()})
		if len(out) > maxRecurringSlots {
			return nil, ErrRuleTooManySlots
		}
	}

	if len(out) == 0 {
		return nil, ErrRuleNoMatchingDays
	}

	return out, nil
}
