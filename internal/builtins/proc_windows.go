//go:build windows

package builtins

import (
	"errors"
	"fmt"
	"io"
	"syscall"

	"golang.org/x/sys/windows"
)

const platformSignalNote = "Windows has no signals; only 0, TERM and KILL work, and both end the process at once (TERM is not graceful)"

const stillActive = 259

// sigNumberMap is what kill can deliver on Windows: a liveness check (0) and
// termination. The numbers match Go's syscall.Signal values.
var sigNumberMap = map[int]string{
	0:  "EXIT",
	9:  "KILL",
	15: "TERM",
}

func signalNumberName(n int) (string, bool) {
	name, ok := sigNumberMap[n]
	return name, ok
}

func printSignalTable(w io.Writer) {
	for _, n := range []int{0, 9, 15} {
		fmt.Fprintf(w, "%2d %s\n", n, sigNumberMap[n])
	}
}

func listedSignals() []string { return []string{"EXIT", "KILL", "TERM"} }

func signalSupported(sig syscall.Signal) bool {
	return sig == 0 || sig == syscall.SIGKILL || sig == syscall.SIGTERM
}

// sendSignal terminates the process (KILL, TERM) or checks that it is alive
// (0). The exit code is 128+signal, as a shell would report it.
func sendSignal(pid int, sig syscall.Signal) error {
	if !signalSupported(sig) {
		return fmt.Errorf("signal %d is not supported", int(sig))
	}
	access := uint32(windows.PROCESS_TERMINATE)
	if sig == 0 {
		access = windows.PROCESS_QUERY_LIMITED_INFORMATION
	}
	h, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if sig != 0 {
		return windows.TerminateProcess(h, uint32(128+int(sig)))
	}
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return err
	}
	if code != stillActive {
		return errors.New("the process has exited")
	}
	return nil
}

func killFailure(err error) string { return fmtOsErr(err) }
