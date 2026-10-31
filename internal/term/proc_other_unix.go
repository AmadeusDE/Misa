//go:build unix && !linux

package term

import "golang.org/x/sys/unix"

// killSession SIGKILLs the session leader's process group and the PTY's
// foreground group (a job started under it), then the leader. Without
// /proc there is no portable way to find background groups.
func (p *unixProc) Kill() {
	own := unix.Getpgrp()
	for _, pg := range []int{p.ID(), p.foregroundGroup()} {
		if pg > 1 && pg != own {
			unix.Kill(-pg, unix.SIGKILL)
		}
	}
	p.cmd.Process.Kill()
}

// session is unused off Linux, where killSession can't scan for members.
func (p *unixProc) session() int { return 0 }

func (p *unixProc) foregroundGroup() int {
	if p.ptm == nil {
		return 0
	}
	sc, err := p.ptm.SyscallConn()
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
