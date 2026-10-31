//go:build windows

package builtins

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
)

// signalNames holds the names Go's Windows syscall package defines. Only
// signalSupported ones can be delivered: Windows has no signals, so the
// delivery is TerminateProcess.
var signalNames = map[string]syscall.Signal{
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"QUIT": syscall.SIGQUIT,
	"ILL":  syscall.SIGILL,
	"TRAP": syscall.SIGTRAP,
	"ABRT": syscall.SIGABRT,
	"BUS":  syscall.SIGBUS,
	"FPE":  syscall.SIGFPE,
	"KILL": syscall.SIGKILL,
	"SEGV": syscall.SIGSEGV,
	"PIPE": syscall.SIGPIPE,
	"ALRM": syscall.SIGALRM,
	"TERM": syscall.SIGTERM,
}

func signalByName(name string) (syscall.Signal, bool) {
	name = strings.TrimPrefix(strings.ToUpper(name), "SIG")
	if sig, ok := signalNames[name]; ok {
		return sig, true
	}
	n, err := strconv.Atoi(name)
	if err == nil && n > 0 && n < 65 {
		return syscall.Signal(n), true
	}
	return 0, false
}

type procInfo struct {
	pid     int
	comm    string
	cmdline string
	uid     uint32
	ppid    int
}

var errProcListUnsupported = errors.New("listing processes is not supported on Windows")

func scanProcs() ([]procInfo, error) { return nil, errProcListUnsupported }
