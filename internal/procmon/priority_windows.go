//go:build windows

package procmon

import "golang.org/x/sys/windows"

// winPriorityClass maps a class to the Win32 PROCESS_*_PRIORITY_CLASS value.
func winPriorityClass(c PriorityClass) (uint32, bool) {
	switch c {
	case PriorityIdle:
		return windows.IDLE_PRIORITY_CLASS, true
	case PriorityBelowNormal:
		return windows.BELOW_NORMAL_PRIORITY_CLASS, true
	case PriorityNormal:
		return windows.NORMAL_PRIORITY_CLASS, true
	case PriorityAboveNormal:
		return windows.ABOVE_NORMAL_PRIORITY_CLASS, true
	case PriorityHigh:
		return windows.HIGH_PRIORITY_CLASS, true
	case PriorityRealtime:
		return windows.REALTIME_PRIORITY_CLASS, true
	}
	return 0, false
}

// priorityClassFromWin maps a PROCESS_*_PRIORITY_CLASS value back to a class.
func priorityClassFromWin(value uint32) (PriorityClass, bool) {
	switch value {
	case windows.IDLE_PRIORITY_CLASS:
		return PriorityIdle, true
	case windows.BELOW_NORMAL_PRIORITY_CLASS:
		return PriorityBelowNormal, true
	case windows.NORMAL_PRIORITY_CLASS:
		return PriorityNormal, true
	case windows.ABOVE_NORMAL_PRIORITY_CLASS:
		return PriorityAboveNormal, true
	case windows.HIGH_PRIORITY_CLASS:
		return PriorityHigh, true
	case windows.REALTIME_PRIORITY_CLASS:
		return PriorityRealtime, true
	}
	return PriorityNormal, false
}

// readPriorityClass reads the process's priority class with the query handle.
func readPriorityClass(h windows.Handle) (PriorityClass, bool) {
	value, err := windows.GetPriorityClass(h)
	if err != nil {
		return PriorityNormal, false
	}
	return priorityClassFromWin(value)
}

// setPriority moves a process to the priority class the nice-equivalent value
// names. Windows has no per-process nice value, so a value between two classes
// resolves to the class at or below it. Changing to Realtime is not offered:
// it needs a privilege and starves the system's own threads.
func setPriority(pid, nice int) error {
	if pid <= 0 {
		return ErrUnsupported
	}
	level, ok := PriorityLevelOf(windowsPriorityLevels, nice)
	if !ok {
		return ErrUnsupported
	}
	class, ok := winPriorityClass(level.class)
	if !ok {
		return ErrUnsupported
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.SetPriorityClass(h, class)
}
