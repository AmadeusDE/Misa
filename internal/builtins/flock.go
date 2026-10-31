//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

// Behaviour adapted from toybox flock.c (0BSD): the default exclusive lock,
// -s/-x/-u/-n and the single-descriptor invocation. The FILE COMMAND form,
// directory operands and -w are Misa extensions; see the usage text below.
package builtins

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
	"golang.org/x/sys/unix"
)

const flockSpec = "^s(shared)x(exclusive)u(unlock)n(nonblock)w(wait):"

const flockUsage = `Usage: flock [-sxun] [-w SECS] FILE COMMAND [ARG]...
       flock [-sxun] [-w SECS] FD

Manage advisory file locks.

  -s, --shared       take a shared lock
  -x, --exclusive    take an exclusive lock (default)
  -u, --unlock       unlock instead of locking
  -n, --nonblock     fail with status 1 instead of waiting for the lock (ignores -w)
  -w, --wait=SECS    give up after SECS seconds (0 waits indefinitely)
  -h, --help         display this help and exit
      --version      output version information and exit

With a command, FILE (or a directory) is opened and created when missing, the
lock is held while COMMAND runs, and COMMAND's status is returned. -u there only
opens the file: nothing is locked yet, so the command runs unlocked.

The single-operand form locks an existing descriptor. Misa has no shell
redirection support yet (S09), so only descriptors 0, 1 and 2 are reachable and
any other number fails with "fd N not available".`

var flockSpecParsed *Spec

func init() {
	var err error
	flockSpecParsed, err = ParseSpec(flockSpec)
	if err != nil {
		panic("builtins: bad flock spec: " + err.Error())
	}
	Register(Command{Name: "flock", Run: runFlock})
}

// flockMode is the requested lock state, independent of the platform's values.
type flockMode int

const (
	flockExclusive flockMode = iota
	flockShared
	flockUnlock
)

var errFlockTimeout = errors.New("timed out waiting for lock")

func runFlock(ctx *Context) int {
	flags, err := flockSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "flock:", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprintln(ctx.Stdout, flockUsage)
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintln(ctx.Stdout, "flock (Misa) 0.1.0")
		return 0
	}
	if flags.Bool("s") && flags.Bool("x") {
		fmt.Fprintln(ctx.Stderr, "flock: -s and -x are mutually exclusive")
		return 1
	}
	mode := flockExclusive
	switch {
	case flags.Bool("u"):
		mode = flockUnlock
	case flags.Bool("s"):
		mode = flockShared
	}
	nonblock := flags.Bool("n")
	var deadline time.Time
	wait, waitSet := flags.ExplicitString("w")
	if waitSet {
		d, err := flockWaitDuration(wait)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "flock: invalid wait time '%s'\n", wait)
			return 1
		}
		if d > 0 {
			deadline = time.Now().Add(d)
		}
	}

	operands := flags.Operands
	if len(operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "flock: missing operand")
		return 1
	}
	if len(operands) == 1 {
		fd, ok := flockDescriptorNumber(operands[0])
		if !ok {
			fmt.Fprintln(ctx.Stderr, "flock: missing command")
			return 1
		}
		return flockDescriptor(ctx, fd, mode, nonblock, waitSet, deadline)
	}
	return flockPath(ctx, operands[0], operands[1:], mode, nonblock, waitSet, deadline)
}

// flockWaitDuration parses -w SECS. Zero (or an empty value) waits forever.
func flockWaitDuration(s string) (time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return 0, fmt.Errorf("empty wait time")
	}
	seconds, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, fmt.Errorf("invalid wait time")
	}
	ns := seconds * float64(time.Second)
	if ns >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64), nil
	}
	return time.Duration(ns), nil
}

// flockDescriptorNumber reports whether the operand names a descriptor at all.
func flockDescriptorNumber(arg string) (int, bool) {
	if arg == "" {
		return 0, false
	}
	for i := 0; i < len(arg); i++ {
		if arg[i] < '0' || arg[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(arg)
	if err != nil {
		return 0, false
	}
	return n, true
}

// flockFile returns the *os.File behind a descriptor Misa can reach. There is
// no shell descriptor table yet (S09), so this is limited to standard streams.
func flockFile(ctx *Context, fd int) *os.File {
	switch fd {
	case 0:
		f, _ := ctx.Stdin.(*os.File)
		return f
	case 1:
		f, _ := ctx.Stdout.(*os.File)
		return f
	case 2:
		f, _ := ctx.Stderr.(*os.File)
		return f
	default:
		return nil
	}
}

func flockDescriptor(ctx *Context, fd int, mode flockMode, nonblock, waitSet bool, deadline time.Time) int {
	if fd != 0 && fd != 1 && fd != 2 {
		fmt.Fprintf(ctx.Stderr, "flock: fd %d not available (needs shell redirection support, S09)\n", fd)
		return 1
	}
	file := flockFile(ctx, fd)
	if file == nil {
		fmt.Fprintf(ctx.Stderr, "flock: fd %d is not an open file\n", fd)
		return 1
	}
	return flockLock(ctx, int(file.Fd()), mode, nonblock, waitSet, deadline, fmt.Sprintf("fd %d", fd))
}

func flockPath(ctx *Context, name string, argv []string, mode flockMode, nonblock, waitSet bool, deadline time.Time) int {
	file, err := openFlockFile(ctx, name)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "flock: %s: %s\n", name, fmtOsErr(err))
		return 1
	}
	defer file.Close()
	if status := flockLock(ctx, int(file.Fd()), mode, nonblock, waitSet, deadline, "'"+name+"'"); status != 0 {
		return status
	}
	return flockRun(ctx, argv)
}

