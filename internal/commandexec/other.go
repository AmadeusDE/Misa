//go:build !unix

package commandexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

func executable(path string) error { _, err := os.Stat(path); return err }
func ErrorStatus(err error) int {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
		return 127
	}
	return 126
}
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}
	var e *exec.ExitError
	if errors.As(err, &e) {
		return e.ExitCode()
	}
	return 125
}

type Process struct{}

func (*Process) Done() <-chan struct{}        { c := make(chan struct{}); close(c); return c }
func (*Process) Wait() int                    { return 125 }
func (*Process) WaitResult() Result           { return Result{Status: 125} }
func (*Process) Signal(os.Signal, bool) error { return errors.New("commandexec: unsupported") }
func Start(context.Context, string, Invocation, Options) (*Process, error) {
	return nil, errors.New("commandexec: unsupported")
}
func Exec(string, Invocation) int { return 125 }
