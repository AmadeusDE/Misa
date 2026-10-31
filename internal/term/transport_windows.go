package term

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// assignProcess is a variable so tests can force assignment failure.
var assignProcess = windows.AssignProcessToJobObject

func startProcess(spec startSpec) (conn, proc, error) {
	if len(spec.Argv) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}
	if err := conAvailable(); err != nil {
		return nil, nil, err
	}
	dir := spec.Dir
	if winDrive(dir) && (len(dir) == 2 || (dir[2] != '\\' && dir[2] != '/')) {
		return nil, nil, fmt.Errorf("drive-relative working directory %q is unsupported", dir)
	}
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return nil, nil, err
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, nil, err
	}
	env := spec.Env
	if env == nil {
		env = os.Environ()
	}
	block, err := windowsEnvBlock(env)
	if err != nil {
		return nil, nil, err
	}
	line, err := windowsCommandLine(spec.Argv)
	if err != nil {
		return nil, nil, err
	}
	exe, err := resolveWindowsExecutable(spec.Argv[0], dir, env, func(path string) bool { s, e := os.Stat(path); return e == nil && !s.IsDir() })
	if err != nil {
		return nil, nil, err
	}
	app, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return nil, nil, err
	}
	command, err := windows.UTF16PtrFromString(line)
	if err != nil {
		return nil, nil, err
	}
	cwd, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, nil, err
	}

	var owned ownership
	defer owned.close()
	var inputRead, inputWrite, outputRead, outputWrite windows.Handle
	if err = windows.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return nil, nil, err
	}
	owned.add(func() {
		if inputRead != 0 {
			windows.CloseHandle(inputRead)
		}
		if inputWrite != 0 {
			windows.CloseHandle(inputWrite)
		}
	})
	if err = windows.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		return nil, nil, err
	}
	owned.add(func() {
		if outputRead != 0 {
			windows.CloseHandle(outputRead)
		}
		if outputWrite != 0 {
			windows.CloseHandle(outputWrite)
		}
	})
	hpc, err := createConsole(spec.Rows, spec.Cols, inputRead, outputWrite)
	if err != nil {
		return nil, nil, err
	}
	owned.add(func() {
		if hpc != 0 {
			if inputWrite != 0 {
				windows.CloseHandle(inputWrite)
				inputWrite = 0
			}
			if outputRead != 0 {
				windows.CloseHandle(outputRead)
				outputRead = 0
			}
			conClose.Call(uintptr(hpc))
		}
	})
	windows.CloseHandle(inputRead)
	inputRead = 0
	windows.CloseHandle(outputWrite)
	outputWrite = 0

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, nil, err
	}
	owned.add(attrs.Delete)
	// PSEUDOCONSOLE takes the handle itself, not a pointer to a handle.
	update := conKernel.NewProc("UpdateProcThreadAttribute")
	r, _, callErr := update.Call(uintptr(unsafe.Pointer(attrs.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(hpc), unsafe.Sizeof(hpc), 0, 0)
	if r == 0 {
		return nil, nil, fmt.Errorf("UpdateProcThreadAttribute: %w", callErr)
	}
	var si windows.StartupInfoEx
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	si.ProcThreadAttributeList = attrs.List()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, err
	}
	owned.add(func() {
		if job != 0 {
			windows.CloseHandle(job)
		}
	})
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, nil, err
	}
	var pi windows.ProcessInformation
	err = windows.CreateProcess(app, command, nil, nil, false, windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT, &block[0], cwd, &si.StartupInfo, &pi)
	if err != nil {
		return nil, nil, err
	}
	owned.add(func() { windows.CloseHandle(pi.Thread) })
	processOwner := owned.add(func() {
		windows.TerminateProcess(pi.Process, 1)
		windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		windows.CloseHandle(pi.Process)
	})
	if err = assignProcess(job, pi.Process); err != nil {
		return nil, nil, fmt.Errorf("assign process to job: %w", err)
	}
	if _, err = windows.ResumeThread(pi.Thread); err != nil {
		return nil, nil, fmt.Errorf("resume process: %w", err)
	}
	c := newWindowsConn(inputWrite, outputRead, hpc, job)
	inputWrite, outputRead, hpc = 0, 0, 0
	p := &windowsProc{id: int(pi.ProcessId), process: pi.Process, conn: c, status: make(chan int, 1)}
	job = 0
	owned.release(processOwner)
	go p.wait()
	return c, p, nil
}
