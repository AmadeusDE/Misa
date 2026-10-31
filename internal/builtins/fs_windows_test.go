//go:build windows

package builtins

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsRefusesChownAndMkfifo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "x")
	assertExitCode(t, runBuiltinIn(t, dir, "chown", "0:0", "f"), 1)
	r := runBuiltinIn(t, dir, "mkfifo", "p")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not supported")
	if _, err := os.Lstat(filepath.Join(dir, "p")); err == nil {
		t.Error("mkfifo created a node")
	}
}

func TestWindowsStatFileSystemIsRefused(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "stat", "-f", ".")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "file system information")
}

func TestWindowsStatMarksUnavailableFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "abc")
	r := runBuiltinIn(t, dir, "stat", "-c", "%s %u %U %b %h", "f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3 ? ? ? 1\n")
}

func TestWindowsTouchKeepsHundredNanosecondPrecision(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f", "x")
	want := time.Unix(1_000_000_000, 123456700)
	if err := touchChtimes(nil, "f", p, want, want, false, true); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(p)
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestWindowsTouchNoDereferenceStampsTheLink(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target", "x")
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Skipf("symlinks need Developer Mode or a privilege: %v", err)
	}
	old := time.Unix(1_000_000_000, 0)
	if err := os.Chtimes(target, old, old); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "touch", "-h", "-d", "2001-02-03 04:05:06", "link")
	assertExitCode(t, r, 0)
	li, _ := os.Lstat(link)
	if li.ModTime().Year() != 2001 {
		t.Errorf("link mtime = %v, want 2001", li.ModTime())
	}
	ti, _ := os.Stat(target)
	if !ti.ModTime().Equal(old) {
		t.Errorf("target mtime = %v, want unchanged %v", ti.ModTime(), old)
	}
}

func TestWindowsChmodReportsEffectiveMode(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f", "x")
	r := runBuiltinIn(t, dir, "chmod", "-v", "0444", "f")
	assertExitCode(t, r, 0)
	if info, _ := os.Stat(p); info.Mode().Perm()&0200 != 0 {
		t.Errorf("mode = %v, want read-only", info.Mode())
	}
	runBuiltinIn(t, dir, "chmod", "0755", "f")
	if info, _ := os.Stat(p); info.Mode().Perm()&0200 == 0 {
		t.Errorf("mode = %v, want writable", info.Mode())
	}
}

func TestWindowsMkdirParents(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mkdir", "-pv", "a/b/c")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "mkdir: created directory 'a'\nmkdir: created directory 'a/b'\nmkdir: created directory 'a/b/c'\n")
}
