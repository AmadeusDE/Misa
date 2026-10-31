package builtins

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

const (
	ExitSuccess       = 0
	ExitFailure       = 1
	ExitUsage         = 2
	ExitTimeout       = 124
	ExitInternal      = 125
	ExitCannotExecute = 126
	ExitNotFound      = 127
	ExitSignalBase    = 128
	ExitCanceled      = 130
)

// ExitStatus maps cancellation, launch errors and process results to shell statuses.
func ExitStatus(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, context.Canceled) {
		return ExitCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ExitTimeout
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return processExitStatus(exitErr)
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return ExitNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		return ExitCannotExecute
	}
	var launchErr *exec.Error
	var pathErr *os.PathError
	if errors.As(err, &launchErr) || errors.As(err, &pathErr) {
		return ExitCannotExecute
	}
	return ExitFailure
}
