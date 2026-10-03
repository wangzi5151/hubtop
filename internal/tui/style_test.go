package tui

import (
	"testing"
	"time"
)

func TestRelTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		in   time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "30s ago"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-72 * time.Hour), "3d ago"},
		{time.Time{}, "-"},
	}
	for _, c := range cases {
		if got := relTime(c.in); got != c.want {
			t.Errorf("relTime(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShortDur(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m30s"},
		{90 * time.Minute, "1h30m"},
	}
	for _, c := range cases {
		if got := shortDur(c.in); got != c.want {
			t.Errorf("shortDur(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
