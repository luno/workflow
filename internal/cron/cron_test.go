package cron

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		spec     string
		baseTime time.Time
		want     time.Time
		wantErr  bool
	}{
		{
			name:     "every 5 minutes",
			spec:     "@every 5m",
			baseTime: baseTime,
			want:     baseTime.Add(5 * time.Minute),
			wantErr:  false,
		},
		{
			name:     "every 1 hour 30 minutes",
			spec:     "@every 1h30m",
			baseTime: baseTime,
			want:     baseTime.Add(90 * time.Minute),
			wantErr:  false,
		},
		{
			name:     "midnight daily",
			spec:     "@midnight",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "daily",
			spec:     "@daily",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "hourly",
			spec:     "@hourly",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "weekly",
			spec:     "@weekly",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "monthly",
			spec:     "@monthly",
			baseTime: baseTime,
			want:     time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "yearly",
			spec:     "@yearly",
			baseTime: baseTime,
			want:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "annually",
			spec:     "@annually",
			baseTime: baseTime,
			want:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "every minute",
			spec:     "* * * * *",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 1, 10, 31, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "every hour at minute 15",
			spec:     "15 * * * *",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 1, 11, 15, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "at 2:30 AM daily",
			spec:     "30 2 * * *",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 2, 2, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "at 9:00 AM on the 15th of every month",
			spec:     "0 9 15 * *",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "at 6:00 PM on Fridays",
			spec:     "0 18 * * 5",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 3, 18, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "at noon on January 1st",
			spec:     "0 12 1 1 *",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "using ? for wildcard",
			spec:     "0 0 ? * *",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "invalid @every format",
			spec:     "@every",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid duration",
			spec:     "@every invalid",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "unknown descriptor",
			spec:     "@unknown",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid crontab - too few fields",
			spec:     "* * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid crontab - too many fields",
			spec:     "* * * * * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid minute value",
			spec:     "60 * * * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid hour value",
			spec:     "* 24 * * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid day value",
			spec:     "* * 32 * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid month value",
			spec:     "* * * 13 *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid weekday value",
			spec:     "* * * * 7",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "every 30 seconds",
			spec:     "@every 30s",
			baseTime: baseTime,
			want:     baseTime.Add(30 * time.Second),
			wantErr:  false,
		},
		{
			name:     "every 2 days",
			spec:     "@every 48h",
			baseTime: baseTime,
			want:     baseTime.Add(48 * time.Hour),
			wantErr:  false,
		},
		{
			name:     "at 3:45 AM on weekends (Saturday)",
			spec:     "45 3 * * 6",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 4, 3, 45, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "at 11:59 PM on Sundays",
			spec:     "59 23 * * 0",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 5, 23, 59, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "on the last day of February",
			spec:     "0 0 28 2 *",
			baseTime: baseTime,
			want:     time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "at 6:30 AM on Christmas Day",
			spec:     "30 6 25 12 *",
			baseTime: baseTime,
			want:     time.Date(2025, 12, 25, 6, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "every 15 minutes during business hours",
			spec:     "*/15 9-17 * * 1-5",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "at midnight on the 1st and 15th",
			spec:     "0 0 1,15 * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "whitespace around spec",
			spec:     "  0 0 * * *  ",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "zero values",
			spec:     "0 0 1 1 0",
			baseTime: baseTime,
			want:     time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "maximum values",
			spec:     "59 23 31 12 6",
			baseTime: baseTime,
			want:     time.Date(2025, 12, 6, 23, 59, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "month rollover from November 30th to December 1st",
			spec:     "0 0 1 12 *",
			baseTime: time.Date(2025, 11, 30, 10, 30, 0, 0, time.UTC),
			want:     time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "year rollover from December to January",
			spec:     "0 0 1 1 *",
			baseTime: time.Date(2025, 12, 31, 10, 30, 0, 0, time.UTC),
			want:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "leap year February 29th",
			spec:     "0 12 29 2 *",
			baseTime: time.Date(2024, 2, 28, 10, 30, 0, 0, time.UTC),
			want:     time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "non-leap year skips February 29th",
			spec:     "0 12 29 2 *",
			baseTime: time.Date(2025, 2, 28, 10, 30, 0, 0, time.UTC),
			want:     time.Date(2028, 2, 29, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "Monday through Friday at 9 AM",
			spec:     "0 9 * * 1",
			baseTime: time.Date(2025, 1, 5, 10, 30, 0, 0, time.UTC),
			want:     time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "negative minute value",
			spec:     "-1 * * * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "negative day value",
			spec:     "* * -5 * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "non-numeric field",
			spec:     "abc * * * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "empty field",
			spec:     " * * * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "day 0 (invalid)",
			spec:     "* * 0 * *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "month 0 (invalid)",
			spec:     "* * * 0 *",
			baseTime: baseTime,
			want:     time.Time{},
			wantErr:  true,
		},
		{
			name:     "hourly from exactly current time",
			spec:     "@hourly",
			baseTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			want:     time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "every microsecond (minimal duration)",
			spec:     "@every 1us",
			baseTime: baseTime,
			want:     baseTime.Add(time.Microsecond),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := Parse(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			got := schedule.Next(tt.baseTime)
			if !got.Equal(tt.want) {
				t.Errorf("Schedule.Next() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	specs := []string{
		"* * * * *",
		"0 0 * * *",
		"15 2 * * 1",
		"0 9 15 * *",
		"30 6 25 12 *",
		"@hourly",
		"@daily",
		"@weekly",
		"@monthly",
		"@yearly",
		"@every 5m",
		"@every 1h30m",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spec := specs[i%len(specs)]
		_, err := Parse(spec)
		if err != nil {
			b.Fatalf("Parse(%s) failed: %v", spec, err)
		}
	}
}

func BenchmarkScheduleNext(b *testing.B) {
	baseTime := time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC)

	benchmarks := []struct {
		name string
		spec string
	}{
		{"every_minute", "* * * * *"},
		{"hourly", "@hourly"},
		{"daily_midnight", "0 0 * * *"},
		{"weekly_sunday", "0 0 * * 0"},
		{"monthly_first", "0 0 1 * *"},
		{"specific_time", "15 14 * * 5"},
		{"every_5min", "@every 5m"},
		{"complex_schedule", "30 6 25 12 *"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			schedule, err := Parse(bm.spec)
			if err != nil {
				b.Fatalf("Parse(%s) failed: %v", bm.spec, err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = schedule.Next(baseTime)
			}
		})
	}
}
