//go:build unix

package term

import (
	"errors"
	"syscall"
)

// waitProcess owns the child's wait status: besides its exit, it sees the
// child stop (^Z, SIGSTOP), which a plain Wait never reports. Continuing is
// tracked by Continue, since Misa is what sends SIGCONT.
func (t *Terminal) waitProcess(pid int) {
	for {
		var ws syscall.WaitStatus
		_, err := syscall.Wait4(pid, &ws, syscall.WUNTRACED, nil)
		switch {
		case errors.Is(err, syscall.EINTR):
			continue
		case err != nil:
			t.procStatus <- 1
			return
		case ws.Exited():
			t.procStatus <- ws.ExitStatus()
			return
		case ws.Signaled():
			t.procStatus <- 128 + int(ws.Signal())
			return
		}
		t.stopped.Store(true)
		t.markDirty()
	}
}

// Stopped reports whether the program is suspended, waiting for Continue.
func (t *Terminal) Stopped() bool { return t.stopped.Load() }

// Continue resumes a stopped program's whole process group, as fg and bg do.
func (t *Terminal) Continue() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return errors.New("no process to continue")
	}
	t.stopped.Store(false)
	return syscall.Kill(-t.cmd.Process.Pid, syscall.SIGCONT)
}
