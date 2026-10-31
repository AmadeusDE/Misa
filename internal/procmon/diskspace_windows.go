//go:build windows

package procmon

import "golang.org/x/sys/windows"

// diskSpace reports the capacity and free bytes of the volume holding path.
func diskSpace(path string) (total, free uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var avail, all, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &all, &totalFree); err != nil {
		return 0, 0, err
	}
	return all, totalFree, nil
}
