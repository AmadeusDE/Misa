// Behaviour adapted from toybox ts.c (0BSD), toys/other/ts.c at commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb.
//
// Deliberate differences from toybox:
//   - A final input line without a trailing newline is timestamped and stays
//     without one; toybox always appends a newline.
//   - More than one operand is an error (toybox ignores the extras).
//   - `-h`/`--help` and `-V`/`--version` come from the shared option parser.
//
// The strftime subset is strftimeFormat from date.go (reused, not duplicated);
// the zone comes from TZ in ctx.Env through getLocation.
package builtins

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

var tsSpecParsed *Spec

// tsNow is the clock hook; tests replace it so output is deterministic.
var tsNow = time.Now

func init() {
	var err error
	tsSpecParsed, err = ParseSpec("ims")
	if err != nil {
		panic("builtins: bad ts spec: " + err.Error())
	}
	Register(Command{Name: "ts", Run: runTs})
}

// tsMillis is toybox's millinow(): wall clock in milliseconds.
func tsMillis(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/int64(time.Millisecond)
}

func runTs(ctx *Context) int {
	flags, err := tsSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "ts:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, tsSpecParsed) {
		return 0
	}
	if len(flags.Operands) > 1 {
		fmt.Fprintf(ctx.Stderr, "ts: extra operand '%s'\nTry 'ts --help' for more information.\n", flags.Operands[1])
		return 1
	}

	format := "%b %d %T"
	rel := flags.Bool("i") || flags.Bool("s")
	if rel || flags.Bool("m") {
		format = "%T"
	}
	if len(flags.Operands) == 1 {
		format = flags.Operands[0]
	}
	incremental := flags.Bool("i")
	millis := flags.Bool("m")

	if ctx.Stdin == nil {
		fmt.Fprintln(ctx.Stderr, "ts: stdin not available")
		return 1
	}

	start := tsMillis(tsNow())
	reader := bufio.NewReaderSize(ctx.Stdin, 64*1024)
	var out []byte
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			newline := line[len(line)-1] == '\n'
			body := line
			if newline {
				body = line[:len(line)-1]
			}

			now := tsMillis(tsNow())
			diff := now
			if rel {
				diff = now - start
			}
			if incremental {
				start = now
			}

			secs := diff / 1000
			t := time.Unix(secs, 0)
			if rel {
				// toybox uses gmtime() for elapsed times.
				t = t.UTC()
			} else {
				t = t.In(getLocation(ctx, t))
			}

			stamp, ok := strftimeFormat(t, format)
			if !ok {
				// strftimeFormat reports with the date builtin's name.
				fmt.Fprintln(ctx.Stderr, "ts:"+strings.TrimPrefix(stamp, "date:"))
				return 1
			}

			out = out[:0]
			out = append(out, stamp...)
			if millis {
				out = append(out, '.')
				frac := strconv.FormatInt(diff%1000, 10)
				for i := len(frac); i < 3; i++ {
					out = append(out, '0')
				}
				out = append(out, frac...)
			}
			out = append(out, ' ')
			out = append(out, body...)
			if newline {
				out = append(out, '\n')
			}
			if _, writeErr := ctx.Stdout.Write(out); writeErr != nil {
				fmt.Fprintf(ctx.Stderr, "ts: write error: %v\n", writeErr)
				return 1
			}
			// One Write per line: nothing is buffered behind us, so the
			// stream advances as soon as a line arrives.
			if flusher, ok := ctx.Stdout.(interface{ Flush() error }); ok {
				if flushErr := flusher.Flush(); flushErr != nil {
					fmt.Fprintf(ctx.Stderr, "ts: write error: %v\n", flushErr)
					return 1
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return 0
			}
			fmt.Fprintf(ctx.Stderr, "ts: read error: %v\n", readErr)
			return 1
		}
	}
}
