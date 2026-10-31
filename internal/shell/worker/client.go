package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/shell/session"
)

type Client struct {
	control         io.ReadWriteCloser
	writeMu         sync.Mutex
	mu              sync.Mutex
	err             error
	done            chan struct{}
	once            sync.Once
	shutdownOnce    sync.Once
	processShutdown func(context.Context) error
	shutdownTimeout time.Duration
	capabilities    []string
	gate            chan struct{}
	next            uint64
	replies         chan frame
}

// Abort closes the control connection and seals the client. A nil error means a clean exit.
func (c *Client) Abort(err error) {
	c.once.Do(func() { c.mu.Lock(); c.err = err; c.mu.Unlock(); close(c.done); _ = c.control.Close() })
	// A process wait can refine an earlier transport failure, but never erase a crash.
	var exit *ExitError
	if errors.As(err, &exit) {
		c.mu.Lock()
		var previous *ExitError
		if (c.err == nil && exit.Status != 0) || (errors.As(c.err, &previous) && !previous.Crash) || (c.err != nil && !errors.Is(c.err, ErrCrash) && !errors.As(c.err, &previous)) {
			c.err = err
		}
		c.mu.Unlock()
	}
}

// setProcessShutdown installs the child lifecycle hook before exposing the client.
func (c *Client) setProcessShutdown(fn func(context.Context) error) {
	c.mu.Lock()
	c.processShutdown = fn
	c.mu.Unlock()
}

// Capabilities returns the server-advertised protocol capabilities.
func (c *Client) Capabilities() []string { return slices.Clone(c.capabilities) }
func (c *Client) Done() <-chan struct{}  { return c.done }
func (c *Client) Err() error             { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *Client) sendContext(ctx context.Context, f frame) error {
	result := make(chan error, 1)
	go func() {
		defer func() {
			if recover() != nil {
				result <- ErrCrash
			}
		}()
		result <- c.send(f)
	}()
	select {
	case e := <-result:
		return e
	case <-ctx.Done():
		c.Abort(ctx.Err())
		return ctx.Err()
	case <-c.done:
		return c.terminalError()
	}
}
func (c *Client) send(f frame) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return writeFrame(c.control, f)
}

// Connect performs the version handshake. The context bounds only the handshake;
// canceling it after Connect returns does not stop the client.
func Connect(ctx context.Context, control io.ReadWriteCloser, version int) (*Client, error) {
	if version == 0 {
		version = Version
	}
	stop := context.AfterFunc(ctx, func() { _ = control.Close() })
	defer stop()
	if e := writeFrame(control, frame{Type: "hello", Version: version, Capabilities: []string{"run", "cancel", "shutdown"}}); e != nil {
		_ = control.Close()
		return nil, e
	}
	f, e := readFrame(control)
	if e != nil {
		_ = control.Close()
		return nil, e
	}
	if f.Type == "error" && f.Error == "session" {
		_ = control.Close()
		return nil, fmt.Errorf("%w: worker session initialization: %s", ErrRuntime, f.Message)
	}
	if f.Type == "crash" {
		_ = control.Close()
		return nil, &ExitError{Status: f.Status, Crash: true, Message: "worker crashed"}
	}
	if f.Type != "hello" || f.Version != version || version != Version {
		_ = control.Close()
		return nil, fmt.Errorf("%w: version mismatch", ErrProtocol)
	}
	c := &Client{control: control, done: make(chan struct{}), gate: make(chan struct{}, 1), replies: make(chan frame, 1), capabilities: slices.Clone(f.Capabilities)}
	c.gate <- struct{}{}
	go c.receive()
	return c, nil
}
func (c *Client) receive() {
	defer func() {
		if recover() != nil {
			c.Abort(ErrCrash)
		}
	}()
	for {
		f, e := readFrame(c.control)
		if e != nil {
			c.Abort(e)
			return
		}
		switch f.Type {
		case "result", "error":
			select {
			case c.replies <- f:
			case <-c.done:
				return
			}
		case "crash":
			c.Abort(&ExitError{Status: f.Status, Crash: true, Message: "worker crashed"})
			return
		case "exit":
			c.Abort(nil)
			return
		default:
			c.Abort(ErrProtocol)
			return
		}
	}
}

