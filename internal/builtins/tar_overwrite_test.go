// Tar overwrite options informed by Toybox tar.c (0BSD).
package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTarKeepOldLongOptions(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("archive"), 0600); err != nil {
		t.Fatal(err)
	}
	code, data, diag := tarRun(src, nil, "-c", "f")
	if code != 0 {
		t.Fatalf("create: %d %s", code, diag)
	}
	for _, flag := range []string{"--keep-old", "--keep-old-files"} {
		t.Run(flag, func(t *testing.T) {
			dst := t.TempDir()
			if err := os.WriteFile(filepath.Join(dst, "f"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			code, _, diag := tarRun(dst, bytes.NewReader(data), "-x", flag)
			got, err := os.ReadFile(filepath.Join(dst, "f"))
			if code != 2 || err != nil || string(got) != "old" || diag == "" {
				t.Fatalf("keep: %d %q %q %v", code, got, diag, err)
			}
			code, _, diag = tarRun(dst, bytes.NewReader(data), "-x", "--overwrite")
			got, err = os.ReadFile(filepath.Join(dst, "f"))
			if code != 0 || err != nil || string(got) != "archive" {
				t.Fatalf("overwrite: %d %q %q %v", code, got, diag, err)
			}
		})
	}
}

func TestTarOverwriteDoesNotFollowDestination(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("archive"), 0600); err != nil {
		t.Fatal(err)
	}
	code, data, diag := tarRun(src, nil, "-c", "f")
	if code != 0 {
		t.Fatalf("create: %d %s", code, diag)
	}
	for _, link := range []string{"symlink", "hardlink"} {
		t.Run(link, func(t *testing.T) {
			dst, outside := t.TempDir(), t.TempDir()
			target := filepath.Join(outside, "target")
			if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			if link == "symlink" {
				err = os.Symlink(target, filepath.Join(dst, "f"))
			} else {
				err = os.Link(target, filepath.Join(dst, "f"))
			}
			if err != nil {
				t.Skip(err)
			}
			code, _, diag := tarRun(dst, bytes.NewReader(data), "-x", "--overwrite")
			got, err := os.ReadFile(filepath.Join(dst, "f"))
			if code != 0 || err != nil || string(got) != "archive" {
				t.Fatalf("extract: %d %q %s %v", code, got, diag, err)
			}
			got, err = os.ReadFile(target)
			if err != nil || string(got) != "untouched" {
				t.Fatalf("outside target changed: %q %v", got, err)
			}
		})
	}
}
