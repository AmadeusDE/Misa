//go:build (!unix && !windows) || aix

package filelock

import (
	"errors"
	"os"
)

var errUnsupported = errors.New("file locking is unsupported on this platform")

func Open(string) (*os.File, error) { return nil, errUnsupported }
func TryLock(*os.File) error        { return errUnsupported }
func Unlock(*os.File) error         { return errUnsupported }
