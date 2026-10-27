package sched

import (
	"errors"
	"fmt"
	"time"
)

// Spec is when a routine fires: a cron spec in TZ (empty = local), or the one-shot At.
type Spec struct {
	Cron string
	At   time.Time
	TZ   string
}

func (s Spec) Location() (*time.Location, error) {
	if s.TZ == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(s.TZ)
	if err != nil {
		return nil, fmt.Errorf("timezone %q: %v", s.TZ, err)
	}
	return loc, nil
}

// Check reports a spec with neither or both of Cron and At, a bad cron spec or an unknown timezone.
func (s Spec) Check() error {
	if (s.Cron == "") == s.At.IsZero() {
		return errors.New("give either a cron spec or an at time")
	}
	if _, err := s.Location(); err != nil {
		return err
	}
	if s.Cron != "" {
		_, err := ParseCron(s.Cron)
		return err
	}
	return nil
}

// After is the first fire strictly after t; zero when a one-shot time has passed or the spec is bad.
func (s Spec) After(t time.Time) time.Time {
	if s.Cron == "" {
		if s.At.After(t) {
			return s.At
		}
		return time.Time{}
	}
	c, err := ParseCron(s.Cron)
	loc, lerr := s.Location()
	if err != nil || lerr != nil {
		return time.Time{}
	}
	return c.Next(t, loc)
}
