//go:build windows

package builtins

import (
	"errors"
	"syscall"
)

const (
	errorInvalidFunction syscall.Errno = 1
	errorDirectory       syscall.Errno = 267
)

// platformErrText gives the POSIX wording for the Windows errors that the
// Unix builtins' diagnostics are written against.
func platformErrText(err error) (string, bool) {
	switch {
	case errors.Is(err, errorDirectory):
		return "Not a directory", true
	case errors.Is(err, errorInvalidFunction):
		// ReadFile on a directory handle fails this way.
		return "Is a directory", true
	}
	return "", false
}
