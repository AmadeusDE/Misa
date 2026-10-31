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

func TestFindDeletePostOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tree", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tree", "sub", "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := findTestRun(t, dir, "tree", "-print", "-delete")
	if code != 0 || stderr != "" || out != "tree/sub/file\ntree/sub\ntree\n" {
		t.Fatalf("%q %q %d", out, stderr, code)
	}
	if _, err := os.Lstat(filepath.Join(dir, "tree")); !os.IsNotExist(err) {
		t.Fatalf("tree remains: %v", err)
	}
}

func TestFindDeleteRejectedCombinations(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"-L", "-delete"}, {"-H", "-delete"}, {"-delete", "-prune"}, {"-prune", "-delete"}} {
		_, stderr, code := findTestRun(t, dir, args...)
		if code == 0 || stderr == "" {
			t.Errorf("%v: %q %d", args, stderr, code)
		}
	}
}

func TestFindDepthPruneFalse(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := findTestRun(t, dir, "-depth", "-prune", "-o", "-print")
	if code != 0 || stderr != "" || out != "./f\n.\n" {
		t.Fatalf("%q %q %d", out, stderr, code)
	}
}

func TestFindPrint0Names(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a\nb", "space name", "-x"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	out, stderr, code := findTestRun(t, dir, "-mindepth", "1", "-print0")
	if code != 0 || stderr != "" || out != "./-x\x00./a\nb\x00./space name\x00" {
		t.Fatalf("%q %q %d", out, stderr, code)
	}
}

func TestFindDeletePartialFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "notempty"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notempty", "keep"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := findTestRun(t, dir, "-mindepth", "1", "-name", "keep", "-o", "-delete")
	if code != 1 || !strings.Contains(stderr, "notempty") {
		t.Fatalf("%q %d", stderr, code)
	}
	if _, err := os.Stat(filepath.Join(dir, "z")); !os.IsNotExist(err) {
		t.Fatalf("z remains: %v", err)
	}
}

func TestFindDeleteDirectorySwap(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	for _, p := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(dir, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "trigger"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "victim"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr strings.Builder
	c := &Context{Dir: dir, Stdout: &out, Stderr: &stderr, Args: []string{"-mindepth", "1", "-exec", "swap", "{}", ";", "-delete"}}
	c.Exec = func(_ context.Context, _ string, sub *Context) int {
		if strings.HasSuffix(sub.Args[len(sub.Args)-1], "/b") {
			if err := os.Rename(filepath.Join(dir, "b"), filepath.Join(dir, "old")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(dir, "b")); err != nil {
				t.Fatal(err)
			}
		}
		return 0
	}
	if code := runFind(c); code == 0 || stderr.Len() == 0 {
		t.Fatalf("swap not reported: %q %d", stderr.String(), code)
	}
	if _, err := os.Stat(filepath.Join(outside, "victim")); err != nil {
		t.Fatalf("outside victim: %v", err)
	}
}

func TestFindDeleteCancellation(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out, stderr strings.Builder
	c := &Context{Dir: dir, Ctx: ctx, Stdout: &out, Stderr: &stderr, Args: []string{"-mindepth", "1", "-exec", "cancel", "{}", ";", "-delete"}}
	c.Exec = func(_ context.Context, _ string, _ *Context) int { cancel(); return 0 }
	if code := runFind(c); code == 0 {
		t.Fatalf("cancellation not reported: %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); err != nil {
		t.Fatalf("current entry removed after cancellation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b")); err != nil {
		t.Fatalf("later entry removed: %v", err)
	}
}

func TestFindDeleteUnreadableDirectory(t *testing.T) {
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "keep"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0700)
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("process can read mode-000 directory")
	}
	_, stderr, code := findTestRun(t, dir, "-mindepth", "1", "-delete")
	if code == 0 || !strings.Contains(stderr, "locked") {
		t.Fatalf("%q %d", stderr, code)
	}
	if _, err := os.Stat(filepath.Join(dir, "z")); !os.IsNotExist(err) {
		t.Fatalf("later entry remains: %v", err)
	}
}
