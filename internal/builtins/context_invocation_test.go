package builtins

import (
	"context"
	"testing"
	"time"
)

func TestContextInvocationClockAndLocation(t *testing.T) {
	fixed := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	a := &Context{Env: []string{"TZ=UTC"}, Clock: func() time.Time { return fixed }}
	b := &Context{Env: []string{"TZ=Asia/Tokyo"}, Clock: func() time.Time { return fixed.Add(time.Hour) }}
	if !a.Now().Equal(fixed) || !b.Now().Equal(fixed.Add(time.Hour)) {
		t.Fatal("invocation clocks leaked")
	}
	_, offset := b.Now().In(b.Location()).Zone()
	if a.Location() != time.UTC || offset != 9*3600 {
		t.Fatal("invocation time zones leaked")
	}
}

func TestContextCancellationDefaultAndOverride(t *testing.T) {
	c := &Context{}
	if c.DoneContext() == nil || c.DoneContext().Err() != nil {
		t.Fatal("invalid default context")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	c.Cancellation = cancelled
	if c.DoneContext().Err() != context.Canceled {
		t.Fatal("invocation cancellation ignored")
	}
}
