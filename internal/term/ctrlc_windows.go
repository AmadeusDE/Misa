package term

import (
	"sync"

	"golang.org/x/sys/windows"
)

// statusControlCExit is the exit code of a console process that took the
// default action for CTRL_C_EVENT (ExitProcess(STATUS_CONTROL_C_EXIT)).
const statusControlCExit = 0xC000013A

// consoleIgnoreCtrlC is the bit kernelbase keeps in the process parameters'
// ConsoleFlags for SetConsoleCtrlHandler(NULL, TRUE).
const consoleIgnoreCtrlC = 1

var (
	conSetCtrlHandler = conKernel.NewProc("SetConsoleCtrlHandler")
	ctrlCMu           sync.Mutex
)

// shellStatus maps a native exit code to the status a shell reports, so a
// child ended by ^C on its pseudoconsole reads as 130, as SIGINT does on Unix.
func shellStatus(code uint32) int {
	if code == statusControlCExit {
		return 130
	}
	return int(code)
}

// ignoringCtrlC reports whether this process ignores CTRL+C.
func ignoringCtrlC() bool {
	params := windows.RtlGetCurrentPeb().ProcessParameters
	return params != nil && params.ConsoleFlags&consoleIgnoreCtrlC != 0
}

// withCtrlCEnabled runs create (a CreateProcess call) with CTRL+C processing
// enabled in this process. CreateProcess copies the caller's ignore-CTRL+C flag
// into the child, and a process started under one that ignores it (a
// CREATE_NEW_PROCESS_GROUP launcher such as a CI runner or IDE) would then
// ignore the ^C its own pseudoconsole turns into CTRL_C_EVENT. A shell gives
// its foreground children a working interrupt, so the pane child must not
// inherit Misa's own disposition. The flag is restored once the child exists.
func withCtrlCEnabled(create func() error) error {
	ctrlCMu.Lock()
	defer ctrlCMu.Unlock()
	if !ignoringCtrlC() {
		return create()
	}
	if r, _, _ := conSetCtrlHandler.Call(0, 0); r == 0 {
		// Best effort: without the call the child keeps the inherited flag.
		return create()
	}
	defer conSetCtrlHandler.Call(0, 1)
	return create()
}
