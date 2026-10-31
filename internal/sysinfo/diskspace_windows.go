//go:build windows

package sysinfo

import "golang.org/x/sys/windows"

// mountSpace reports the capacity and free bytes of the volume holding path.
func mountSpace(path string) (total, free uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var avail, all, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &all, &totalFree); err != nil {
		return 0, 0, err
	}
	return all, avail, nil
}
