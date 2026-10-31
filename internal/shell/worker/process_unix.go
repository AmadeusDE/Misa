//go:build unix

package worker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"
)

// StartProcess launches the supplied executable directly, with a private fd 3
// socket. It never invokes a host shell. The context bounds startup and the
// handshake only; use Shutdown to end the worker after StartProcess returns.
func StartProcess(ctx context.Context, options ProcessOptions) (*Client, error) {
	exe := options.Executable
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return nil, err
		}
	}
	args := options.Args
	if args == nil {
		args = []string{"sh", "--worker"}
	}
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}
	unix.CloseOnExec(pair[0])
	unix.CloseOnExec(pair[1])
	parentFile := os.NewFile(uintptr(pair[0]), "worker parent")
	child := os.NewFile(uintptr(pair[1]), "worker child")
	parent, err := net.FileConn(parentFile)
	parentFile.Close()
	if err != nil {
		child.Close()
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = options.Dir
	cmd.Env = options.Env
	streams, err := prepareStreams(cmd, options)
	if err != nil {
		parent.Close()
		child.Close()
		return nil, err
	}
	cmd.ExtraFiles = []*os.File{child}
	timeout := options.ShutdownTimeout
	if timeout <= 0 {
		timeout = time.Second
	}
	cmd.WaitDelay = timeout
	if err = cmd.Start(); err != nil {
		streams.closeAll()
		parent.Close()
		child.Close()
		return nil, err
	}
	child.Close()
	streams.start()
	wait := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); streams.closeInput(); close(wait) }()
	transport := &processControl{Conn: parent, done: wait, result: &waitErr}
	// Wait and handshake race: the child can die while a descendant retains fd 3.
	connectCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type connected struct {
		c   *Client
		err error
	}
	result := make(chan connected, 1)
	go func() { c, e := Connect(connectCtx, transport, options.Version); result <- connected{c, e} }()
	var c *Client
	select {
	case <-wait:
		parent.Close()
		cancel()
		<-result
		return nil, processExit(waitErr)
	case r := <-result:
		if r.err != nil {
			parent.Close()
			_ = cmd.Process.Kill()
			<-wait
			return nil, r.err
		}
		c = r.c
	case <-ctx.Done():
		parent.Close()
		cancel()
		_ = cmd.Process.Kill()
		<-wait
		<-result
		return nil, ctx.Err()
	}
	c.mu.Lock()
	c.shutdownTimeout = timeout
	c.mu.Unlock()
	exited := make(chan struct{})
	go func() {
		<-wait
		// Give the receiver a bounded opportunity to consume a terminal frame
		// already buffered before process exit. A descendant may retain fd 3.
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-c.Done():
		case <-timer.C:
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		c.Abort(processExit(waitErr))
		close(exited)
	}()
	c.setProcessShutdown(func(ctx context.Context) error {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-exited:
			if waitErr != nil {
				if err := c.Err(); err != nil {
					return err
				}
				return processExit(waitErr)
			}
			return nil
		case <-ctx.Done():
		case <-timer.C:
		}
		_ = cmd.Process.Kill()
		<-exited
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("shell worker shutdown timed out")
	})
	return c, nil
}

func processExit(err error) error {
	var x *exec.ExitError
	status := 0
	if errors.As(err, &x) {
		status = x.ExitCode()
	} else if err != nil {
		status = -1
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &ExitError{Status: status, Message: message, Err: err}
}

// processControl resolves a closed socket against the reaper before exposing
// transport EOF. A live child that closes its control socket must not block
// protocol failure indefinitely.
type processControl struct {
	net.Conn
	done   <-chan struct{}
	result *error
}

func (c *processControl) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err == nil || n > 0 {
		return n, err
	}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-c.done:
		return n, processExit(*c.result)
	case <-timer.C:
		return n, err
	}
}
