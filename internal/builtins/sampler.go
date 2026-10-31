package builtins

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

// sampleSleep waits between repeated samples. It is a seam: tests replace it so
// interval loops run instantly, and it honours cancellation (a Ctrl-C in a
// `free -s 1` loop stops before the next sample instead of after it).
var sampleSleep = func(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// parseSampleSeconds parses an interval in seconds. Only a positive value makes
// sense: a zero or negative interval would be an unbounded busy loop.
func parseSampleSeconds(s string) (time.Duration, error) {
	secs, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds value '%s'", s)
	}
	if secs <= 0 {
		return 0, errors.New("seconds argument must be greater than 0")
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// printProcError reports a proc reader failure. An unsupported platform gets a
// plain explanation instead of a missing-file error, and nothing is invented in
// its place.
func printProcError(ctx *Context, cmd, what string, err error) {
	if errors.Is(err, proc.ErrUnsupported) {
		fmt.Fprintf(ctx.Stderr, "%s: %s is not supported on this platform\n", cmd, what)
		return
	}
	fmt.Fprintf(ctx.Stderr, "%s: failed to read %s: %v\n", cmd, what, err)
}
