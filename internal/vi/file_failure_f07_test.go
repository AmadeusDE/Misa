package vi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileF07WriteFailureRetainsSnapshot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	e, path := f07Editor(t, "original\n")
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Error(err)
		}
	})
	before := e.file.info
	saved := e.savedRevision
	f07Keys(t, e, "x:w<CR>")
	if !e.modified || e.savedRevision != saved || e.file.info != before {
		t.Error("failed temporary-file creation changed saved state")
	}
	if e.statusMsg == "" || strings.Contains(e.statusMsg, "written") {
		t.Errorf("write error not reported: %q", e.statusMsg)
	}
	if got := f07Read(t, path); got != "original\n" {
		t.Errorf("failed save changed file: %q", got)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f07Keys(t, e, ":w<CR>")
	if e.modified || f07Read(t, path) != "riginal\n" {
		t.Errorf("retry did not save: dirty=%v status=%q", e.modified, e.statusMsg)
	}
}
