package imageout

import (
	"testing"
	"time"
)

func allProbeGates() ProbeGates {
	return ProbeGates{OptIn: true, SerializedOutput: true, ConsumerSupport: true, BoundedInput: true}
}

func TestProbeGatesFailClosed(t *testing.T) {
	for name, gates := range map[string]ProbeGates{
		"default":        {},
		"no opt in":      {SerializedOutput: true, ConsumerSupport: true, BoundedInput: true},
		"no output hook": {OptIn: true, ConsumerSupport: true, BoundedInput: true},
		"no consumer":    {OptIn: true, SerializedOutput: true, BoundedInput: true},
		"no input":       {OptIn: true, SerializedOutput: true, ConsumerSupport: true},
	} {
		t.Run(name, func(t *testing.T) {
			p := NewProbe(gates)
			if _, ok := p.Begin(1, time.Time{}); ok {
				t.Fatal("unexpected query")
			}
			p.Reply(1, true, time.Time{})
			if renderer, done := p.Result(time.Time{}); renderer != HalfBlock || !done {
				t.Fatalf("got %v, %v", renderer, done)
			}
		})
	}
}

func TestProbeTimeoutAndCorrelation(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := NewProbe(allProbeGates())
	if _, ok := p.Begin(0, now); ok {
		t.Fatal("zero token accepted")
	}
	q, ok := p.Begin(123, now)
	if !ok || q.Token != 123 || q.Deadline != now.Add(ProbeTimeout) {
		t.Fatalf("query = %+v, %v", q, ok)
	}
	if _, ok := p.Begin(124, now); ok {
		t.Fatal("second query")
	}
	p.Reply(0, true, now)
	p.Reply(124, true, now)
	p.Reply(123, false, now.Add(-time.Nanosecond))
	if renderer, done := p.Result(now); renderer != HalfBlock || done {
		t.Fatalf("pending = %v, %v", renderer, done)
	}
	if renderer, done := p.Result(q.Deadline); renderer != HalfBlock || !done {
		t.Fatalf("timeout = %v, %v", renderer, done)
	}
	p.Reply(123, true, q.Deadline.Add(time.Nanosecond))
	if renderer, done := p.Result(q.Deadline); renderer != HalfBlock || !done {
		t.Fatalf("late reply = %v, %v", renderer, done)
	}
}

func TestProbeReplyAndSessionCache(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	positive := NewProbe(allProbeGates())
	negative := NewProbe(allProbeGates())
	positive.Begin(1, now)
	negative.Begin(2, now)
	negative.Reply(1, true, now)
	negative.Reply(2, false, now)
	positive.Reply(1, true, now.Add(ProbeTimeout-time.Nanosecond))
	positive.Reply(1, false, now.Add(ProbeTimeout))
	for _, tc := range []struct {
		p    *Probe
		want Renderer
	}{{positive, Kitty}, {negative, HalfBlock}} {
		if got, done := tc.p.Result(now.Add(ProbeTimeout * 2)); got != tc.want || !done {
			t.Fatalf("got %v, %v, want %v", got, done, tc.want)
		}
		if _, ok := tc.p.Begin(3, now); ok {
			t.Fatal("cached session restarted")
		}
	}
	fresh := NewProbe(allProbeGates())
	if _, ok := fresh.Begin(3, now); !ok {
		t.Fatal("new session inherited cache")
	}
}

func TestProbeReplyAtDeadlineIsLate(t *testing.T) {
	now := time.Now()
	p := NewProbe(allProbeGates())
	q, _ := p.Begin(1, now)
	p.Reply(1, true, q.Deadline)
	if got, done := p.Result(q.Deadline); got != HalfBlock || !done {
		t.Fatalf("got %v, %v", got, done)
	}
}
