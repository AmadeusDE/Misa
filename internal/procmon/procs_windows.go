//go:build windows

package procmon

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// errProcGone means the process exited between enumeration and the detail
// read, so its row must be dropped rather than shown with stale fields.
var errProcGone = errors.New("process exited")

// memoryCounters mirrors PROCESS_MEMORY_COUNTERS (psapi.h); only the working
// set and the committed size are used.
type memoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// getProcessMemoryInfo resolves GetProcessMemoryInfo without cgo. Windows 7
// and later export it from kernel32 as K32GetProcessMemoryInfo; psapi.dll
// forwards the older name, so the kernel32 export is preferred.
var getProcessMemoryInfo = resolveGetProcessMemoryInfo()

func resolveGetProcessMemoryInfo() *windows.LazyProc {
	candidates := []struct{ dll, proc string }{
		{"kernel32.dll", "K32GetProcessMemoryInfo"},
		{"psapi.dll", "GetProcessMemoryInfo"},
	}
	for _, c := range candidates {
		proc := windows.NewLazySystemDLL(c.dll).NewProc(c.proc)
		if err := proc.Find(); err == nil {
			return proc
		}
	}
	return nil
}

// queryMemoryCounters fills m for the process handle h. An unsupported call is
// reported as an error so the caller keeps the row without memory values.
func queryMemoryCounters(h windows.Handle, m *memoryCounters) error {
	if getProcessMemoryInfo == nil {
		return ErrUnsupported
	}
	m.cb = uint32(unsafe.Sizeof(*m))
	r1, _, err := getProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(m)), uintptr(m.cb))
	if r1 == 0 {
		return err
	}
	return nil
}

// ReadAllProcs enumerates every process with a Toolhelp snapshot and reads the
// per-process detail the caller is allowed to see. A process that disappears
// mid-read is dropped; one that is protected or owned by another user keeps its
// row with the affected fields marked missing, never filled with zeros.
func ReadAllProcs() []Process {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	var procs []Process
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		row := procRow{
			PID:      int(entry.ProcessID),
			PPID:     int(entry.ParentProcessID),
			Name:     windows.UTF16ToString(entry.ExeFile[:]),
			Threads:  int(entry.Threads),
			BasePrio: entry.PriClassBase,
		}
		p := row.process()
		// There is no system-wide memory total before the native system
		// information provider (I03), so %MEM is not a meaningful number.
		p.Missing |= MissingMemPct

		if row.PID == 0 {
			// The idle process has no handle to open; its row is still real.
			procs = append(procs, p)
			continue
		}

		detail, err := readDetail(uint32(row.PID))
		switch {
		case errors.Is(err, errProcGone):
			continue
		case err != nil:
			procs = append(procs, p)
		default:
			procs = append(procs, p.withDetail(detail))
		}
	}
	return procs
}

// readDetail reads CPU times, memory and the executable path of one process.
// errProcGone reports a process that no longer exists.
func readDetail(pid uint32) (procDetail, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return procDetail{}, errProcGone
		}
		return procDetail{}, err
	}
	defer windows.CloseHandle(h)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return procDetail{}, err
	}

	d := procDetail{
		Kernel100ns: uint64(kernel.Nanoseconds() / 100),
		User100ns:   uint64(user.Nanoseconds() / 100),
		Identity:    uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime),
	}

	var mem memoryCounters
	if err := queryMemoryCounters(h, &mem); err == nil {
		d.WorkingSet = uint64(mem.workingSetSize)
		d.Commit = uint64(mem.pagefileUsage)
	}

	if image, err := queryImageName(h); err == nil {
		d.Image = image
	}
	if class, ok := readPriorityClass(h); ok {
		d.Class, d.HasClass = class, true
	}
	return d, nil
}

// queryImageName is the process's full executable path. Long paths need a
// buffer bigger than MAX_PATH.
func queryImageName(h windows.Handle) (string, error) {
	buf := make([]uint16, 4096)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}
