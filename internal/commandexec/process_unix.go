//go:build unix

package commandexec

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Process struct {
	cmd    *exec.Cmd
	owned  bool
	done   chan struct{}
	result Result
}

func (p *Process) Done() <-chan struct{} { return p.done }
func (p *Process) Wait() int             { return p.WaitResult().Status }

// WaitResult waits for the process and returns its status and termination cause.
// Like Wait, it may be called repeatedly or concurrently.
func (p *Process) WaitResult() Result { <-p.done; return p.result }
func (p *Process) Signal(sig os.Signal, group bool) error {
	select {
	case <-p.done:
		return os.ErrProcessDone
	default:
	}
	if group && p.owned {
		s, ok := sig.(syscall.Signal)
		if !ok {
			return syscall.EINVAL
		}
		return syscall.Kill(-p.cmd.Process.Pid, s)
	}
	return p.cmd.Process.Signal(sig)
}

// Start starts a command. A caller-blocked input reader cannot be interrupted;
// its copying goroutine may outlive Wait. The owner must unblock a blocked
// output writer before Wait can return.
func Start(ctx context.Context, name string, inv Invocation, opt Options) (*Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := Resolve(name, inv)
	if err != nil {
		return nil, err
	}
	argv0 := inv.Argv0
	if argv0 == "" {
		argv0 = name
	}
	cmd := exec.Command(path)
	cmd.Args = append([]string{argv0}, inv.Args...)
	cmd.Dir = inv.Dir
	cmd.Env = inv.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: !opt.ShareGroup}
	var start, wait []*os.File
	var outputs []<-chan struct{}
	var writeMu sync.Mutex
	progress := make(chan struct{}, 1)
	var busy atomic.Int32 // output copiers currently writing to the caller
	closeFiles := func(fs []*os.File) {
		for _, f := range fs {
			_ = f.Close()
		}
	}
	fail := func(e error) (*Process, error) {
		closeFiles(start)
		closeFiles(wait)
		for _, d := range outputs {
			<-d
		}
		return nil, e
	}
	input := inv.Stdin
	if input == nil {
		input = bytes.NewReader(nil)
	}
	if f, ok := input.(*os.File); ok {
		cmd.Stdin = f
	} else {
		r, w, e := os.Pipe()
		if e != nil {
			return fail(e)
		}
		start = append(start, r)
		wait = append(wait, w)
		cmd.Stdin = r
		go func() { _, _ = io.Copy(w, input); _ = w.Close() }()
	}
	output := func(dst io.Writer) (*os.File, error) {
		if dst == nil {
			dst = io.Discard
		}
		if f, ok := dst.(*os.File); ok {
			return f, nil
		}
		r, w, e := os.Pipe()
		if e != nil {
			return nil, e
		}
		start = append(start, w)
		wait = append(wait, r)
		d := make(chan struct{})
		outputs = append(outputs, d)
		go func() {
			defer close(d)
			copyOutput(lockedWriter{&writeMu, dst}, r, &busy, progress)
			_ = r.Close()
		}()
		return w, nil
	}
	if cmd.Stdout, err = output(inv.Stdout); err != nil {
		return fail(err)
	}
	if cmd.Stderr, err = output(inv.Stderr); err != nil {
		return fail(err)
	}
	if err = cmd.Start(); err != nil {
		return fail(err)
	}
	closeFiles(start)
	p := &Process{cmd: cmd, owned: !opt.ShareGroup, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		result := make(chan error, 1)
		go func() { result <- cmd.Wait() }()
		// Group signaling after a leader is reaped has a small PID reuse race;
		// eliminating it needs platform-specific process handles (pidfds).
		signal := func(sig os.Signal) {
			if p.owned {
				if s, ok := sig.(syscall.Signal); ok {
					_ = syscall.Kill(-cmd.Process.Pid, s)
				}
			} else {
				_ = cmd.Process.Signal(sig)
			}
		}
		killed := false
		kill := func() {
			if !killed {
				signal(syscall.SIGKILL)
				killed = true
			}
		}
		var e error
		reaped := false
		select {
		case e = <-result:
			reaped = true
		case <-ctx.Done():
			if opt.CancelGrace > 0 {
				sig := opt.CancelSignal
				if sig == nil {
					sig = syscall.SIGTERM
				}
				signal(sig)
				timer := time.NewTimer(opt.CancelGrace)
				select {
				case e = <-result:
					reaped = true
				case <-timer.C:
				}
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			// An owned group can still contain descendants after its leader exits.
			if p.owned || !reaped {
				kill()
			}
			if !reaped {
				e = <-result
				reaped = true
			}
		}
		// Optionally bound the drain when a background descendant inherited a
		// pipe; the bound restarts on output. Writers already blocked in caller
		// code must still be unblocked by its owner.
		var delay <-chan time.Time
		var timer *time.Timer
		if opt.WaitDelay > 0 {
			timer = time.NewTimer(opt.WaitDelay)
			defer timer.Stop()
			delay = timer.C
		}
		cancelCh := ctx.Done()
		if ctx.Err() != nil {
			if p.owned {
				kill()
			}
			cancelCh = nil
		} else if killed {
			cancelCh = nil
		}
		for _, d := range outputs {
			for {
				select {
				case <-d:
					goto next
				case <-cancelCh:
					if p.owned {
						kill()
					}
					cancelCh = nil
				case <-progress:
					if timer != nil {
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						timer.Reset(opt.WaitDelay)
					}
				case <-delay:
					if busy.Load() > 0 {
						timer.Reset(opt.WaitDelay)
						continue
					}
					goto drained
				}
			}
		next:
		}
	drained:
		closeFiles(wait)
		for _, d := range outputs {
			<-d
		}
		if state := cmd.ProcessState; state != nil {
			if status, ok := state.Sys().(syscall.WaitStatus); ok {
				p.result.Signaled = status.Signaled()
			}
		}
		if ctx.Err() != nil {
			p.result.Status = ContextStatus(ctx.Err())
		} else {
			p.result.Status = ExitStatus(e)
		}
	}()
	return p, nil
}

// Serialize stdout and stderr even when they share a non-thread-safe writer.
type lockedWriter struct {
	mu  *sync.Mutex
	dst io.Writer
}

func (w lockedWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dst.Write(b)
}

// copyOutput copies r to w, counting itself busy while a write is in progress
// and reporting each completed write, so a bounded drain only expires while
// every copier is idle waiting for more output.
func copyOutput(w io.Writer, r io.Reader, busy *atomic.Int32, progress chan<- struct{}) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			busy.Add(1)
			_, werr := w.Write(buf[:n])
			busy.Add(-1)
			select {
			case progress <- struct{}{}:
			default:
			}
			if werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
