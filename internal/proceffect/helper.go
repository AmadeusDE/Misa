package proceffect

import (
	"errors"
	"os"
	"sync/atomic"
)

// ErrUnavailable indicates that this host has not registered its multicall
// executable as a process-effect helper.
var ErrUnavailable = errors.New("process isolation unavailable")

var helperPath atomic.Pointer[string]

// EnableSelfHelper opts a multicall host in to the private helper protocol.
// The host must dispatch Flag to Handle before any ordinary command handling.
func EnableSelfHelper() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	helperPath.Store(&path)
	return nil
}

// Available reports whether the host registered its multicall executable.
func Available() bool { return helperPath.Load() != nil }

func registeredHelper() (string, error) {
	p := helperPath.Load()
	if p == nil {
		return "", ErrUnavailable
	}
	return *p, nil
}
