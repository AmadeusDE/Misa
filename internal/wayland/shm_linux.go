package wayland

import (
	"os"

	"golang.org/x/sys/unix"
)

func anonymousFile() (*os.File, error) {
	fd, err := unix.MemfdCreate("misa-shm", unix.MFD_CLOEXEC)
	if err != nil {
		return anonymousFileInDir()
	}
	return os.NewFile(uintptr(fd), "misa-shm"), nil
}
