// Behaviour adapted from toybox toys/other/usleep.c (0BSD, commit 6a0c0ef).
//
// usleep pauses for a whole number of microseconds. It pauses the same way the
// sleep builtin does (see sleepFor): a plain time.Sleep, so cooperative
// cancellation is the shared F05 item rather than something usleep invents.
//
// Known intentional differences: toybox parses its argument with atol, so
// "abc" silently sleeps 0 microseconds; Misa rejects anything that is not a
// non-negative integer as a usage error.
package builtins

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var usleepSpecParsed *Spec

func init() {
	spec, err := ParseSpec("")
	if err != nil {
		panic("builtins: bad usleep spec: " + err.Error())
	}
	usleepSpecParsed = spec
	Register(Command{Name: "usleep", Run: runUsleep})
}

// maxUsleep is the largest microsecond count that fits a time.Duration.
const maxUsleep = math.MaxInt64 / int64(time.Microsecond)

func runUsleep(ctx *Context) int {
	flags, err := usleepSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "usleep: %v\n", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, usleepSpecParsed) {
		return 0
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "usleep: missing operand")
		return 1
	}
	if len(flags.Operands) > 1 {
		fmt.Fprintf(ctx.Stderr, "usleep: unexpected argument %q\n", flags.Operands[1])
		return 1
	}

	arg := strings.TrimSpace(flags.Operands[0])
	us, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || us < 0 {
		fmt.Fprintf(ctx.Stderr, "usleep: invalid number %q\n", flags.Operands[0])
		return 1
	}
	if us > maxUsleep {
		fmt.Fprintf(ctx.Stderr, "usleep: time interval out of range %q\n", flags.Operands[0])
		return 1
	}

	sleepFor(time.Duration(us) * time.Microsecond)
	return 0
}
