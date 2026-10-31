package sysinfo

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	sysKernel            = windows.NewLazySystemDLL("kernel32.dll")
	globalMemoryStatusEx = sysKernel.NewProc("GlobalMemoryStatusEx")
	getTickCount64       = sysKernel.NewProc("GetTickCount64")
)

type memoryStatusEx struct {
	Length, MemoryLoad                                                                                   uint32
	TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
}

func windowsOS() (name, distro, kernel string) {
	version := windows.RtlGetVersion()
	// ProductType 1 is workstation; unknown types must not be rebranded as clients.
	server := version.ProductType != 1
	var product, edition, display, installation string
	var ubr uint64
	var hasUBR bool
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err == nil {
		product, _, _ = k.GetStringValue("ProductName")
		edition, _, _ = k.GetStringValue("EditionID")
		display, _, _ = k.GetStringValue("DisplayVersion")
		installation, _, _ = k.GetStringValue("InstallationType")
		var e error
		ubr, _, e = k.GetIntegerValue("UBR")
		hasUBR = e == nil
		k.Close()
	}
	if strings.Contains(strings.ToLower(installation), "server") || strings.Contains(strings.ToLower(product), "server") {
		server = true
	}
	if pk, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\ProductOptions`, registry.QUERY_VALUE); err == nil {
		typ, _, err := pk.GetStringValue("ProductType")
		pk.Close()
		if err == nil && !strings.EqualFold(typ, "WinNT") {
			server = true
		}
	}
	name, distro = windowsVersionFallback(product, edition, display, version.MajorVersion, version.MinorVersion, version.BuildNumber, ubr, hasUBR, server)
	kernel = windowsKernel(version.MajorVersion, version.MinorVersion, version.BuildNumber, ubr, hasUBR)
	return
}

func windowsFetch(cwd string) (Info, bool) {
	_, distro, kernel := windowsOS()
	mem := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ram := "?"
	if ok, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); ok != 0 {
		ram = windowsMemory(mem.TotalPhys, mem.AvailPhys)
	}
	var ticks uint64
	if getTickCount64.Find() == nil {
		low, high, _ := getTickCount64.Call()
		ticks = uint64(low)
		if unsafe.Sizeof(uintptr(0)) == 4 {
			ticks |= uint64(high) << 32
		}
	}
	return Info{Time: time.Now().Format("15:04:05"), CWD: cwd, RAM: ram, Uptime: windowsUptime(ticks), Kernel: kernel, Terminal: windowsTerminal(), Distro: distro}, true
}

func windowsCPU() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	name, _, err := k.GetStringValue("ProcessorNameString")
	name = strings.Join(strings.Fields(name), " ")
	if err != nil || name == "" {
		return ""
	}
	return fmt.Sprintf("%s (%d)", name, runtime.NumCPU())
}

func windowsStorage() string {
	size, err := windows.GetLogicalDriveStrings(0, nil)
	if err != nil || size == 0 {
		return ""
	}
	var buf []uint16
	for {
		buf = make([]uint16, size)
		n, err := windows.GetLogicalDriveStrings(uint32(len(buf)), &buf[0])
		if err != nil || n == 0 {
			return ""
		}
		if n < uint32(len(buf)) {
			buf = buf[:n]
			break
		}
		size = n + 1
	}
	var lines []string
	for len(buf) > 0 {
		i := 0
		for i < len(buf) && buf[i] != 0 {
			i++
		}
		if i == 0 {
			break
		}
		root := windows.UTF16ToString(buf[:i])
		buf = buf[i:]
		if len(buf) > 0 {
			buf = buf[1:]
		}
		p, err := windows.UTF16PtrFromString(root)
		if err != nil || windows.GetDriveType(p) != windows.DRIVE_FIXED {
			continue
		}
		total, free, err := mountSpace(root)
		if err != nil {
			continue
		}
		if line := windowsStorageLine(root, total, free); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func windowsTerminal() string {
	if term := os.Getenv("TERM_PROGRAM"); term != "" {
		return term
	}
	if os.Getenv("WT_SESSION") != "" {
		return "Windows Terminal"
	}
	return os.Getenv("TERM")
}

func windowsFetchExtended() (ExtendedInfo, bool) {
	name, _, _ := windowsOS()
	return ExtendedInfo{Title: getTitle(), OS: name, CPU: windowsCPU(), Editor: getEditor(), Storage: windowsStorage(), LocalIP: getLocalIP(), Terminal: windowsTerminal()}, true
}
