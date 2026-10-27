package term

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// procRoot is the procfs the session scan walks. Tests point it at a fake tree.
var procRoot = "/proc"

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
// Unreadable or missing /proc entries are skipped. If /proc itself cannot be
// listed, the leader's process group is signalled instead. If pidfd is
// refused (Android before 12), that pid is signalled directly.
func (t *Terminal) killSession() {
	sid := t.sid
	if own, err := unix.Getsid(0); sid == 0 || err != nil || own == sid {
		t.cmd.Process.Kill()
		return
	}
	for i := 0; i < 3; i++ {
		pids, err := sessionMembers(procRoot, sid)
		if err != nil {
			t.killLeaderGroup()
			return
		}
		for _, pid := range pids {
			signalMember(pid)
		}
	}
	t.cmd.Process.Kill()
}

// sessionMembers returns pids whose /proc/<pid>/stat session field is sid.
// EACCES and missing stat files are skipped. ReadDir failure is returned so
// the caller can fall back.
func sessionMembers(root string, sid int) ([]int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		stat, err := os.ReadFile(filepath.Join(root, e.Name(), "stat"))
		if err != nil {
			continue
		}
		if s, ok := procSession(string(stat)); ok && s == sid {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func signalMember(pid int) {
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err == nil {
		unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0)
		unix.Close(pidfd)
		return
	}
	if errors.Is(err, unix.ESRCH) {
		return
	}
	unix.Kill(pid, unix.SIGKILL)
}

// killLeaderGroup is the fallback when /proc cannot be walked: the leader's
// group and the PTY's foreground group, then the leader.
func (t *Terminal) killLeaderGroup() {
	own := unix.Getpgrp()
	for _, pg := range []int{t.cmd.Process.Pid, t.foregroundGroup()} {
		if pg > 1 && pg != own {
			unix.Kill(-pg, unix.SIGKILL)
		}
	}
	t.cmd.Process.Kill()
}

func (t *Terminal) foregroundGroup() int {
	if t.ptm == nil {
		return 0
	}
	sc, err := t.ptm.SyscallConn()
	if err != nil {
		return 0
	}
	pg := 0
	sc.Control(func(fd uintptr) {
		if v, err := unix.IoctlGetInt(int(fd), unix.TIOCGPGRP); err == nil {
			pg = v
		}
	})
	return pg
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
