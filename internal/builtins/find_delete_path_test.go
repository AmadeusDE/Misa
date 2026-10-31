// Tests informed by Toybox find.test, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD),
// and uutils findutils delete.rs, commit d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT).
package builtins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindDeleteDotRoots(t *testing.T) {
	for _, root := range []string{".", "./", "sub/."} {
		t.Run(root, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
				t.Fatal(err)
			}
			target := dir
			if root == "sub/." {
				target = filepath.Join(dir, "sub")
			}
			if err := os.WriteFile(filepath.Join(target, "file"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			_, stderr, code := findTestRun(t, dir, root, "-delete")
			if code != 0 || stderr != "" {
				t.Fatalf("%q %d", stderr, code)
			}
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("dot root deleted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target, "file")); !os.IsNotExist(err) {
				t.Fatalf("child remains: %v", err)
			}
		})
	}
}

func TestFindDeleteDotDotRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := findTestRun(t, dir, "sub/..", "-delete")
	if code == 0 || stderr == "" {
		t.Fatalf("dotdot accepted: %q %d", stderr, code)
	}
	if _, err := os.Stat(filepath.Join(dir, "sub")); err != nil {
		t.Fatalf("parent contents deleted: %v", err)
	}
}

func TestFindDeleteSymlinkLeaf(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "target")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := findTestRun(t, dir, "link", "-delete")
	if code != 0 || stderr != "" {
		t.Fatalf("%q %d", stderr, code)
	}
	if _, err := os.Lstat(filepath.Join(dir, "link")); !os.IsNotExist(err) {
		t.Fatalf("link remains: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target deleted: %v", err)
	}
}

func TestFindDeleteReplacedSymlink(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "target")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr strings.Builder
	c := &Context{Dir: dir, Stdout: &out, Stderr: &stderr, Args: []string{"file", "-exec", "swap", ";", "-delete"}}
	c.Exec = func(_ context.Context, _ string, _ *Context) int {
		if err := os.Rename(file, file+"-old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, file); err != nil {
			t.Fatal(err)
		}
		return 0
	}
	if code := runFind(c); code == 0 || stderr.Len() == 0 {
		t.Fatalf("%q %d", stderr.String(), code)
	}
	if info, err := os.Lstat(file); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("replacement removed: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target deleted: %v", err)
	}
}

func TestFindDeleteLiteralSymlinkDotDot(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "child"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []string{dir, outside} {
		if err := os.WriteFile(filepath.Join(parent, "victim"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, stderr, code := findTestRun(t, dir, "link/../victim", "-delete")
	if code != 0 || stderr != "" {
		t.Fatalf("%q %d", stderr, code)
	}
	if _, err := os.Stat(filepath.Join(outside, "victim")); !os.IsNotExist(err) {
		t.Fatalf("intended victim remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "victim")); err != nil {
		t.Fatalf("wrong victim deleted: %v", err)
	}
}

func TestFindFullPathLiteral(t *testing.T) {
	for _, tc := range []struct{ base, path, want string }{
		{"/base", "link/../victim", "/base/link/../victim"},
		{"/base/", "./x", "/base/./x"},
		{"", "./x", "./x"},
		{"/base", "/other/../x", "/other/../x"},
	} {
		if got := findFullPath(tc.base, tc.path); got != tc.want {
			t.Errorf("findFullPath(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}
