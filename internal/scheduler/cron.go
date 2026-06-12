package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NextDue(job Job, now time.Time) time.Time {
	now = now.UTC().Truncate(time.Second)
	switch strings.TrimSpace(job.ScheduleType) {
	case ScheduleManual:
		return time.Time{}
	case ScheduleInterval:
		interval, err := time.ParseDuration(strings.TrimSpace(job.ScheduleExpr))
		if err != nil || interval <= 0 {
			return time.Time{}
		}
		base := parseTimeOrZero(job.LastRunAt)
		if base.IsZero() {
			base = parseTimeOrZero(job.CreatedAt)
		}
		if base.IsZero() {
			base = now
		}
		return base.Add(interval).UTC()
	case ScheduleOneTime:
		if strings.TrimSpace(job.LastRunAt) != "" {
			return time.Time{}
		}
		return parseTimeOrZero(job.ScheduleExpr)
	case ScheduleCron:
		base := parseTimeOrZero(job.LastRunAt)
		if base.IsZero() {
			base = parseTimeOrZero(job.CreatedAt)
		}
		if base.IsZero() {
			base = now
		}
		next, err := NextCronTime(job.ScheduleExpr, base)
		if err != nil {
			return time.Time{}
		}
		return next
	default:
		return time.Time{}
	}
}

func NextCronTime(expr string, after time.Time) (time.Time, error) {
	spec, err := parseCronSpec(expr)
	if err != nil {
		return time.Time{}, err
	}
	cursor := after.UTC().Truncate(time.Minute).Add(time.Minute)
	deadline := cursor.AddDate(1, 0, 0)
	for !cursor.After(deadline) {
		if spec.matches(cursor) {
			return cursor, nil
		}
		cursor = cursor.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("cron expression has no matching time within one year")
}

func MissedRunCount(job Job, now time.Time, capCount int) int {
	if capCount <= 0 {
		capCount = 1000
	}
	now = now.UTC().Truncate(time.Second)
	nextDue := parseTimeOrZero(job.NextDueAt)
	if nextDue.IsZero() || nextDue.After(now) || nextDue.Equal(now) {
		return 0
	}
	switch strings.TrimSpace(job.ScheduleType) {
	case ScheduleInterval:
		interval, err := time.ParseDuration(strings.TrimSpace(job.ScheduleExpr))
		if err != nil || interval <= 0 {
			return 0
		}
		count := int(now.Sub(nextDue) / interval)
		if count > capCount {
			return capCount
		}
		return count
	case ScheduleCron:
		count := 0
		cursor := nextDue
		for count < capCount {
			next, err := NextCronTime(job.ScheduleExpr, cursor)
			if err != nil || next.After(now) {
				break
			}
			count++
			cursor = next
		}
		return count
	default:
		return 0
	}
}

func validateCronExpression(expr string) error {
	_, err := parseCronSpec(expr)
	return err
}

type cronSpec struct {
	minute     cronField
	hour       cronField
	dayOfMonth cronField
	month      cronField
	dayOfWeek  cronField
}

type cronField map[int]bool

func parseCronSpec(expr string) (cronSpec, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return cronSpec{}, fmt.Errorf("cron jobs require a five-field cron expression")
	}
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSpec{}, fmt.Errorf("cron minute field: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return cronSpec{}, fmt.Errorf("cron hour field: %w", err)
	}
	dayOfMonth, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return cronSpec{}, fmt.Errorf("cron day-of-month field: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return cronSpec{}, fmt.Errorf("cron month field: %w", err)
	}
	dayOfWeek, err := parseCronField(fields[4], 0, 7)
	if err != nil {
		return cronSpec{}, fmt.Errorf("cron day-of-week field: %w", err)
	}
	if dayOfWeek[7] {
		dayOfWeek[0] = true
		delete(dayOfWeek, 7)
	}
	return cronSpec{minute: minute, hour: hour, dayOfMonth: dayOfMonth, month: month, dayOfWeek: dayOfWeek}, nil
}

func parseCronField(input string, minValue int, maxValue int) (cronField, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("field is empty")
	}
	result := cronField{}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty list item")
		}
		step := 1
		if strings.Contains(part, "/") {
			pieces := strings.Split(part, "/")
			if len(pieces) != 2 {
				return nil, fmt.Errorf("invalid step expression %q", part)
			}
			parsedStep, err := strconv.Atoi(pieces[1])
			if err != nil || parsedStep <= 0 {
				return nil, fmt.Errorf("invalid step in %q", part)
			}
			step = parsedStep
			part = pieces[0]
		}
		start, end, err := cronRange(part, minValue, maxValue)
		if err != nil {
			return nil, err
		}
		for value := start; value <= end; value += step {
			result[value] = true
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("field has no values")
	}
	return result, nil
}

func cronRange(input string, minValue int, maxValue int) (int, int, error) {
	if input == "*" || input == "" {
		return minValue, maxValue, nil
	}
	if strings.Contains(input, "-") {
		pieces := strings.Split(input, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("invalid range %q", input)
		}
		start, err := parseCronInt(pieces[0], minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		end, err := parseCronInt(pieces[1], minValue, maxValue)
		if err != nil {
			return 0, 0, err
		}
		if start > end {
			return 0, 0, fmt.Errorf("range start is after end: %q", input)
		}
		return start, end, nil
	}
	value, err := parseCronInt(input, minValue, maxValue)
	return value, value, err
}

func parseCronInt(input string, minValue int, maxValue int) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", input)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("value %d outside %d-%d", value, minValue, maxValue)
	}
	return value, nil
}

func (spec cronSpec) matches(value time.Time) bool {
	weekday := int(value.Weekday())
	return spec.minute[value.Minute()] &&
		spec.hour[value.Hour()] &&
		spec.dayOfMonth[value.Day()] &&
		spec.month[int(value.Month())] &&
		spec.dayOfWeek[weekday]
}

func parseTimeOrZero(input string) time.Time {
	if strings.TrimSpace(input) == "" {
		return time.Time{}
	}
	value, err := time.Parse(time.RFC3339, strings.TrimSpace(input))
	if err != nil {
		return time.Time{}
	}
	return value.UTC()
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