// Start uses the same protocol as an external worker over an in-memory connection.
// Its context bounds startup only, not the lifetime of the returned client.
func Start(ctx context.Context, options Options) (*Client, error) {
	a, b := net.Pipe()
	go func() { _ = Serve(context.Background(), b, options) }()
	return Connect(ctx, a, Version)
}
func (c *Client) terminalError() error {
	if e := c.Err(); e != nil {
		return e
	}
	return session.ErrExited
}
func (c *Client) Run(ctx context.Context, script string) (session.Result, error) {
	select {
	case <-ctx.Done():
		return session.Result{}, ctx.Err()
	case <-c.done:
		return session.Result{}, c.terminalError()
	case <-c.gate:
	}
	release := true
	defer func() {
		if release {
			c.gate <- struct{}{}
		}
	}()
	select {
	case <-c.done:
		return session.Result{}, c.terminalError()
	default:
	}
	c.next++
	id := c.next
	if e := c.sendContext(ctx, frame{Type: "run", ID: id, Script: script}); e != nil {
		if !errors.Is(e, ErrProtocol) {
			c.Abort(e)
		}
		return session.Result{}, e
	}
	select {
	case f := <-c.replies:
		r, e := decodeResult(id, f)
		if e != nil {
			c.Abort(e)
		}
		return r, e
	case <-c.done:
		return session.Result{}, c.terminalError()
	case <-ctx.Done():
		// Keep the gate until the canceled run's response has been consumed.
		release = false
		go func() {
			defer func() {
				if recover() != nil {
					c.Abort(ErrCrash)
				}
				c.gate <- struct{}{}
			}()
			if e := c.send(frame{Type: "cancel", ID: id}); e != nil {
				c.Abort(e)
				return
			}
			select {
			case <-c.replies:
			case <-c.done:
			}
		}()
		return session.Result{}, ctx.Err()
	}
}
func decodeResult(id uint64, f frame) (session.Result, error) {
	if f.ID != id || f.Type != "result" {
		return session.Result{}, ErrProtocol
	}
	r := session.Result{Status: f.Status, Exited: f.Exited, ParseError: f.Syntax}
	switch f.Error {
	case "":
	case "syntax":
		if f.Syntax == nil {
			return r, ErrProtocol
		}
		r.Err = f.Syntax
	case "canceled":
		r.Err = context.Canceled
	case "deadline":
		r.Err = context.DeadlineExceeded
	case "exited":
		r.Err = session.ErrExited
	case "runtime":
		r.Err = ErrRuntime
	default:
		return r, ErrProtocol
	}
	return r, nil
}

// Cancel requests cancellation of the specified active run ID.
func (c *Client) Cancel(ctx context.Context, id uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.terminalError()
	default:
	}
	return c.sendContext(ctx, frame{Type: "cancel", ID: id})
}

// Shutdown is idempotent and bounded by ctx, including blocked control I/O.
func (c *Client) Shutdown(ctx context.Context) (err error) {
	c.mu.Lock()
	timeout := c.shutdownTimeout
	c.mu.Unlock()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	defer func() {
		c.mu.Lock()
		hook := c.processShutdown
		c.mu.Unlock()
		if hook != nil {
			if e := hook(ctx); err == nil {
				err = e
			}
		}
	}()
	select {
	case <-c.done:
		if errors.Is(c.Err(), io.EOF) {
			return nil
		}
		return c.Err()
	default:
	}
	c.shutdownOnce.Do(func() {
		go func() {
			defer func() {
				if recover() != nil {
					c.Abort(ErrCrash)
				}
			}()
			if e := c.send(frame{Type: "shutdown"}); e != nil {
				c.Abort(e)
			}
		}()
	})
	select {
	case <-c.done:
		if errors.Is(c.Err(), io.EOF) {
			return nil
		}
		return c.Err()
	case <-ctx.Done():
		c.Abort(ctx.Err())
		return ctx.Err()
	}
}
