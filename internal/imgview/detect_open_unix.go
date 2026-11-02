//go:build unix

package imgview

import (
	"os"

	"golang.org/x/sys/unix"
)

// Nonblocking open prevents a file swapped for a FIFO after Stat from hanging.
func openImageCandidate(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
