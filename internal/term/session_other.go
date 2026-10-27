//go:build !linux

package term

import "golang.org/x/sys/unix"

// killSession SIGKILLs the session leader's process group and the PTY's
// foreground group (a job started under it), then the leader. Without
// /proc there is no portable way to find background groups.
func (t *Terminal) killSession() {
	own := unix.Getpgrp()
	for _, pg := range []int{t.cmd.Process.Pid, t.foregroundGroup()} {
		if pg > 1 && pg != own {
			unix.Kill(-pg, unix.SIGKILL)
		}
	}
	t.cmd.Process.Kill()
}

// session is unused off Linux, where killSession can't scan for members.
func (t *Terminal) session() int { return 0 }

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
