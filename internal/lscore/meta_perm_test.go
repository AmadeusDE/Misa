//go:build unix

package lscore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectInaccessibleDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := Collect(dir, CollectOpts{}); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("Collect error = %v, want permission denied", err)
	}
	if _, ok := Identify(filepath.Join(dir, "f")); ok {
		t.Error("Identify below an unsearchable directory must report false")
	}
	if fi := statFile(filepath.Join(dir, "f"), "f", false); fi.Err == nil {
		t.Error("statFile below an unsearchable directory must record the error")
	}
}
