package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Schedule interface {
	Next(time.Time) time.Time
}

func Parse(spec string) (Schedule, error) {
	spec = strings.TrimSpace(spec)

	if strings.HasPrefix(spec, "@every") {
		return parseEvery(spec)
	}

	if strings.HasPrefix(spec, "@") {
		return parseDescriptor(spec)
	}

	return parseCrontab(spec)
}

func parseEvery(spec string) (Schedule, error) {
	parts := strings.Fields(spec)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid @every format: %s", spec)
	}

	duration, err := time.ParseDuration(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid duration in @every: %s", parts[1])
	}

	if duration <= 0 {
		return nil, fmt.Errorf("duration must be > 0: %s", parts[1])
	}

	return &everySchedule{duration: duration}, nil
}

func parseDescriptor(spec string) (Schedule, error) {
	switch spec {
	case "@yearly", "@annually":
		return &cronSchedule{minute: 0, hour: 0, day: 1, month: 1, weekday: -1}, nil
	case "@monthly":
		return &cronSchedule{minute: 0, hour: 0, day: 1, month: -1, weekday: -1}, nil
	case "@weekly":
		return &cronSchedule{minute: 0, hour: 0, day: -1, month: -1, weekday: 0}, nil
	case "@daily", "@midnight":
		return &cronSchedule{minute: 0, hour: 0, day: -1, month: -1, weekday: -1}, nil
	case "@hourly":
		return &cronSchedule{minute: 0, hour: -1, day: -1, month: -1, weekday: -1}, nil
	default:
		return nil, fmt.Errorf("unknown descriptor: %s", spec)
	}
}

func parseCrontab(spec string) (Schedule, error) {
	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return nil, fmt.Errorf("crontab spec must have 5 fields, got %d", len(fields))
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute: %v", err)
	}

	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour: %v", err)
	}

	day, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day: %v", err)
	}

	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month: %v", err)
	}

	weekday, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid weekday: %v", err)
	}

	return &cronSchedule{
		minute:  minute,
		hour:    hour,
		day:     day,
		month:   month,
		weekday: weekday,
	}, nil
}

func parseField(field string, min, max int) (int, error) {
	if field == "*" || field == "?" {
		return -1, nil
	}

	val, err := strconv.Atoi(field)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", field)
	}

	if val < min || val > max {
		return 0, fmt.Errorf("value %d out of range [%d-%d]", val, min, max)
	}

	return val, nil
}

type everySchedule struct {
	duration time.Duration
}

func (e *everySchedule) Next(t time.Time) time.Time {
	return t.Add(e.duration)
}

type cronSchedule struct {
	minute  int
	hour    int
	day     int
	month   int
	weekday int
}

func (c *cronSchedule) Next(t time.Time) time.Time {
	next := t.Add(time.Minute).Truncate(time.Minute)

	for i := 0; i < 4*365*24*60; i++ {
		if c.matches(next) {
			return next
		}
		next = next.Add(time.Minute)
	}

	return time.Time{}
}

func (c *cronSchedule) matches(t time.Time) bool {
	if c.minute != -1 && t.Minute() != c.minute {
		return false
	}
	if c.hour != -1 && t.Hour() != c.hour {
		return false
	}
	if c.month != -1 && int(t.Month()) != c.month {
		return false
	}

	dayMatches := c.day == -1 || t.Day() == c.day
	weekdayMatches := c.weekday == -1 || int(t.Weekday()) == c.weekday

	if c.day != -1 && c.weekday != -1 {
		return dayMatches || weekdayMatches
	}

	return dayMatches && weekdayMatches
}
