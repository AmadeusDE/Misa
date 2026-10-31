//go:build unix

package term

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// ptyConn is the master side of a PTY.
type ptyConn struct{ *os.File }

func (c ptyConn) Resize(rows, cols int) error { return setWinsize(c.File, rows, cols) }

// startProcess runs argv on a new PTY session.
func startProcess(spec startSpec) (conn, proc, error) {
	cmd := exec.Command(spec.Argv[0], spec.Argv[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(spec.Rows), Cols: uint16(spec.Cols)})
	if err != nil {
		return nil, nil, err
	}
	if p, err := pollable(f); err == nil {
		f = p
	}
	p := &unixProc{cmd: cmd, ptm: f, status: make(chan int, 1), onStop: spec.OnStop}
	p.sid = p.session()
	go p.wait()
	return ptyConn{f}, p, nil
}

// openFuncTTY opens a PTY for an in-process program: the core gets the master,
// the program the tty. A pipe-pair or ConPTY transport would instead hand the
// program separate reader and writer files plus tty metadata (see
// docs/plans/windows.md), which is why this stays behind the transport.
func openFuncTTY(rows, cols int) (conn, *os.File, error) {
	ptm, tty, err := pty.Open()
	if err != nil {
		return nil, nil, err
	}
	pty.Setsize(ptm, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	f, err := pollable(ptm)
	if err != nil {
		tty.Close()
		return nil, nil, err
	}
	return ptyConn{f}, tty, nil
}

// unixProc is a child that leads its own session on the PTY.
type unixProc struct {
	cmd    *exec.Cmd
	ptm    *os.File
	sid    int // the PTY's session, sampled at start (0: unknown)
	status chan int
	onStop func()
}

func (p *unixProc) ID() int            { return p.cmd.Process.Pid }
func (p *unixProc) Status() <-chan int { return p.status }

// wait owns the child's wait status: besides its exit, it sees the child stop
// (^Z, SIGSTOP), which a plain Wait never reports. Continuing is tracked by
// Terminal.Continue, since Misa is what sends SIGCONT.
func (p *unixProc) wait() {
	for {
		var ws syscall.WaitStatus
		_, err := syscall.Wait4(p.ID(), &ws, syscall.WUNTRACED, nil)
		switch {
		case errors.Is(err, syscall.EINTR):
			continue
		case err != nil:
			p.status <- 1
			return
		case ws.Exited():
			p.status <- ws.ExitStatus()
			return
		case ws.Signaled():
			p.status <- 128 + int(ws.Signal())
			return
		}
		if p.onStop != nil {
			p.onStop()
		}
	}
}

// Continue resumes the whole process group.
func (p *unixProc) Continue() error { return syscall.Kill(-p.ID(), syscall.SIGCONT) }

// ExitStatus is a process's status the way a shell reports it: 128+signal for
// a process killed by a signal.
func ExitStatus(err error) int {
	ee, ok := err.(*exec.ExitError)
	switch {
	case err == nil:
		return 0
	case !ok:
		return 1
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ee.ExitCode()
}
