package term

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func (t *Terminal) ptyIoctl(req uint, out *int32) bool {
	sc, err := t.ptm.SyscallConn()
	if err != nil {
		return false
	}
	ok := false
	sc.Control(func(fd uintptr) {
		_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(unsafe.Pointer(out)))
		ok = e == 0
	})
	return ok
}

// session is the PTY's session id when the leader we started owns it.
// The PTY forgets its session once the leader exits, so it is read at start.
func (t *Terminal) session() int {
	var tty int32
	if t.ptm == nil || !t.ptyIoctl(unix.TIOCGSID, &tty) || int(tty) != t.cmd.Process.Pid {
		return 0
	}
	return int(tty)
}

// killSession SIGKILLs every process in the PTY's session (foreground and
// background job groups), not just the leader, even after the leader exited.
// Linux never hands out a session's id as a pid while a member is alive, so
// the scan can't hit an unrelated session; signals go through pidfds so a pid
// recycled mid-scan is never hit. It never targets the host's own session.
func (t *Terminal) killSession() {
	sid := t.sid
	if own, err := unix.Getsid(0); sid == 0 || err != nil || own == sid {
		t.cmd.Process.Kill()
		return
	}
	for i := 0; i < 3; i++ {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			break
		}
		for _, e := range entries {
			pid, err := strconv.Atoi(e.Name())
			if err != nil || pid <= 0 {
				continue
			}
			pidfd, err := unix.PidfdOpen(pid, 0)
			if err != nil {
				continue
			}
			if stat, err := os.ReadFile("/proc/" + e.Name() + "/stat"); err == nil {
				if s, ok := procSession(string(stat)); ok && s == sid {
					unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0)
				}
			}
			unix.Close(pidfd)
		}
	}
	t.cmd.Process.Kill()
}

// procSession extracts the session id (field 6) from /proc/<pid>/stat. comm is
// parenthesised and may itself contain spaces or parentheses, so fields are
// counted after the last closing parenthesis.
func procSession(stat string) (int, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, false
	}
	fields := strings.Fields(stat[i+1:])
	if len(fields) < 4 {
		return 0, false
	}
	sid, err := strconv.Atoi(fields[3])
	return sid, err == nil
}
