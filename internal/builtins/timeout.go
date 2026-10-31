//go:build unix

// Behaviour adapted from toybox timeout.c (0BSD) and uutils coreutils timeout (MIT).
package builtins

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

func init() {
	Register(Command{Name: "timeout", Run: runTimeout})
}

func runTimeout(ctx *Context) int {
	spec, _ := ParseSpec("^k(kill-after):s(signal):v(verbose)f(foreground)p(preserve-status)")
	opts, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "timeout: %v\n", err)
		return 125
	}
	if opts.HelpRequested {
		fmt.Fprintln(ctx.Stdout, "Usage: timeout [OPTION] DURATION COMMAND [ARG]...\nRun COMMAND with a time limit. DURATION accepts s, m, h, d suffixes.\n  -k, --kill-after=DURATION  send KILL after a further delay\n  -s, --signal=SIGNAL        signal to send (default TERM)\n  -v, --verbose              report signals sent\n  -f, --foreground           signal only the command, not its children\n  -p, --preserve-status      return the command's exit status")
		return 0
	}
	if opts.VersionRequested {
		fmt.Fprintln(ctx.Stdout, "timeout (Misa) 0.1.0")
		return 0
	}
	if len(opts.Operands) < 2 {
		fmt.Fprintln(ctx.Stderr, "timeout: missing operand\nTry 'timeout --help' for more information.")
		return 125
	}
	sig := syscall.SIGTERM
	if value, ok := opts.ExplicitString("s"); ok {
		var valid bool
		sig, valid = timeoutSignal(value)
		if !valid {
			fmt.Fprintf(ctx.Stderr, "timeout: '%s': invalid signal\n", value)
			return 125
		}
	}
	var killAfter time.Duration
	if value, ok := opts.ExplicitString("k"); ok {
		killAfter, err = timeoutDuration(value)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "timeout: invalid time interval '%s'\n", value)
			return 125
		}
	}
	duration, err := timeoutDuration(opts.Operands[0])
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "timeout: invalid time interval '%s'\n", opts.Operands[0])
		return 125
	}
	args := opts.Operands[1:]
	var outputMu sync.Mutex
	childCtx := *ctx
	childCtx.Stdout = timeoutOutput(ctx.Stdout, &outputMu)
	childCtx.Stderr = timeoutOutput(ctx.Stderr, &outputMu)
	inv := commandexec.Invocation{Dir: ctx.Dir, Env: ctx.Environ(), Args: args[1:],
		Stdin: ctx.Stdin, Stdout: childCtx.Stdout, Stderr: childCtx.Stderr}
	options := commandexec.Options{ShareGroup: opts.Bool("f"), WaitDelay: 100 * time.Millisecond}
	var proc *commandexec.Process
	if _, builtin := Lookup(args[0]); builtin && !strings.ContainsAny(args[0], `/\`) {
		proc, err = proceffect.StartOptions(ctx.CancellationContext(), proceffect.Request{DispatchOnly: true, Argv: args}, proceffect.Invocation{
			Dir: inv.Dir, Env: inv.Env, Stdin: inv.Stdin, Stdout: inv.Stdout, Stderr: inv.Stderr,
		}, options)
	} else {
		proc, err = commandexec.Start(ctx.CancellationContext(), args[0], inv, options)
	}
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "timeout: failed to run command '%s': %s\n", args[0], fmtOsErr(err))
		if errors.Is(err, proceffect.ErrUnavailable) {
			return 125
		}
		if ctx.CancellationContext().Err() != nil {
			return commandexec.ContextStatus(ctx.CancellationContext().Err())
		}
		return commandexec.ErrorStatus(err)
	}
	var timer *time.Timer
	var alarm <-chan time.Time
	if duration != 0 {
		timer = time.NewTimer(duration)
		alarm = timer.C
		defer timer.Stop()
	}
	timedOut := false
	for {
		select {
		case <-proc.Done():
			status := proc.Wait()
			if timedOut && !opts.Bool("p") && status != 137 {
				return 124
			}
			return status
		case <-alarm:
			// Prefer an already completed command when its deadline coincides with Wait.
			select {
			case <-proc.Done():
				status := proc.Wait()
				if timedOut && !opts.Bool("p") && status != 137 {
					return 124
				}
				return status
			default:
			}
			timedOut = true
			if opts.Bool("v") {
				fmt.Fprintf(childCtx.Stderr, "timeout: sending signal %s to command '%s'\n", timeoutSignalName(sig), args[0])
			}
			_ = proc.Signal(sig, !opts.Bool("f"))
			if sig != syscall.SIGKILL && sig != syscall.SIGCONT {
				_ = proc.Signal(syscall.SIGCONT, !opts.Bool("f"))
			}
			alarm = nil
			if killAfter > 0 && sig != syscall.SIGKILL {
				sig = syscall.SIGKILL
				timer.Reset(killAfter)
				alarm = timer.C
			}
		}
	}
}

type timeoutWriter struct {
	writer io.Writer
	mu     *sync.Mutex
}

func timeoutOutput(w io.Writer, mu *sync.Mutex) io.Writer {
	if _, ok := w.(*os.File); ok {
		return w
	}
	return &timeoutWriter{writer: w, mu: mu}
}

func (w *timeoutWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func timeoutDuration(input string) (time.Duration, error) {
	parse := func(s string) (float64, error) {
		if strings.ContainsRune(s, '_') {
			return 0, strconv.ErrSyntax
		}
		hex := strings.HasPrefix(strings.TrimLeft(s, "+-"), "0x") || strings.HasPrefix(strings.TrimLeft(s, "+-"), "0X")
		if hex && !strings.ContainsAny(s, "pP") {
			s += "p0"
		}
		value, err := strconv.ParseFloat(s, 64)
		if errors.Is(err, strconv.ErrRange) {
			err = nil
		}
		return value, err
	}
	number := input
	multiplier := time.Second
	value, err := parse(number)
	if err != nil && len(number) > 0 {
		switch number[len(number)-1] {
		case 's':
			multiplier = time.Second
		case 'm':
			multiplier = time.Minute
		case 'h':
			multiplier = time.Hour
		case 'd':
			multiplier = 24 * time.Hour
		default:
			return 0, err
		}
		number = number[:len(number)-1]
		value, err = parse(number)
	}
	mantissa := strings.ToLower(number)
	exponent := "e"
	if strings.Contains(mantissa, "0x") {
		exponent = "p"
	}
	if i := strings.Index(mantissa, exponent); i >= 0 {
		mantissa = mantissa[:i]
	}
	nonzero := strings.ContainsAny(mantissa, "123456789abcdef")
	if err != nil || math.IsNaN(value) || value < 0 || math.Signbit(value) && nonzero {
		return 0, fmt.Errorf("invalid duration")
	}
	nanos := value * float64(multiplier)
	if nanos >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64), nil
	}
	if nanos < 1 && nonzero {
		return time.Nanosecond, nil
	}
	return time.Duration(nanos), nil
}

func timeoutSignal(input string) (syscall.Signal, bool) {
	name := strings.TrimPrefix(strings.ToUpper(input), "SIG")
	realtime := map[string]int{"RTMIN": 34, "RTMAX": 64}
	if n, ok := realtime[name]; ok {
		return syscall.Signal(n), true
	}
	for prefix, base := range realtime {
		op := "+"
		if prefix == "RTMAX" {
			op = "-"
		}
		if strings.HasPrefix(name, prefix+op) {
			n, err := strconv.Atoi(strings.TrimPrefix(name, prefix+op))
			if err != nil || n < 0 || n > 30 {
				return 0, false
			}
			if op == "-" {
				n = -n
			}
			return syscall.Signal(base + n), true
		}
	}
	if name == "0" {
		return 0, true
	}
	switch name {
	case "IOT":
		name = "ABRT"
	case "CLD":
		name = "CHLD"
	case "POLL":
		name = "IO"
	}
	return signalByName(name)
}

func timeoutSignalName(sig syscall.Signal) string {
	if sig == 0 {
		return "0"
	}
	if sig == 34 {
		return "RTMIN"
	}
	if sig == 64 {
		return "RTMAX"
	}
	if sig > 34 && sig < 64 {
		return fmt.Sprintf("RTMIN+%d", sig-34)
	}
	for name, value := range signalNames {
		if value == sig {
			return name
		}
	}
	return strconv.Itoa(int(sig))
}
