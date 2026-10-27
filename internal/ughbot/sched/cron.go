// Package sched parses routine times: 5-field cron specs and one-shot "at" times.
package sched

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // timezones work on hosts without a zoneinfo database
)

// Cron is a parsed 5-field spec. Each field is a bit set of allowed values.
type Cron struct {
	minute, hour, dom, month, dow uint64
	domAny, dowAny                bool
}

type field struct {
	name     string
	min, max int
	names    []string
}

var fields = []field{
	{"minute", 0, 59, nil},
	{"hour", 0, 23, nil},
	{"day of month", 1, 31, nil},
	{"month", 1, 12, []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}},
	{"day of week", 0, 7, []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}},
}

var macros = map[string]string{
	"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
	"@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *",
}

// horizon bounds the search for the next fire; a spec with none inside it never fires.
const horizon = 8 * 366 * 24 * time.Hour

// ParseCron reads "min hour dom month dow" (with *, lists, ranges, /steps, month and day names)
// or a macro such as @daily. A spec that can never fire is an error.
func ParseCron(spec string) (Cron, error) {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if m, ok := macros[spec]; ok {
		spec = m
	} else if strings.HasPrefix(spec, "@") {
		return Cron{}, fmt.Errorf("unknown cron macro %q", spec)
	}
	parts := strings.Fields(spec)
	if len(parts) != 5 {
		return Cron{}, fmt.Errorf("cron spec needs 5 fields (minute hour day-of-month month day-of-week), got %d", len(parts))
	}
	var sets [5]uint64
	for i, p := range parts {
		set, err := parseField(p, fields[i])
		if err != nil {
			return Cron{}, err
		}
		sets[i] = set
	}
	if sets[4]&(1<<7) != 0 {
		sets[4] |= 1 // 7 is Sunday too
	}
	c := Cron{minute: sets[0], hour: sets[1], dom: sets[2], month: sets[3], dow: sets[4] &^ (1 << 7),
		domAny: parts[2] == "*", dowAny: parts[4] == "*"}
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if c.Next(from, time.UTC).IsZero() {
		return Cron{}, fmt.Errorf("cron spec %q never fires", spec)
	}
	return c, nil
}

func parseField(s string, f field) (uint64, error) {
	var set uint64
	for _, item := range strings.Split(s, ",") {
		rng, stepText, hasStep := strings.Cut(item, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return 0, fmt.Errorf("%s: bad step %q", f.name, stepText)
			}
			step = n
		}
		lo, hi := f.min, f.max
		if rng != "*" {
			a, b, isRange := strings.Cut(rng, "-")
			var err error
			if lo, err = f.value(a); err != nil {
				return 0, err
			}
			hi = lo
			if isRange {
				if hi, err = f.value(b); err != nil {
					return 0, err
				}
			} else if hasStep {
				hi = f.max
			}
			if lo > hi {
				return 0, fmt.Errorf("%s: range %q runs backwards", f.name, rng)
			}
		}
		for v := lo; v <= hi; v += step {
			set |= 1 << v
		}
	}
	return set, nil
}

func (f field) value(s string) (int, error) {
	for i, n := range f.names {
		if s == n {
			return i + f.min, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < f.min || v > f.max {
		return 0, fmt.Errorf("%s: %q is not in %d-%d", f.name, s, f.min, f.max)
	}
	return v, nil
}

func (c Cron) dayMatches(t time.Time) bool {
	dom := c.dom&(1<<t.Day()) != 0
	dow := c.dow&(1<<int(t.Weekday())) != 0
	switch {
	case c.domAny && c.dowAny:
		return true
	case c.domAny:
		return dow
	case c.dowAny:
		return dom
	}
	return dom || dow // both restricted: either matches, as in Vixie cron
}

// Next is the first fire strictly after after, in loc's wall clock; zero when there is none
// within the horizon. Wall times a DST jump skips are skipped.
func (c Cron) Next(after time.Time, loc *time.Location) time.Time {
	after = after.In(loc)
	day := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, loc)
	end := after.Add(horizon)
	for ; !day.After(end); day = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, loc) {
		if c.month&(1<<int(day.Month())) == 0 || !c.dayMatches(day) {
			continue
		}
		for h := 0; h < 24; h++ {
			if c.hour&(1<<h) == 0 {
				continue
			}
			for m := 0; m < 60; m++ {
				if c.minute&(1<<m) == 0 {
					continue
				}
				t := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc)
				if t.After(after) && t.Hour() == h && t.Minute() == m {
					return t
				}
			}
		}
	}
	return time.Time{}
}

// ParseAt reads a one-shot time: "in <duration>" (e.g. in 2h, in 90m), RFC 3339, or
// "YYYY-MM-DD HH:MM" in loc. It must be in the future.
func ParseAt(s string, now time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	var t time.Time
	if d, ok := strings.CutPrefix(s, "in "); ok {
		dur, err := time.ParseDuration(strings.ReplaceAll(d, " ", ""))
		if err != nil {
			return time.Time{}, fmt.Errorf("at: %v", err)
		}
		t = now.Add(dur)
	} else if v, err := time.Parse(time.RFC3339, s); err == nil {
		t = v
	} else if v, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		t = v
	} else {
		return time.Time{}, errors.New(`at: use "in 2h", "2006-01-02 15:04" or RFC 3339`)
	}
	if !t.After(now) {
		return time.Time{}, fmt.Errorf("at: %s is not in the future", t.Format(time.RFC3339))
	}
	return t, nil
}
