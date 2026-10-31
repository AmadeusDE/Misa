//go:build linux

package mounts

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const (
	mountInfoPath = "/proc/self/mountinfo"
	mountsPath    = "/proc/mounts"
)

// List reads the mountinfo table, falling back to /proc/mounts when that fails.
// GOOS=android matches the linux build tag and has both files.
func List() ([]Mount, error) { return list(mountInfoPath, mountsPath) }

// list prefers the mountinfo table (it carries roots and options) and returns
// the mountinfo error when neither table can be read.
func list(mountInfoPath, mountsPath string) ([]Mount, error) {
	mounts, err := readMountTable(mountInfoPath, ParseMountInfo)
	if err == nil {
		return mounts, nil
	}
	if fallback, ferr := readMountTable(mountsPath, ParseMounts); ferr == nil {
		return fallback, nil
	}
	return nil, err
}

func readMountTable(path string, parse func(io.Reader) ([]Mount, error)) ([]Mount, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f)
}

func Statfs(path string) (Stat, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return Stat{}, err
	}
	bsize := st.Frsize
	if bsize == 0 {
		bsize = st.Bsize
	}
	return Stat{
		Blocks: st.Blocks,
		Bfree:  st.Bfree,
		Bavail: st.Bavail,
		Files:  st.Files,
		Ffree:  st.Ffree,
		Bsize:  clamp(bsize),
	}, nil
}
