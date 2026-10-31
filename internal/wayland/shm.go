//go:build unix

package wayland

import (
	"os"

	"golang.org/x/sys/unix"
)

// SharedMemory is a mapped anonymous file to hand the compositor as a wl_shm
// pool.
type SharedMemory struct {
	File *os.File
	Data []byte
}

func NewSharedMemory(size int) (*SharedMemory, error) {
	f, err := anonymousFile()
	if err != nil {
		return nil, err
	}
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return nil, err
	}
	data, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &SharedMemory{File: f, Data: data}, nil
}

func (m *SharedMemory) Close() {
	unix.Munmap(m.Data)
	m.File.Close()
}

// anonymousFileInDir makes an unlinked file for systems without memfd.
func anonymousFileInDir() (*os.File, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "misa-shm-")
	if err != nil {
		return nil, err
	}
	os.Remove(f.Name())
	return f, nil
}
