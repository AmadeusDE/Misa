//go:build !unix && !windows

package sysinfo

import "errors"

// mountSpace has no provider on this platform: the space is unavailable.
func mountSpace(path string) (total, free uint64, err error) {
	return 0, 0, errors.New("disk space unavailable on this platform")
}
