package imgview

import (
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

// SystemClock is the production clock. Timers follow the monotonic reading, so
// wall-clock adjustments cannot make a frame boundary fire early or late.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func (SystemClock) NewTimer(d time.Duration) imageframe.Timer {
	return systemTimer{timer: time.NewTimer(max(d, 0))}
}

type systemTimer struct{ timer *time.Timer }

func (t systemTimer) C() <-chan time.Time { return t.timer.C }
func (t systemTimer) Stop() bool          { return t.timer.Stop() }

// ManualClock is a deterministic clock for playback tests. Time only moves in
// Advance, which delivers every due timer once, in deadline order. It is safe
// for concurrent use because a playback wait runs outside the test goroutine.
type ManualClock struct {
	mu     sync.Mutex
	now    time.Time
	seq    uint64
	timers []*manualTimer
}

func NewManualClock() *ManualClock { return &ManualClock{now: time.Unix(0, 0).UTC()} }

func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *ManualClock) NewTimer(d time.Duration) imageframe.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	t := &manualTimer{ch: make(chan time.Time, 1), deadline: c.now.Add(max(d, 0))}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves time forward by d and fires the timers that are due.
func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d > 0 {
		c.now = c.now.Add(d)
	}
	live := c.timers[:0]
	for _, t := range c.timers {
		if t.due(c.now) {
			t.fire(c.now)
			continue
		}
		live = append(live, t)
	}
	c.timers = live
}

// Pending counts timers that have neither fired nor been stopped.
func (c *ManualClock) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.done() {
			n++
		}
	}
	return n
}

type manualTimer struct {
	ch       chan time.Time
	deadline time.Time
	mu       sync.Mutex
	stopped  bool
	fired    bool
}

func (t *manualTimer) C() <-chan time.Time { return t.ch }

func (t *manualTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped || t.fired {
		return false
	}
	t.stopped = true
	return true
}

func (t *manualTimer) done() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped || t.fired
}

func (t *manualTimer) due(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.stopped && !t.fired && !t.deadline.After(now)
}

func (t *manualTimer) fire(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped || t.fired {
		return
	}
	t.fired = true
	t.ch <- now
}
