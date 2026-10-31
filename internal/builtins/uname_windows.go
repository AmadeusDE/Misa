//go:build windows

package builtins

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetNativeSystemInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetNativeSystemInfo")

// nativeMachine reports the machine the OS runs on, not the one this binary
// was built for (an amd64 build may run on arm64 under emulation).
func nativeMachine() string {
	var info [64]byte // SYSTEM_INFO; wProcessorArchitecture is the first field
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&info[0])))
	return machineFromProcessorArchitecture(uint16(info[0]) | uint16(info[1])<<8)
}

func readUname() (unameInfo, error) {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	v := windows.RtlGetVersion()
	return unameInfo{
		sysname:  "Windows_NT",
		nodename: host,
		release:  fmt.Sprintf("%d.%d", v.MajorVersion, v.MinorVersion),
		version:  fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber),
		machine:  nativeMachine(),
		osName:   "Windows",
	}, nil
}
