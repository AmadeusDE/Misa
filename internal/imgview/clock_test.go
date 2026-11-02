package imgview

import (
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

func TestManualClockFiresTimersInDeadlineOrder(t *testing.T) {
	c := NewManualClock()
	start := c.Now()
	late := c.NewTimer(10 * time.Millisecond)
	early := c.NewTimer(5 * time.Millisecond)

	c.Advance(6 * time.Millisecond)
	select {
	case <-early.C():
	default:
		t.Fatal("timer at 5ms did not fire at 6ms")
	}
	select {
	case <-late.C():
		t.Fatal("timer at 10ms fired at 6ms")
	default:
	}
	if got := c.Now(); !got.Equal(start.Add(6 * time.Millisecond)) {
		t.Fatalf("now = %v", got.Sub(start))
	}

	c.Advance(4 * time.Millisecond)
	select {
	case <-late.C():
	default:
		t.Fatal("timer at 10ms did not fire at 10ms")
	}
}

func TestManualClockStopPreventsDelivery(t *testing.T) {
	c := NewManualClock()
	tm := c.NewTimer(time.Millisecond)
	if !tm.Stop() {
		t.Fatal("first Stop reported an already stopped timer")
	}
	if tm.Stop() {
		t.Fatal("second Stop reported a stop")
	}
	c.Advance(time.Second)
	select {
	case <-tm.C():
		t.Fatal("stopped timer delivered")
	default:
	}
}

func TestManualClockZeroDelayFiresOnAdvance(t *testing.T) {
	c := NewManualClock()
	tm := c.NewTimer(0)
	c.Advance(0)
	select {
	case <-tm.C():
	default:
		t.Fatal("zero-delay timer did not fire")
	}
	if c.Pending() != 0 {
		t.Fatalf("pending timers = %d", c.Pending())
	}
}

func TestManualClockSatisfiesImageframeClock(t *testing.T) {
	var clock imageframe.Clock = NewManualClock()
	tm := clock.NewTimer(time.Millisecond)
	if tm.Stop() != true {
		t.Fatal("Stop")
	}
	var system imageframe.Clock = SystemClock{}
	if system.Now().IsZero() {
		t.Fatal("system clock has no time")
	}
}

func TestSystemClockTimerFiresAndStops(t *testing.T) {
	clock := SystemClock{}
	stopped := clock.NewTimer(time.Hour)
	if !stopped.Stop() {
		t.Fatal("fresh timer was not stopped")
	}
	if stopped.Stop() {
		t.Fatal("second Stop reported a stop")
	}
	tm := clock.NewTimer(time.Millisecond)
	select {
	case <-tm.C():
	case <-time.After(10 * time.Second):
		t.Fatal("system timer never fired")
	}
	if a, b := clock.Now(), clock.Now(); b.Before(a) {
		t.Fatalf("clock went backwards: %v then %v", a, b)
	}
}

func TestManualClockConcurrentWaitAndAdvance(t *testing.T) {
	c := NewManualClock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		timer := c.NewTimer(10 * time.Millisecond)
		select {
		case <-timer.C():
		case <-time.After(10 * time.Second):
			t.Error("timer never fired")
		}
	}()
	for c.Pending() == 0 {
		time.Sleep(time.Millisecond)
	}
	c.Advance(10 * time.Millisecond)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("waiting goroutine did not finish")
	}
}
