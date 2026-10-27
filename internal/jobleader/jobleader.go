//go:build unix

// Package jobleader runs one command the way an interactive shell runs a job.
// The leader holds the terminal's session and puts the command in its own
// foreground process group. A program alone in its session cannot stop on ^Z
// (the kernel drops SIGTSTP for orphaned process groups); under a leader it
// can, and the leader then stops itself so whoever watches the session (Misa)
// sees the suspension, and resumes the job when it is continued.
package jobleader

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"
)

// Flag selects leader mode on the Misa binary: misa --job-leader cmd args...
const Flag = "--job-leader"

// Run starts argv as the foreground job and returns its shell exit status.
func Run(argv []string) int {
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "misa: job leader: no command")
		return 2
	}
	// Caught (not ignored) so the job gets default handling after exec while
	// the leader, a background group, is never stopped by them.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGTSTP, syscall.SIGTTIN, syscall.SIGTTOU)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if term.IsTerminal(os.Stdin.Fd()) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Foreground: true, Ctty: 0}
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "misa: %s: %v\n", argv[0], err)
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return 127
		}
		return 126
	}
	return follow(cmd.Process.Pid)
}

func follow(pid int) int {
	for {
		var ws syscall.WaitStatus
		_, err := syscall.Wait4(pid, &ws, syscall.WUNTRACED, nil)
		switch {
		case errors.Is(err, syscall.EINTR):
			continue
		case err != nil:
			return 1
		case ws.Exited():
			return ws.ExitStatus()
		case ws.Signaled():
			return 128 + int(ws.Signal())
		}
		syscall.Kill(os.Getpid(), syscall.SIGSTOP)
		syscall.Kill(-pid, syscall.SIGCONT)
	}
}

// SuspendOnCtrlZ lets this process stop on ^Z even when it is alone in its
// session, where the kernel would drop the terminal's SIGTSTP: it catches the
// signal and stops itself, as full-screen programs like Claude Code do.
func SuspendOnCtrlZ() {
	tstp := make(chan os.Signal, 1)
	signal.Notify(tstp, syscall.SIGTSTP)
	go func() {
		for range tstp {
			syscall.Kill(os.Getpid(), syscall.SIGSTOP)
		}
	}()
}
