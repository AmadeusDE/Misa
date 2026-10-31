//go:build !unix && !windows

package procmon

import "errors"

// diskSpace has no provider on this platform: the space is unavailable.
func diskSpace(path string) (total, free uint64, err error) {
	return 0, 0, errors.New("disk space unavailable on this platform")
}