// openFlockFile opens a path for locking, creating a missing file. O_CREAT on
// an existing directory fails with EISDIR, so directories open read-only.
func openFlockFile(ctx *Context, name string) (*os.File, error) {
	path := resolvePath(ctx, name)
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil && errors.Is(err, unix.EISDIR) {
		return os.Open(path)
	}
	return file, err
}

// flockLock takes or releases the lock, reporting a diagnostic on failure.
// Waits poll a non-blocking request so cancellation and -w both stay reachable.
func flockLock(ctx *Context, fd int, mode flockMode, nonblock, waitSet bool, deadline time.Time, target string) int {
	op := unix.LOCK_EX
	switch mode {
	case flockShared:
		op = unix.LOCK_SH
	case flockUnlock:
		op = unix.LOCK_UN
	}
	if mode == flockUnlock {
		if err := unix.Flock(fd, op); err != nil {
			fmt.Fprintf(ctx.Stderr, "flock: %s: %s\n", target, fmtOsErr(err))
			return 1
		}
		return 0
	}
	if nonblock {
		err := unix.Flock(fd, op|unix.LOCK_NB)
		if err == nil {
			return 0
		}
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			// Toybox exits 1 without a diagnostic when a non-blocking lock is busy.
			return 1
		}
		fmt.Fprintf(ctx.Stderr, "flock: %s: %s\n", target, fmtOsErr(err))
		return 1
	}
	var deadlineOrZero time.Time
	if waitSet {
		deadlineOrZero = deadline
	}
	err := flockWait(ctx.CancellationContext(), fd, op, deadlineOrZero)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errFlockTimeout):
		fmt.Fprintf(ctx.Stderr, "flock: timed out waiting for lock on %s\n", target)
		return 1
	case ctx.CancellationContext().Err() != nil:
		return commandexec.ContextStatus(ctx.CancellationContext().Err())
	default:
		fmt.Fprintf(ctx.Stderr, "flock: %s: %s\n", target, fmtOsErr(err))
		return 1
	}
}

// flockWait requests op until it succeeds, the deadline passes (zero waits
// forever) or the context is cancelled.
func flockWait(ctx context.Context, fd, op int, deadline time.Time) error {
	delay := 2 * time.Millisecond
	for {
		err := unix.Flock(fd, op|unix.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return errFlockTimeout
			}
			if remaining < delay {
				delay = remaining
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < 50*time.Millisecond {
			delay *= 2
			if delay > 50*time.Millisecond {
				delay = 50 * time.Millisecond
			}
		}
	}
}

// flockRun launches COMMAND through the shared launcher and returns its status.
func flockRun(ctx *Context, argv []string) int {
	childCtx := ctx.CancellationContext()
	inv := proceffect.Invocation{Dir: ctx.Dir, Env: ctx.Environ(),
		Stdin: ctx.Stdin, Stdout: ctx.Stdout, Stderr: ctx.Stderr}
	var proc *commandexec.Process
	var err error
	if _, builtin := Lookup(argv[0]); builtin && !strings.ContainsAny(argv[0], `/\`) {
		proc, err = proceffect.StartOptions(childCtx, proceffect.Request{DispatchOnly: true, Argv: argv}, inv, commandexec.Options{})
	} else {
		proc, err = commandexec.Start(childCtx, argv[0], commandexec.Invocation{
			Dir: inv.Dir, Env: inv.Env, Args: argv[1:],
			Stdin: inv.Stdin, Stdout: inv.Stdout, Stderr: inv.Stderr,
		}, commandexec.Options{})
	}
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "flock: failed to run command '%s': %s\n", argv[0], fmtOsErr(err))
		if errors.Is(err, proceffect.ErrUnavailable) {
			return 125
		}
		if childCtx.Err() != nil {
			return commandexec.ContextStatus(childCtx.Err())
		}
		return commandexec.ErrorStatus(err)
	}
	return proc.Wait()
}
