//go:build unix

package vi

import (
	"os/exec"
	"syscall"
)

func shellExecutable() string { return "/bin/sh" }

func configureShellProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
