package humanize

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTime(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "-"},
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"hours ago", now.Add(-3 * time.Hour), "3h ago"},
		{"days ago", now.Add(-4 * 24 * time.Hour), "4d ago"},
		{"older than 30d", now.Add(-45 * 24 * time.Hour), "2026-05-27"},
		{"future seconds", now.Add(30 * time.Second), "in <1m"},
		{"future minutes", now.Add(10 * time.Minute), "in 10m"},
		{"future days", now.Add(29 * 24 * time.Hour), "in 29d"},
		{"far future", now.Add(60 * 24 * time.Hour), "2026-09-09"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Time(tc.t, now))
		})
	}
}

func TestDuration(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"seconds", 45 * time.Second, "45s"},
		{"minutes", 30 * time.Minute, "30m"},
		{"whole hours", 6 * time.Hour, "6h"},
		{"hours and minutes", 90 * time.Minute, "1h30m"},
		{"whole days", 168 * time.Hour, "7d"},
		{"days and hours", 36 * time.Hour, "1d12h"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Duration(tc.d))
		})
	}
}
