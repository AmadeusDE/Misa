package sysinfo

import (
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsNativeInvariants(t *testing.T) {
	info := Fetch(`C:\`)
	if info.CWD != `C:\` || info.Time == "" || info.Distro == "" || info.RAM == "" || info.Uptime == "" || info.Kernel == "" {
		t.Fatalf("incomplete basic info: %+v", info)
	}
	mem := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if ok, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); ok == 0 || mem.TotalPhys == 0 || mem.AvailPhys > mem.TotalPhys {
		t.Fatalf("GlobalMemoryStatusEx: ok=%d total=%d available=%d error=%v", ok, mem.TotalPhys, mem.AvailPhys, err)
	}
	if info.RAM == "?" {
		t.Fatal("Fetch did not report memory")
	}
	if err := getTickCount64.Find(); err != nil {
		t.Fatalf("GetTickCount64: %v", err)
	}
	low, high, _ := getTickCount64.Call()
	ticks := uint64(low)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		ticks |= uint64(high) << 32
	}
	if ticks == 0 || info.Uptime == "?" {
		t.Fatalf("uptime: ticks=%d display=%q", ticks, info.Uptime)
	}
	v := windows.RtlGetVersion()
	if v.BuildNumber == 0 {
		t.Fatal("RtlGetVersion returned zero build")
	}
	prefix := windowsKernel(v.MajorVersion, v.MinorVersion, v.BuildNumber, 0, false)
	if info.Kernel != prefix && !strings.HasPrefix(info.Kernel, prefix+".") {
		t.Fatalf("kernel %q does not match RtlGetVersion %q", info.Kernel, prefix)
	}
	extended := FetchExtended()
	if !strings.HasPrefix(extended.OS, "Windows") {
		t.Fatalf("OS = %q", extended.OS)
	}
	if extended.GPU != "" || extended.Shell != "" {
		t.Fatalf("unsupported fields populated: %+v", extended)
	}
	if extended.Title == "" {
		t.Fatal("missing title")
	}
}
