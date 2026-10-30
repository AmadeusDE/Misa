package builtins

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(Command{Name: "sleep", Run: runSleep})
}

func runSleep(ctx *Context) int {
	if len(ctx.Args) == 0 {
		fmt.Fprintln(ctx.Stderr, "sleep: error: the following required arguments were not provided:\n  <NUMBER>...")
		return 1
	}

	var total time.Duration
	for _, arg := range ctx.Args {
		d, err := parseSleepDuration(arg)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "sleep: invalid time interval %q\n", arg)
			return 1
		}
		total += d
	}

	sleepFor(total)
	return 0
}

// sleepFor is how the sleep family pauses: a plain time.Sleep, so a pane that
// closes ends the process rather than the pause. Cooperative cancellation for
// sleep and usleep is the shared F05 item; see docs/plans/builtins.md.
func sleepFor(d time.Duration) { time.Sleep(d) }

func parseSleepDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty")
	}

	multiplier := time.Second
	last := s[len(s)-1]
	switch last {
	case 's':
		s = s[:len(s)-1]
	case 'm':
		multiplier = time.Minute
		s = s[:len(s)-1]
	case 'h':
		multiplier = time.Hour
		s = s[:len(s)-1]
	case 'd':
		multiplier = 24 * time.Hour
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if val < 0 {
		return 0, fmt.Errorf("negative")
	}

	return time.Duration(val * float64(multiplier)), nil
}
