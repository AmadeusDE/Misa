// The existing-file case follows uutils/coreutils (MIT),
// tests/by-util/test_ln.rs at fee61ff5d4f87e0f912a52e639942b5f7ad58663,
// test_symlink_remove_existing_same_src_and_dest. Other cases are Misa regressions.
package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestF08LnForceSelfPreservesEntry(t *testing.T) {
	for _, tc := range []struct{ kind, dest string }{{"file", "a"}, {"file", "./a"}, {"symlink", "a"}, {"symlink", "./a"}} {
		t.Run(tc.kind+"/"+tc.dest, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "a")
			if tc.kind == "file" {
				if err := os.WriteFile(path, []byte("contents"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink("missing", path); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			status := Run(nil, "ln", &Context{Dir: dir, Args: []string{"-sf", "a", tc.dest}, Stdout: &out, Stderr: &stderr})
			if status != 1 || out.String() != "" || !strings.Contains(stderr.String(), "same file") {
				t.Errorf("status=%d stdout=%q stderr=%q", status, out.String(), stderr.String())
			}
			after, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Errorf("entry replaced: before=%v after=%v", before, after)
			}
			if tc.kind == "file" {
				b, err := os.ReadFile(path)
				if err != nil || string(b) != "contents" {
					t.Errorf("contents=%q err=%v", b, err)
				}
			} else {
				target, err := os.Readlink(path)
				if err != nil || target != "missing" {
					t.Errorf("target=%q err=%v", target, err)
				}
			}
		})
	}
}

func TestF08LnForceSelfMissing(t *testing.T) {
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	status := Run(nil, "ln", &Context{Dir: dir, Args: []string{"-sf", "a", "a"}, Stdout: &out, Stderr: &stderr})
	target, err := os.Readlink(filepath.Join(dir, "a"))
	if status != 0 || out.String() != "" || stderr.String() != "" || err != nil || target != "a" {
		t.Errorf("status=%d stdout=%q stderr=%q target=%q err=%v", status, out.String(), stderr.String(), target, err)
	}
}
