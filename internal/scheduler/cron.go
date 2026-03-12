package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSchedule struct {
	minutes     fieldSet
	hours       fieldSet
	daysOfMonth fieldSet
	months      fieldSet
	daysOfWeek  fieldSet
}

type fieldSet struct {
	allowed map[int]struct{}
}

func (f fieldSet) Has(value int) bool {
	_, ok := f.allowed[value]
	return ok
}

func ParseCron(expr string) (*CronSchedule, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields")
	}

	minutes, err := parseField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	hours, err := parseField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	daysOfMonth, err := parseField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	months, err := parseField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	daysOfWeek, err := parseField(parts[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	return &CronSchedule{
		minutes:     minutes,
		hours:       hours,
		daysOfMonth: daysOfMonth,
		months:      months,
		daysOfWeek:  daysOfWeek,
	}, nil
}

func (c *CronSchedule) NextAfter(after time.Time) (time.Time, error) {
	if c == nil {
		return time.Time{}, fmt.Errorf("cron schedule is nil")
	}

	candidate := after.Truncate(time.Minute).Add(time.Minute)
	limit := candidate.Add(366 * 24 * time.Hour)

	for !candidate.After(limit) {
		if c.matches(candidate) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within one year")
}

func (c *CronSchedule) matches(ts time.Time) bool {
	return c.minutes.Has(ts.Minute()) &&
		c.hours.Has(ts.Hour()) &&
		c.daysOfMonth.Has(ts.Day()) &&
		c.months.Has(int(ts.Month())) &&
		c.daysOfWeek.Has(int(ts.Weekday()))
}

func parseField(raw string, min, max int) (fieldSet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fieldSet{}, fmt.Errorf("field is empty")
	}

	values := make(map[int]struct{})
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch {
		case part == "*":
			for value := min; value <= max; value++ {
				values[value] = struct{}{}
			}
		case strings.HasPrefix(part, "*/"):
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				return fieldSet{}, fmt.Errorf("invalid step %q", part)
			}
			for value := min; value <= max; value += step {
				values[value] = struct{}{}
			}
		case strings.Contains(part, "-"):
			rangeParts := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return fieldSet{}, fmt.Errorf("invalid range start %q", part)
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return fieldSet{}, fmt.Errorf("invalid range end %q", part)
			}
			if start > end || start < min || end > max {
				return fieldSet{}, fmt.Errorf("range %q is out of bounds", part)
			}
			for value := start; value <= end; value++ {
				values[value] = struct{}{}
			}
		default:
			value, err := strconv.Atoi(part)
			if err != nil {
				return fieldSet{}, fmt.Errorf("invalid value %q", part)
			}
			if value < min || value > max {
				return fieldSet{}, fmt.Errorf("value %d is out of bounds", value)
			}
			values[value] = struct{}{}
		}
	}

	return fieldSet{allowed: values}, nil
}
