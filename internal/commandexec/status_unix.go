//go:build unix

package commandexec

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func executable(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() || st.Mode()&0111 == 0 {
		return &os.PathError{Op: "exec", Path: path, Err: syscall.EACCES}
	}
	return nil
}
func ErrorStatus(err error) int {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound) || errors.Is(err, syscall.ENOTDIR) {
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
		if s, ok := e.Sys().(syscall.WaitStatus); ok {
			if s.Signaled() {
				return 128 + int(s.Signal())
			}
			return s.ExitStatus()
		}
	}
	return 125
}
