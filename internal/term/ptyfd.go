//go:build unix

package term

import (
	"os"

	"golang.org/x/sys/unix"
)

// pollable swaps a PTY master for a non-blocking, close-on-exec duplicate
// backed by the runtime poller. creack/pty leaves the master blocking, and
// Close cannot interrupt a blocked Read on a blocking file, so a session whose
// terminal side is still held open would never end.
func pollable(f *os.File) (*os.File, error) {
	sc, err := f.SyscallConn()
	if err != nil {
		return nil, err
	}
	dup := -1
	var dupErr error
	if err := sc.Control(func(fd uintptr) {
		dup, dupErr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
	}); err != nil {
		return nil, err
	}
	if dupErr != nil {
		return nil, dupErr
	}
	if err := unix.SetNonblock(dup, true); err != nil {
		unix.Close(dup)
		return nil, err
	}
	name := f.Name()
	f.Close()
	return os.NewFile(uintptr(dup), name), nil
}

// setWinsize resizes through the poller so the master stays non-blocking
// (pty.Setsize would call Fd and switch it back to blocking mode).
func setWinsize(f *os.File, rows, cols int) error {
	sc, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var ioErr error
	err = sc.Control(func(fd uintptr) {
		ioErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
	})
	if err != nil {
		return err
	}
	return ioErr
}
