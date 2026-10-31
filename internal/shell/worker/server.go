package worker

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/AmadeusDE/Misa/internal/shell/session"
)

// Serve owns one session. Its context controls the worker lifetime. Closing
// control interrupts protocol I/O. A handler that
// ignores cancellation can outlive Serve; caller streams are never closed here.
func Serve(ctx context.Context, control io.ReadWriteCloser, options Options) (err error) {
	defer func() {
		if recover() != nil {
			_ = writeFrame(control, frame{Type: "crash", Status: 125, Error: "crash"})
			err = &ExitError{Status: 125, Crash: true, Message: "worker crashed"}
		}
		_ = control.Close()
	}()
	stop := context.AfterFunc(ctx, func() { _ = control.Close() })
	defer stop()
	v := options.Version
	if v == 0 {
		v = Version
	}
	hello, e := readFrame(control)
	if e != nil {
		return e
	}
	if hello.Type != "hello" || hello.Version != v || v != Version {
		_ = writeFrame(control, frame{Type: "error", Error: "version"})
		return fmt.Errorf("%w: version mismatch", ErrProtocol)
	}
	s, e := session.New(options.Session)
	if e != nil {
		_ = writeFrame(control, frame{Type: "error", Error: "session", Message: e.Error()})
		return e
	}
	defer s.Close()
	if e = writeFrame(control, frame{Type: "hello", Version: v, Capabilities: []string{"run", "cancel", "shutdown"}}); e != nil {
		return e
	}
	var mu sync.Mutex
	var active uint64
	var lastID uint64
	var activeDone chan struct{}
	var cancel context.CancelFunc
	var writes sync.Mutex
	send := func(f frame) error { writes.Lock(); defer writes.Unlock(); return writeFrame(control, f) }
	defer func() {
		mu.Lock()
		if cancel != nil {
			cancel()
		}
		mu.Unlock()
	}()
	// One owner performs every session run; the reader remains free to cancel it.
	type job struct {
		f      frame
		ctx    context.Context
		cancel context.CancelFunc
		done   chan struct{}
	}
	jobs := make(chan job)
	ownerDone := make(chan struct{})
	defer close(jobs)
	go func() {
		defer close(ownerDone)
		var current job
		defer func() {
			if recover() != nil {
				_ = send(frame{Type: "crash", ID: current.f.ID, Error: "crash", Status: 125})
				_ = control.Close()
				if current.done != nil {
					close(current.done)
				}
			}
		}()
		for j := range jobs {
			current = j
			r := s.Run(j.ctx, j.f.Script)
			j.cancel()
			mu.Lock()
			active = 0
			cancel = nil
			mu.Unlock()
			if e := send(resultFrame(j.f.ID, r)); e != nil {
				_ = control.Close()
			}
			close(j.done)
			current = job{}
		}
	}()
	for {
		f, e := readFrame(control)
		if e != nil {
			return e
		}
		switch f.Type {
		case "run":
			mu.Lock()
			if f.ID == 0 || f.ID <= lastID || active != 0 {
				mu.Unlock()
				if e = send(frame{Type: "error", ID: f.ID, Error: "busy"}); e != nil {
					return e
				}
				continue
			}
			active = f.ID
			lastID = f.ID
			done := make(chan struct{})
			activeDone = done
			runctx, c := context.WithCancel(ctx)
			cancel = c
			mu.Unlock()
			select {
			case jobs <- job{f: f, ctx: runctx, cancel: c, done: done}:
			case <-ownerDone:
				c()
				return ErrCrash
			case <-ctx.Done():
				c()
				return ctx.Err()
			}
		case "cancel":
			mu.Lock()
			if active == f.ID && cancel != nil {
				cancel()
			}
			mu.Unlock()
		case "shutdown":
			mu.Lock()
			if cancel != nil {
				cancel()
			}
			done := activeDone
			mu.Unlock()
			if done != nil {
				select {
				case <-done:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			_ = send(frame{Type: "exit"})
			return nil
		default:
			return fmt.Errorf("%w: unexpected message", ErrProtocol)
		}
	}
}
