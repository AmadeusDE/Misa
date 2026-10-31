package builtins

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// Android's shell domain often refuses hard links, FIFOs and /dev/full, and
// its patch has no GNU --batch. These probes skip only when the host cannot
// do the operation, so the same tests still run on Linux.

func skipWithoutHardlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "a")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(src, filepath.Join(dir, "b")); err != nil {
		t.Skipf("hard links are not permitted: %v", err)
	}
}

func skipWithoutFifos(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := makeFifo(filepath.Join(dir, "p"), 0o600); err != nil {
		t.Skipf("fifos are not permitted: %v", err)
	}
}

func skipWithoutGroups(t *testing.T) {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Skip(err.Error())
	}
	ids, err := u.GroupIds()
	if err != nil || len(ids) == 0 {
		if err == nil {
			err = errors.New("no groups")
		}
		t.Skip(err.Error())
	}
}

func hostPatch(t *testing.T) string {
	t.Helper()
	patch, err := exec.LookPath("patch")
	if err != nil {
		t.Skip("patch unavailable")
	}
	out, err := exec.Command(patch, "--batch", "--version").CombinedOutput()
	if err != nil && strings.Contains(strings.ToLower(string(out)), "unknown option") {
		t.Skip("patch has no --batch")
	}
	return patch
}
