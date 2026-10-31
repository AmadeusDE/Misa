//go:build unix && !aix

package filelock

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestUnixOpenProperties(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("FD_CLOEXEC: flags=%#x err=%v", flags, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("permissions: %v", info.Mode())
	}
}
