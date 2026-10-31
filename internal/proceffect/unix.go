//go:build unix

package proceffect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"syscall"
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
func handle(args []string, dispatch func([]string) int) int {
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
	if len(args) != 2 || (args[0] != "adjust" && args[0] != "dispatch") {
		fmt.Fprintln(os.Stderr, "process effect: invalid protocol")
		return 125
	}
	req, e := decode(args[1])
	if e != nil || len(req.Argv) == 0 {
		fmt.Fprintln(os.Stderr, "process effect: invalid request")
		return 125
	}
	if args[0] == "dispatch" {
		if dispatch == nil {
			return 125
		}
		inEffectGroup.Store(true)
		return dispatch(req.Argv)
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

func run(ctx context.Context, req Request, inv Invocation) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return contextStatus(ctx)
	}
	if len(req.Argv) == 0 {
		return 125
	}
	exe, e := registeredHelper()
	if e != nil {
		if inv.Stderr != nil {
			fmt.Fprintln(inv.Stderr, ErrUnavailable)
		}
		return 125
	}
	cmd := exec.Command(exe, Flag, "adjust", encode(req))
	cmd.Dir = inv.Dir
	cmd.Env = append([]string{}, inv.Env...)
	nested := inEffectGroup.Load()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: !nested}
	// Own the pipes so cancellation closes our ends and output copiers can be
	// joined; a caller-owned Write already blocked must be unblocked by its owner.
	var closeAfterStart, closeAfterWait []*os.File
	var outputDone []<-chan struct{}
	input := inv.Stdin
	if input == nil {
		input = bytes.NewReader(nil)
	}
	if f, ok := input.(*os.File); ok {
		cmd.Stdin = f
	} else {
		r, w, err := os.Pipe()
		if err != nil {
			return 125
		}
		cmd.Stdin = r
		closeAfterStart = append(closeAfterStart, r)
		closeAfterWait = append(closeAfterWait, w)
		go func() { _, _ = io.Copy(w, input); _ = w.Close() }()
	}
	makeOutput := func(dst io.Writer) *os.File {
		if dst == nil {
			dst = io.Discard
		}
		if f, ok := dst.(*os.File); ok {
			return f
		}
		r, w, err := os.Pipe()
		if err != nil {
			return nil
		}
		closeAfterStart = append(closeAfterStart, w)
		closeAfterWait = append(closeAfterWait, r)
		done := make(chan struct{})
		outputDone = append(outputDone, done)
		go func() { defer close(done); _, _ = io.Copy(cancelWriter{ctx, dst}, r); _ = r.Close() }()
		return w
	}
	cmd.Stdout = makeOutput(inv.Stdout)
	cmd.Stderr = makeOutput(inv.Stderr)
	if cmd.Stdout == nil || cmd.Stderr == nil {
		for _, f := range append(closeAfterStart, closeAfterWait...) {
			_ = f.Close()
		}
		return 125
	}
	e = cmd.Start()
	for _, f := range closeAfterStart {
		_ = f.Close()
	}
	if e != nil {
		for _, f := range closeAfterWait {
			_ = f.Close()
		}
		return 125
	}
	kill := func() {
		if nested {
			_ = cmd.Process.Kill()
		} else {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case e = <-done:
	case <-ctx.Done():
		kill()
		e = <-done
	}
	// Preserve output on normal completion. On cancellation close our pipes,
	// then join output workers so no caller-owned writer is touched after Run
	// returns. The caller must unblock any Write that is already in progress.
	for _, d := range outputDone {
		select {
		case <-d:
		case <-ctx.Done():
			kill()
			goto drained
		}
	}
drained:
	for _, f := range closeAfterWait {
		_ = f.Close()
	}
	for _, d := range outputDone {
		<-d
	}

	if ctx.Err() != nil {
		return contextStatus(ctx)
	}
	if e == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(e, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				return 128 + int(ws.Signal())
			}
			return ws.ExitStatus()
		}
	}
	return 125
}
func contextStatus(ctx context.Context) int {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return 124
	}
	return 130
}

// Do not begin new writes once cancellation is observed. A Write already in
// progress remains caller-owned and cannot be forcibly interrupted.
type cancelWriter struct {
	ctx context.Context
	dst io.Writer
}

func (w cancelWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.dst.Write(p)
}
