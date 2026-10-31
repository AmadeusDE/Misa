//go:build !unix

package builtins

import "os/exec"

func processExitStatus(err *exec.ExitError) int {
	if code := err.ExitCode(); code >= 0 {
		return code
	}
	return ExitFailure
}
