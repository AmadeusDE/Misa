//go:build unix

package builtins

import (
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const timeoutDefaultPath = "/bin:/usr/bin:/system/bin" // /system/bin is Android's toybox directory

func hasPathSeparator(name string) bool { return strings.ContainsRune(name, '/') }

func executableNames(_ *Context, name string) []string { return []string{name} }

func isExecutableMode(info fs.FileInfo) bool { return info.Mode()&0111 != 0 }

// timeoutProcAttr puts the command in its own process group so the signal
// reaches its children too.
func timeoutProcAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

func timeoutShell() string {
	for _, sh := range []string{"/bin/sh", "/usr/bin/sh", "/system/bin/sh"} {
		info, err := os.Stat(sh)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return sh
		}
	}
	return "/bin/sh"
}

// timeoutDeliver signals the command's group (only the command with
// foreground) and wakes it if it was stopped.
func timeoutDeliver(cmd *exec.Cmd, sig syscall.Signal, foreground bool) {
	pid := cmd.Process.Pid
	if !foreground {
		pid = -pid
	}
	_ = syscall.Kill(pid, sig)
	if sig != syscall.SIGKILL && sig != syscall.SIGCONT {
		_ = syscall.Kill(pid, syscall.SIGCONT)
	}
}
