// dd's status=progress line.
//
// The line is the same "bytes copied, seconds, rate" text the summary ends
// with, printed on standard error every second with a carriage return in front
// so it overwrites itself, and a final newline before the summary. The format
// and that ordering follow uutils coreutils dd (src/uu/dd/src/progress.rs, MIT)
// at the commit pinned in docs/CREDITS.md.
package builtins

import (
	"fmt"
	"io"
	"time"
)

// ddProgressTicks delivers the interval ticks of status=progress and the
// function that stops them. Tests replace it to drive the line from a channel
// of their own, so no test depends on wall-clock timing.
var ddProgressTicks = func() (<-chan time.Time, func()) {
	ticker := time.NewTicker(time.Second)
	return ticker.C, ticker.Stop
}

// ddProgressPrinted is called after every progress line; tests replace it to
// know when the line has reached the writer.
var ddProgressPrinted = func() {}

// ddProgress is a running status=progress printer, or nil when the copy does
// not report progress.
type ddProgress struct {
	copier  *ddCopier
	w       io.Writer
	start   time.Time
	stopFn  func()
	done    chan struct{}
	exited  chan struct{}
	printed bool
}

// startDDProgress starts the periodic line when status=progress was asked for.
func startDDProgress(w io.Writer, c *ddCopier, start time.Time, status ddStatus) *ddProgress {
	if !status.progress() {
		return nil
	}
	ticks, stopFn := ddProgressTicks()
	p := &ddProgress{
		copier: c,
		w:      w,
		start:  start,
		stopFn: stopFn,
		done:   make(chan struct{}),
		exited: make(chan struct{}),
	}
	go p.run(ticks)
	return p
}

// run prints one line per tick until the copy finishes.
func (p *ddProgress) run(ticks <-chan time.Time) {
	defer close(p.exited)
	for {
		select {
		case <-p.done:
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
			line := ddTransferLine(p.copier.snapshot(), ddElapsed(p.start))
			// The copy goroutine prints read errors to the same writer.
			p.copier.mu.Lock()
			fmt.Fprintf(p.w, "\r%s", line)
			p.copier.mu.Unlock()
			p.printed = true
			ddProgressPrinted()
		}
	}
}

// stop ends the printer and waits for it, so the summary that follows is the
// last thing written.
func (p *ddProgress) stop() {
	if p == nil {
		return
	}
	close(p.done)
	p.stopFn()
	<-p.exited
}

// any reports whether a progress line was printed, which the summary needs to
// know to start on a line of its own.
func (p *ddProgress) any() bool { return p != nil && p.printed }
