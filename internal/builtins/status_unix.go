//go:build unix

package builtins

import (
	"os/exec"
	"syscall"
)

func processExitStatus(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return ExitSignalBase + int(status.Signal())
	}
	if code := err.ExitCode(); code >= 0 {
		return code
	}
	return ExitFailure
}
