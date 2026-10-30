// Behaviour adapted from toybox toys/other/count.c (0BSD, commit 6a0c0ef).
//
// count copies stdin to stdout and reports progress on stderr, at most four
// times per second. stdout stays byte-exact: the progress line goes to stderr
// only, ends in "   \r", and the final one is closed with "\n".
package builtins

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const countSpec = "l(long)"

var countSpecParsed *Spec

func init() {
	var err error
	countSpecParsed, err = ParseSpec(countSpec)
	if err != nil {
		panic("builtins: bad count spec: " + err.Error())
	}
	Register(Command{Name: "count", Run: runCount})
}

const (
	// countPeriodMs is both the display interval and the width of one rate
	// slice, in milliseconds (toybox hard-codes 250).
	countPeriodMs = 250
	// countSlices is the ring of rate slices covering the last ~16 seconds.
	countSlices = 64
	// countChunkSize is the read size toybox uses (its sizeof(toybuf)).
	countChunkSize = 65536
)

// countClock and countTicker are the millisecond clock and the update ticker.
// Tests replace both so a run is deterministic.
var (
	countClock  = func() uint64 { return uint64(time.Now().UnixMilli()) }
	countTicker = func() (<-chan time.Time, func()) {
		t := time.NewTicker(countPeriodMs * time.Millisecond)
		return t.C, t.Stop
	}
)

func runCount(ctx *Context) int {
	flags, err := countSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "count: %v\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, countSpecParsed) {
		return 0
	}
	if len(flags.Operands) > 0 {
		fmt.Fprintf(ctx.Stderr, "count: unexpected argument %q\n", flags.Operands[0])
		return 1
	}
	if ctx.Stdin == nil {
		fmt.Fprintln(ctx.Stderr, "count: stdin not available")
		return 1
	}

	ticks, stop := countTicker()
	defer stop()
	return countStream(ctx, flags.Bool("l"), countClock, ticks)
}

// countProgress is count's progress state: the total, the current rate slice
// and the ring of finished slices, all in toybox's layout.
type countProgress struct {
	long  bool
	size  uint64
	tick  uint
	slice [countSlices]uint64
	start uint64 // first display time, ms
	then  uint64 // next display deadline, ms
}

func newCountProgress(long bool, now uint64) *countProgress {
	return &countProgress{long: long, start: now, then: now}
}

// add records n copied bytes in the current slice.
func (p *countProgress) add(n uint64) {
	p.size += n
	p.slice[(countSlices-1)&p.tick] += n
}

// advance retires every 250ms period that ended at or before now, zeroing the
// slice each new period starts with. toybox calls this right after a display.
func (p *countProgress) advance(now uint64) {
	for p.then <= now {
		p.then += countPeriodMs
		p.tick++
		p.slice[(countSlices-1)&p.tick] = 0
	}
}

// line renders one progress update, ready for stderr. The trailing "   \r"
// lets the next update overwrite it; the caller appends "\n" for the last one.
func (p *countProgress) line(now uint64) string {
	var extra string
	if p.long {
		periods := p.tick
		if periods == 0 {
			periods = 1
		} else if periods > countSlices-1 {
			periods = countSlices - 1
		}
		// The current period is still filling up, so count a prorated part of
		// the oldest period too (toybox's compensation for a short average).
		bytes := (p.slice[(countSlices-1)&(p.tick-periods)] * (countPeriodMs - now%countPeriodMs)) / countPeriodMs
		for i := uint(0); i < periods; i++ {
			bytes += p.slice[(countSlices-1)&(p.tick-i)]
		}
		window := uint64(periods * countPeriodMs)
		if now > p.start && window > now-p.start {
			window = now - p.start
		}
		seconds := (now - p.start) / 1000
		extra = fmt.Sprintf(", %sb, %sb/s, %dm%02ds",
			toyboxHumanReadable(p.size), toyboxHumanReadable(bytes*1000/window),
			seconds/60, seconds%60)
	}
	return fmt.Sprintf("%d bytes%s   \r", p.size, extra)
}

// toyboxHumanReadable formats num like toybox's human_readable(buf, num, 0):
// at most three digits, powers of 1024, and one decimal for single digits
// ("1.4K"). The uint64 arithmetic intentionally wraps for huge counts, which
// is what toybox's unsigned long long does.
func toyboxHumanReadable(num uint64) string {
	const divisor = 1024
	snap := uint64(0)
	unit := 0
	s := strconv.FormatUint(num, 10)
	for len(s) > 3 {
		snap = num
		num = (num + divisor/2) / divisor
		unit++
		s = strconv.FormatUint(num, 10)
	}
	if unit > 0 && len(s) == 1 {
		whole := snap / divisor
		tenths := ((snap-whole*divisor)*100 + 50) / divisor / 10
		s = strconv.FormatUint(whole, 10) + "." + strconv.FormatUint(tenths, 10)
	}
	const suffixes = " kMGTPEZY"
	if unit > 0 && unit < len(suffixes) {
		s += strings.ToUpper(string(suffixes[unit]))
	}
	return s
}

// countStream copies ctx.Stdin to ctx.Stdout, writing a progress update to
// ctx.Stderr before the first byte, on every tick that is due, after a chunk
// once a period has passed, and once at the end. A nil ticks channel disables
// the periodic updates (tests use that).
func countStream(ctx *Context, long bool, now func() uint64, ticks <-chan time.Time) int {
	p := newCountProgress(long, now())
	if _, err := io.WriteString(ctx.Stderr, p.line(p.start)); err != nil {
		return 1
	}
	p.advance(p.start)

	type chunk struct {
		data []byte
		err  error
	}
	// The reader goroutine is what lets an idle stdin still update the display,
	// like toybox's poll() timeout. It ends at EOF; on an early return it can
	// stay blocked in Read, but it owns nothing the caller waits for.
	chunks := make(chan chunk, 1)
	go func() {
		buf := make([]byte, countChunkSize)
		for {
			n, err := ctx.Stdin.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				chunks <- chunk{data: data}
			}
			if err != nil {
				chunks <- chunk{err: err}
				return
			}
		}
	}()

	update := func() bool {
		n := now()
		if _, err := io.WriteString(ctx.Stderr, p.line(n)); err != nil {
			return false
		}
		p.advance(n)
		return true
	}

	for {
		select {
		case c := <-chunks:
			if len(c.data) > 0 {
				if _, err := ctx.Stdout.Write(c.data); err != nil {
					fmt.Fprintf(ctx.Stderr, "count: write error: %s\n", fmtOsErr(err))
					return 1
				}
				p.add(uint64(len(c.data)))
				if n := now(); n >= p.then && !update() {
					return 1
				}
			}
			if c.err == nil {
				continue
			}
			if _, err := io.WriteString(ctx.Stderr, p.line(now())+"\n"); err != nil {
				return 1
			}
			if c.err != io.EOF {
				fmt.Fprintf(ctx.Stderr, "count: %s\n", fmtOsErr(c.err))
				return 1
			}
			return 0
		case _, ok := <-ticks:
			if !ok {
				// No periodic updates left; keep copying until EOF.
				ticks = nil
				continue
			}
			if n := now(); n >= p.then && !update() {
				return 1
			}
		}
	}
}
