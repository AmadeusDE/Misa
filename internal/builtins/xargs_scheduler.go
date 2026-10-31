package builtins

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/AmadeusDE/Misa/internal/commandexec"
	"github.com/AmadeusDE/Misa/internal/proceffect"
)

// All writes share a lock, including diagnostics: stdout and stderr may alias.
// Writes, not whole processes, are serialized so output remains streaming.
type xargsWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w xargsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

// Files are safe for concurrent writes and can be passed directly to children.
func xargsOutput(lock *sync.Mutex, w io.Writer) io.Writer {
	if _, ok := w.(*os.File); ok {
		return w
	}
	return xargsWriter{lock, w}
}

type xargsScheduler struct {
	ctx            *Context
	name           string
	limit, running int // running is owned by the producer
	done           chan struct{}
	inputCtx       context.Context
	cancelInput    context.CancelFunc
	stdout, stderr io.Writer
	mu             sync.Mutex // serializes launch against fatal completions
	result         int
	fatal          bool
}

func newXargsScheduler(ctx *Context, name string, limit int) *xargsScheduler {
	input, cancel := context.WithCancel(ctx.CancellationContext())
	lock := new(sync.Mutex)
	return &xargsScheduler{ctx: ctx, name: name, limit: limit, done: make(chan struct{}, 1), inputCtx: input, cancelInput: cancel,
		stdout: xargsOutput(lock, ctx.Stdout), stderr: xargsOutput(lock, ctx.Stderr)}
}
func (s *xargsScheduler) close() { s.cancelInput() }
func (s *xargsScheduler) stopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fatal || s.ctx.CancellationContext().Err() != nil
}
func (s *xargsScheduler) ready() bool {
	for {
		if s.stopped() {
			return false
		}
		select {
		case <-s.done:
			s.running--
			continue
		default:
		}
		if s.running < s.limit {
			return true
		}
		select {
		case <-s.done:
			s.running--
		case <-s.inputCtx.Done():
			return false
		}
	}
}
func (s *xargsScheduler) launch(sub *Context, trace bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fatal || s.ctx.CancellationContext().Err() != nil {
		return
	}
	if trace {
		// Like Toybox, -t prints each word followed by a space.
		var line strings.Builder
		for _, word := range append([]string{s.name}, sub.Args...) {
			line.WriteString(word)
			line.WriteByte(' ')
		}
		fmt.Fprintln(s.stderr, line.String())
	}
	sub.Stdout, sub.Stderr = s.stdout, s.stderr
	s.running++
	go func() {
		status := xargsExec(s.ctx, s.name, sub)
		s.mu.Lock()
		if xargsRank(status) > xargsRank(s.result) {
			s.result = status
		}
		fatal := status >= 124 && status <= 127
		if fatal {
			s.fatal = true
			s.cancelInput()
		}
		s.mu.Unlock()
		if status == 124 {
			fmt.Fprintf(s.stderr, "xargs: %s: exited with status 255; aborting\n", s.name)
		}
		s.done <- struct{}{}
	}()
}
func xargsRank(status int) int {
	switch status {
	case 124:
		return 5
	case 125:
		return 4
	case 126:
		return 3
	case 127:
		return 2
	case 123:
		return 1
	}
	return 0
}
func (s *xargsScheduler) finish(local int) int {
	for s.running > 0 {
		<-s.done
		s.running--
	}
	if err := s.ctx.CancellationContext().Err(); err != nil {
		return ExitStatus(err)
	}
	if local != 0 {
		return local
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

// Integer-only nested execution cannot distinguish normal exits 126/127 or
// 128+ from launch errors and signals. The external path has structured status.
func xargsExec(ctx *Context, name string, sub *Context) int {
	if ctx.Exec != nil {
		return xargsIntegerStatus(ctx.Exec(ctx.CancellationContext(), name, sub))
	}
	if !strings.ContainsAny(name, `/\`) {
		if _, ok := Lookup(name); ok {
			return xargsIntegerStatus(proceffect.Run(ctx.CancellationContext(), proceffect.Request{DispatchOnly: true, Argv: append([]string{name}, sub.Args...)}, proceffect.Invocation{Dir: sub.Dir, Env: sub.Env, Stdin: sub.Stdin, Stdout: sub.Stdout, Stderr: sub.Stderr}))
		}
	}
	p, err := commandexec.Start(ctx.CancellationContext(), name, commandexec.Invocation{Dir: sub.Dir, Env: sub.Env, Stdin: sub.Stdin, Stdout: sub.Stdout, Stderr: sub.Stderr, Args: sub.Args, Argv0: name}, commandexec.Options{})
	if err != nil {
		fmt.Fprintln(sub.Stderr, "xargs:", err)
		return commandexec.ErrorStatus(err)
	}
	result := p.WaitResult()
	if result.Signaled {
		return 125
	}
	if result.Status == 255 {
		return 124
	}
	if result.Status != 0 {
		return 123
	}
	return 0
}
func xargsIntegerStatus(status int) int {
	switch {
	case status == 0:
		return 0
	case status == 255:
		return 124
	case status == 126 || status == 127:
		return status
	case status >= 128:
		return 125
	default:
		return 123
	}
}
