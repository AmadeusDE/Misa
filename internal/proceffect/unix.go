//go:build unix

package proceffect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AmadeusDE/Misa/internal/commandexec"
)

var inEffectGroup atomic.Bool

func encode(req Request) string {
	b, _ := json.Marshal(req)
	return base64.RawURLEncoding.EncodeToString(b)
}
func decode(s string) (Request, error) {
	var r Request
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(b, &r)
	return r, e
}
func handle(args []string, dispatch func(context.Context, []string) int) int {
	if e := EnableSelfHelper(); e != nil {
		fmt.Fprintf(os.Stderr, "process effect: helper unavailable: %v\n", e)
		return 125
	}
	if len(args) == 1 && args[0] == "umask" {
		mask := syscall.Umask(0)
		syscall.Umask(mask)
		fmt.Printf("%d", mask)
		return 0
	}
	if len(args) != 2 || (args[0] != "adjust" && args[0] != "dispatch" && args[0] != "renice") {
		fmt.Fprintln(os.Stderr, "process effect: invalid protocol")
		return 125
	}
	req, e := decode(args[1])
	if e != nil {
		fmt.Fprintln(os.Stderr, "process effect: invalid request")
		return 125
	}
	// SIG_IGN survives exec, so the child's descendants outlive a closed
	// terminal. Only the isolated child is affected.
	if req.IgnoreHup {
		signal.Ignore(syscall.SIGHUP)
	}
	if args[0] == "renice" {
		return reniced(req.Renice)
	}
	if len(req.Argv) == 0 {
		fmt.Fprintln(os.Stderr, "process effect: invalid request")
		return 125
	}
	if args[0] == "dispatch" {
		if dispatch == nil {
			return 125
		}
		inEffectGroup.Store(true)
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signals)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stopped, joined := make(chan struct{}), make(chan struct{})
		var received os.Signal
		go func() {
			defer close(joined)
			select {
			case received = <-signals:
				cancel()
			case <-stopped:
			}
		}()
		status := dispatch(ctx, req.Argv)
		close(stopped)
		<-joined
		if received != nil {
			return 128 + int(received.(syscall.Signal))
		}
		return status
	}
	runtime.LockOSThread()
	n, e := niceness()
	if e == nil { // Saturate without overflowing int.
		if req.Adjustment > 0 && req.Adjustment > 19-n {
			n = 19
		} else if req.Adjustment < 0 && req.Adjustment < -20-n {
			n = -20
		} else {
			n += req.Adjustment
		}
		e = syscall.Setpriority(syscall.PRIO_PROCESS, 0, n)
	}
	if e != nil {
		if errors.Is(e, syscall.EPERM) || errors.Is(e, syscall.EACCES) {
			fmt.Fprintln(os.Stderr, "nice: warning: setpriority: Permission denied")
		} else {
			fmt.Fprintf(os.Stderr, "nice: warning: setpriority: %v\n", e)
		}
	}
	exe, e := registeredHelper()
	if e == nil {
		e = syscall.Exec(exe, []string{exe, Flag, "dispatch", args[1]}, os.Environ())
	}
	fmt.Fprintf(os.Stderr, "process effect: re-exec: %v\n", e)
	return 125
}

func sharesGroup() bool { return inEffectGroup.Load() }

// reniced applies explicit targets from the isolated child. Each failure is
// reported on its own line; any failure makes the child exit 1.
func reniced(r *ReniceRequest) int {
	if r == nil || len(r.IDs) == 0 {
		fmt.Fprintln(os.Stderr, "process effect: invalid request")
		return 125
	}
	status := 0
	for _, id := range r.IDs {
		// toybox's renice treats -n as an increment over the current value.
		current, e := priority(r.Which, id)
		if e != nil {
			fmt.Fprintf(os.Stderr, "renice: setpriority %d: %s\n", id, errnoText(e))
			status = 1
			continue
		}
		next := current + r.Priority
		if next > 19 {
			next = 19
		} else if next < -20 {
			next = -20
		}
		if e := syscall.Setpriority(r.Which, id, next); e != nil {
			fmt.Fprintf(os.Stderr, "renice: setpriority %d: %s\n", id, errnoText(e))
			status = 1
		}
	}
	return status
}

// errnoText renders an errno as a capitalised reason, matching the diagnostics
// the stream utilities print.
func errnoText(e error) string {
	switch {
	case errors.Is(e, syscall.EPERM), errors.Is(e, syscall.EACCES):
		return "Permission denied"
	case errors.Is(e, syscall.ESRCH):
		return "No such process"
	case errors.Is(e, syscall.EINVAL):
		return "Invalid argument"
	}
	s := e.Error()
	if s != "" && s[0] >= 'a' && s[0] <= 'z' {
		s = string(s[0]-32) + s[1:]
	}
	return s
}

func start(ctx context.Context, req Request, inv Invocation, opts commandexec.Options) (*commandexec.Process, error) {
	if len(req.Argv) == 0 && req.Renice == nil {
		return nil, errors.New("process effect: missing command")
	}
	exe, err := registeredHelper()
	if err != nil {
		return nil, err
	}
	stage := "adjust"
	if req.DispatchOnly {
		stage = "dispatch"
	}
	if req.Renice != nil {
		stage = "renice"
	}
	// A helper may own nested groups. Give cooperative dispatch time to tear
	// those down before the outer owner forcibly kills its own group.
	opts.CancelSignal, opts.CancelGrace = syscall.SIGTERM, 350*time.Millisecond
	return commandexec.Start(ctx, exe, commandexec.Invocation{
		Dir: inv.Dir, Env: append([]string{}, inv.Env...), Args: []string{Flag, stage, encode(req)},
		Stdin: inv.Stdin, Stdout: inv.Stdout, Stderr: inv.Stderr,
	}, opts)
}

func run(ctx context.Context, req Request, inv Invocation) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return commandexec.ContextStatus(ctx.Err())
	}
	child, err := Start(ctx, req, inv)
	if err != nil {
		if inv.Stderr != nil {
			fmt.Fprintln(inv.Stderr, err)
		}
		return 125
	}
	return child.Wait()
}
