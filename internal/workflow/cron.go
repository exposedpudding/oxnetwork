package workflow

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is a parsed standard 5-field cron expression evaluated in UTC.
// Supported syntax per field: *, N, A-B, lists, and /step on * or ranges.
// Names (JAN, MON) and macros (@daily) are intentionally unsupported.
type Schedule struct {
	minute, hour, dom, month, dow uint64
	domStar, dowStar              bool
}

var cronRanges = [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}

var errCronFields = errors.New("Use 5 fields: minute hour day-of-month month day-of-week.")

func ParseCron(expression string) (Schedule, error) {
	parts := strings.Fields(expression)
	if len(parts) != 5 {
		return Schedule{}, errCronFields
	}
	var masks [5]uint64
	for i, part := range parts {
		mask, err := parseCronField(part, cronRanges[i][0], cronRanges[i][1])
		if err != nil {
			return Schedule{}, err
		}
		masks[i] = mask
	}
	// Day-of-week 7 is Sunday, the same as 0.
	if masks[4]&(1<<7) != 0 {
		masks[4] |= 1
		masks[4] &^= 1 << 7
	}
	return Schedule{minute: masks[0], hour: masks[1], dom: masks[2], month: masks[3], dow: masks[4], domStar: parts[2] == "*" || strings.HasPrefix(parts[2], "*/"), dowStar: parts[4] == "*" || strings.HasPrefix(parts[4], "*/")}, nil
}

func parseCronField(field string, lo, hi int) (uint64, error) {
	var mask uint64
	for _, item := range strings.Split(field, ",") {
		base, stepText, hasStep := strings.Cut(item, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return 0, errors.New("Cron step must be at least 1.")
			}
			step = n
		}
		start, end := lo, hi
		switch {
		case base == "*":
		case strings.Contains(base, "-"):
			a, b, _ := strings.Cut(base, "-")
			x, errA := strconv.Atoi(a)
			y, errB := strconv.Atoi(b)
			if errA != nil || errB != nil {
				return 0, fmt.Errorf("Invalid cron field %q.", field)
			}
			start, end = x, y
		default:
			n, err := strconv.Atoi(base)
			if err != nil {
				return 0, fmt.Errorf("Invalid cron field %q.", field)
			}
			start, end = n, n
			if hasStep {
				end = hi
			}
		}
		if start < lo || end > hi || start > end {
			return 0, fmt.Errorf("Value out of range in %q.", field)
		}
		for v := start; v <= end; v += step {
			mask |= 1 << uint(v)
		}
	}
	return mask, nil
}

// Matches reports whether the schedule fires in the minute containing t.
// As in standard cron, when both day-of-month and day-of-week are restricted
// a day matches if either does.
func (s Schedule) Matches(t time.Time) bool {
	t = t.UTC()
	if s.minute&(1<<uint(t.Minute())) == 0 || s.hour&(1<<uint(t.Hour())) == 0 || s.month&(1<<uint(t.Month())) == 0 {
		return false
	}
	domOK := s.dom&(1<<uint(t.Day())) != 0
	dowOK := s.dow&(1<<uint(t.Weekday())) != 0
	switch {
	case s.domStar && s.dowStar:
		return true
	case s.domStar:
		return dowOK
	case s.dowStar:
		return domOK
	default:
		return domOK || dowOK
	}
}

// Next returns the first matching minute strictly after t, searching up to
// one year ahead. The second result is false when none exists.
func (s Schedule) Next(t time.Time) (time.Time, bool) {
	t = t.UTC().Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(1, 0, 1)
	for ; t.Before(limit); t = t.Add(time.Minute) {
		if s.Matches(t) {
			return t, true
		}
	}
	return time.Time{}, false
}
