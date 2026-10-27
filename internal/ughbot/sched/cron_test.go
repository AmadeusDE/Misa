package sched

import (
	"testing"
	"time"
)

func must(t *testing.T, spec string) Cron {
	t.Helper()
	c, err := ParseCron(spec)
	if err != nil {
		t.Fatalf("%s: %v", spec, err)
	}
	return c
}

func TestNext(t *testing.T) {
	utc := time.UTC
	base := time.Date(2026, 10, 17, 21, 30, 0, 0, utc) // a Saturday
	cases := []struct {
		spec string
		want time.Time
	}{
		{"* * * * *", time.Date(2026, 10, 17, 21, 31, 0, 0, utc)},
		{"0 9 * * *", time.Date(2026, 10, 18, 9, 0, 0, 0, utc)},
		{"@daily", time.Date(2026, 10, 18, 0, 0, 0, 0, utc)},
		{"@hourly", time.Date(2026, 10, 17, 22, 0, 0, 0, utc)},
		{"*/15 * * * *", time.Date(2026, 10, 17, 21, 45, 0, 0, utc)},
		{"0 10 * * mon", time.Date(2026, 10, 19, 10, 0, 0, 0, utc)},
		{"0 10 * * 1-5", time.Date(2026, 10, 19, 10, 0, 0, 0, utc)},
		{"0 10 * * 0", time.Date(2026, 10, 18, 10, 0, 0, 0, utc)},
		{"0 10 * * 7", time.Date(2026, 10, 18, 10, 0, 0, 0, utc)},
		{"30 8 1 * *", time.Date(2026, 11, 1, 8, 30, 0, 0, utc)},
		{"0 0 1 jan *", time.Date(2027, 1, 1, 0, 0, 0, 0, utc)},
		{"0 0 29 2 *", time.Date(2028, 2, 29, 0, 0, 0, 0, utc)},
		{"0 12 13 * fri", time.Date(2026, 10, 23, 12, 0, 0, 0, utc)}, // dom OR dow when both are set
		{"5,35 21 * * *", time.Date(2026, 10, 17, 21, 35, 0, 0, utc)},
		{"0 8-18/5 * * *", time.Date(2026, 10, 18, 8, 0, 0, 0, utc)},
	}
	for _, c := range cases {
		if got := must(t, c.spec).Next(base, utc); !got.Equal(c.want) {
			t.Errorf("%q: got %v want %v", c.spec, got, c.want)
		}
	}
}

func TestNextInATimezoneAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-03-08 02:30 does not exist in New York; the next 02:30 that does is the 9th.
	got := must(t, "30 2 * * *").Next(time.Date(2026, 3, 8, 0, 0, 0, 0, ny), ny)
	if want := time.Date(2026, 3, 9, 2, 30, 0, 0, ny); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = must(t, "0 9 * * *").Next(time.Date(2026, 3, 8, 0, 0, 0, 0, ny), ny)
	if got.Hour() != 9 || got.Day() != 8 {
		t.Fatalf("9:00 local on the DST day: %v", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, spec := range []string{"", "* * * *", "60 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *",
		"* * * * 8", "*/0 * * * *", "5-1 * * * *", "x * * * *", "@sometimes", "* * 31 2 *"} {
		if _, err := ParseCron(spec); err == nil {
			t.Errorf("%q: want error", spec)
		}
	}
}

func TestParseAt(t *testing.T) {
	now := time.Date(2026, 10, 17, 21, 30, 0, 0, time.UTC)
	cases := map[string]time.Time{
		"in 2h":                now.Add(2 * time.Hour),
		"in 90m":               now.Add(90 * time.Minute),
		"2026-10-20 09:00":     time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
		"2026-10-20T09:00:00Z": time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
	}
	for in, want := range cases {
		got, err := ParseAt(in, now, time.UTC)
		if err != nil || !got.Equal(want) {
			t.Errorf("%q: %v %v", in, got, err)
		}
	}
	for _, in := range []string{"tomorrow", "in -2h", "in 0s", "2020-01-01 00:00"} {
		if _, err := ParseAt(in, now, time.UTC); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
}
