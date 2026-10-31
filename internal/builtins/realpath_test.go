package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealpathRegularFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hi")
	r := runBuiltinIn(t, dir, "realpath", "hello.txt")
	assertExitCode(t, r, 0)
	got := strings.TrimSpace(r.stdout)
	want := filepath.Join(dir, "hello.txt")
	if got != want {
		t.Errorf("realpath = %q, want %q", got, want)
	}
}

func TestRealpathMissing(t *testing.T) {
	r := runBuiltin(t, "realpath")
	assertExitCode(t, r, 1)
}

func TestRealpathNonexistent(t *testing.T) {
	r := runBuiltin(t, "realpath", "/no/such/path")
	assertExitCode(t, r, 1)
}

func TestRealpathCanonMissing(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "-m", "/no/such/path")
	assertExitCode(t, r, 0)
	// A rooted path stays on the volume of the working directory.
	assertStdout(t, r, filepath.VolumeName(dir)+filepath.FromSlash("/no/such/path")+"\n")
}

func TestReadlinkSymlink(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target.txt", "data")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "readlink", "link.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, target+"\n")
}

func TestReadlinkNotSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "regular.txt", "data")
	r := runBuiltinIn(t, dir, "readlink", "regular.txt")
	assertExitCode(t, r, 1)
}

func TestReadlinkCanonicalize(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target.txt", "data")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "readlink", "-f", "link.txt")
	assertExitCode(t, r, 0)
	got := strings.TrimSpace(r.stdout)
	if got != target {
		t.Errorf("readlink -f = %q, want %q", got, target)
	}
}

func TestReadlinkMissing(t *testing.T) {
	r := runBuiltin(t, "readlink")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "the following required arguments were not provided")
}

func TestRealpathNonexistentLastComponent(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "does-not-exist")
	assertExitCode(t, r, 0)
	got := strings.TrimSpace(r.stdout)
	want := filepath.Join(dir, "does-not-exist")
	if got != want {
		t.Errorf("realpath = %q, want %q", got, want)
	}
}

func TestRealpathExistingMode(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "-e", "does-not-exist")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file")
}

func TestRealpathSymlinkToNonexistent(t *testing.T) {
	dir := t.TempDir()
	os.Symlink("not_file", filepath.Join(dir, "link1"))
	r := runBuiltinIn(t, dir, "realpath", "link1")
	assertExitCode(t, r, 0)
	got := strings.TrimSpace(r.stdout)
	want := filepath.Join(dir, "not_file")
	if got != want {
		t.Errorf("realpath = %q, want %q", got, want)
	}
}

func TestRealpathSymlinkBadParent(t *testing.T) {
	dir := t.TempDir()
	os.Symlink("bad/not_file", filepath.Join(dir, "link2"))
	r := runBuiltinIn(t, dir, "realpath", "link2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file")
}

func TestRealpathEmptyString(t *testing.T) {
	r := runBuiltin(t, "realpath", "")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, ": No such file or directory")
}

func TestRealpathMissingParent(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "no/such/dir/file")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
}

func TestRealpathTrailingSlashNonexistent(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "does-not-exist/")
	assertExitCode(t, r, 0)
	got := strings.TrimSpace(r.stdout)
	want := filepath.Join(dir, "does-not-exist")
	if got != want {
		t.Errorf("realpath = %q, want %q", got, want)
	}
}

func TestRealpathEmptyRelativeTo(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "--relative-to=", ".")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, ": No such file or directory")
}

func TestRealpathEmptyRelativeBase(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "realpath", "--relative-base=", ".")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, ": No such file or directory")
}
