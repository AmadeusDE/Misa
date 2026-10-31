//go:build unix

package builtins

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestReadFSMetaMatchesStat(t *testing.T) {
	path := writeFile(t, t.TempDir(), "f", "hello")
	info := mustStat(t, path)
	st := info.Sys().(*syscall.Stat_t)
	m, ok := readFSMeta(path, info)
	if !ok {
		t.Fatal("readFSMeta failed")
	}
	if m.Missing != 0 {
		t.Errorf("Unix reports every field, Missing = %b", m.Missing)
	}
	if m.Ino != st.Ino || m.Dev != uint64(st.Dev) || m.Nlink != uint64(st.Nlink) || m.Blocks != st.Blocks || m.Uid != st.Uid || m.Gid != st.Gid || m.Mode != uint32(st.Mode) {
		t.Errorf("fsMeta %+v differs from %+v", m, st)
	}
	if got := time.Unix(m.Mtime.Sec, m.Mtime.Nsec); !got.Equal(info.ModTime()) {
		t.Errorf("mtime = %v, want %v", got, info.ModTime())
	}
}

func TestStatFormatOnUnixMatchesOS(t *testing.T) {
	path := writeFile(t, t.TempDir(), "f", "hello")
	r := runBuiltinIn(t, filepath.Dir(path), "stat", "-c", "%s %h %Y", "f")
	assertExitCode(t, r, 0)
	info := mustStat(t, path)
	want := "5 1 " + strconv.FormatInt(info.ModTime().Unix(), 10) + "\n"
	if r.stdout != want {
		t.Errorf("stat -c = %q, want %q", r.stdout, want)
	}
}

func TestTouchNoDereferenceStampsTheLinkOnly(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target", "x")
	old := time.Unix(1_000_000_000, 123456789)
	if err := os.Chtimes(target, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "touch", "-h", "-d", "2001-02-03 04:05:06", "link")
	assertExitCode(t, r, 0)
	li, _ := os.Lstat(filepath.Join(dir, "link"))
	if li.ModTime().Year() != 2001 {
		t.Errorf("link mtime = %v, want 2001", li.ModTime())
	}
	ti, _ := os.Stat(target)
	if !ti.ModTime().Equal(old) {
		t.Errorf("target mtime = %v, want unchanged %v", ti.ModTime(), old)
	}
}

func TestTouchAccessOnlyKeepsModificationTime(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f", "x")
	old := time.Unix(1_000_000_000, 987654321)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "touch", "-a", "-d", "2001-02-03 04:05:06", "f"), 0)
	info, _ := os.Stat(p)
	if !info.ModTime().Equal(old) {
		t.Errorf("mtime = %v, want %v kept", info.ModTime(), old)
	}
	a, _, _ := fsTimes(info)
	if a.Year() != 2001 {
		t.Errorf("atime = %v, want 2001", a)
	}
}

func fsTimes(info os.FileInfo) (a, m, c time.Time) {
	st := info.Sys().(*syscall.Stat_t)
	return time.Unix(int64(rawAtim(st).Sec), int64(rawAtim(st).Nsec)), info.ModTime(), time.Unix(int64(rawCtim(st).Sec), int64(rawCtim(st).Nsec))
}

func TestCpPreservesTimestampsWithP(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "src", "x")
	old := time.Unix(1_000_000_000, 123456789)
	if err := os.Chtimes(src, old, old); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "cp", "-p", "src", "dst"), 0)
	info, _ := os.Stat(filepath.Join(dir, "dst"))
	if !info.ModTime().Equal(old) {
		t.Errorf("dst mtime = %v, want %v", info.ModTime(), old)
	}
}

func TestCpNoDereferenceCopiesTheLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "target", "x")
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "cp", "-P", "link", "copy"), 0)
	if fi, err := os.Lstat(filepath.Join(dir, "copy")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("cp -P did not create a symlink: %v %v", fi, err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "cp", "-L", "link", "deref"), 0)
	if fi, err := os.Lstat(filepath.Join(dir, "deref")); err != nil || !fi.Mode().IsRegular() {
		t.Errorf("cp -L did not create a regular file: %v %v", fi, err)
	}
}

func TestMvOverwritesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "new")
	writeFile(t, dir, "b", "old")
	assertExitCode(t, runBuiltinIn(t, dir, "mv", "a", "b"), 0)
	data, _ := os.ReadFile(filepath.Join(dir, "b"))
	if string(data) != "new" {
		t.Errorf("b = %q, want new", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Errorf("a still exists: %v", err)
	}
}

func TestDuCountsHardLinkOnce(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a", "hello")
	if err := os.Link(p, filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "du", "-a", "-b", ".")
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, "\ta\n") == strings.Contains(r.stdout, "\tb\n") {
		t.Errorf("du listed both names of one inode:\n%s", r.stdout)
	}
}
