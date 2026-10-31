//go:build windows

package procmon

import (
	"syscall"

	"golang.org/x/sys/windows"
)

var platformActions = Actions{
	Signals: windowsSignals,
	send: func(pid int, _ syscall.Signal) error {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
		if err != nil {
			return err
		}
		defer windows.CloseHandle(h)
		return windows.TerminateProcess(h, 1)
	},
}
