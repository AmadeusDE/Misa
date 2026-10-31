//go:build linux

package mounts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func writeTable(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestListFallback(t *testing.T) {
	const (
		oneInfo = "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n"
		oneMnts = "/dev/sda1 / ext4 rw,relatime 0 0\n"
	)
	missing := filepath.Join(t.TempDir(), "absent")

	t.Run("mountinfo wins", func(t *testing.T) {
		got, err := list(writeTable(t, "mountinfo", oneInfo), writeTable(t, "mounts", oneMnts+"tmpfs /tmp tmpfs rw 0 0\n"))
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 1 || got[0].Root != "/" || got[0].Options != "rw,relatime" {
			t.Fatalf("got %+v, want the single mountinfo row", got)
		}
	})
	t.Run("missing mountinfo falls back", func(t *testing.T) {
		got, err := list(missing, writeTable(t, "mounts", oneMnts))
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 1 || got[0].Device != "/dev/sda1" || got[0].Root != "" {
			t.Fatalf("got %+v, want the /proc/mounts row", got)
		}
	})
	t.Run("malformed mountinfo falls back", func(t *testing.T) {
		got, err := list(writeTable(t, "mountinfo", "22 1 8:1\n"), writeTable(t, "mounts", oneMnts))
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != 1 || got[0].Dir != "/" {
			t.Fatalf("got %+v, want the /proc/mounts row", got)
		}
	})
	t.Run("both missing reports the mountinfo error", func(t *testing.T) {
		_, err := list(missing, missing)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("err = %v, want a not-exist error", err)
		}
		if got, want := err.(*fs.PathError).Path, missing; got != want {
			t.Errorf("error path %q, want the mountinfo path %q", got, want)
		}
	})
}
