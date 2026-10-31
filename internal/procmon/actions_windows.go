//go:build windows

package procmon

import (
	"syscall"

	"golang.org/x/sys/windows"
)

var platformActions = Actions{
	Signals: windowsSignals,
	Levels:  windowsPriorityLevels,
	renice:  setPriority,
	send: func(pid int, _ syscall.Signal) error {
		if pid <= 0 {
			return ErrUnsupported
		}
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
		if err != nil {
			return err
		}
		defer windows.CloseHandle(h)
		return windows.TerminateProcess(h, 1)
	},
}
