package builtins

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStatBasic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hello world\n")
	r := runBuiltinIn(t, dir, "stat", "file.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "File: file.txt")
	assertStdoutContains(t, r, "Size:")
}

func TestStatFormatSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "12345")
	r := runBuiltinIn(t, dir, "stat", "-c", "%s", "file.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "5\n")
}

func TestStatFormatName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "stat", "-c", "%n", "file.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "file.txt\n")
}

func TestStatFormatPerms(t *testing.T) {
	skipOnWindows(t, "Windows keeps no permission bits")
	dir := t.TempDir()
	p := writeFile(t, dir, "file.txt", "data")
	os.Chmod(p, 0644)
	r := runBuiltinIn(t, dir, "stat", "-c", "%a", "file.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "644\n")
}

func TestStatFormatType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "stat", "-c", "%F", "file.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "regular file\n")
}

func TestStatFormatTypeDir(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	r := runBuiltinIn(t, dir, "stat", "-c", "%F", "subdir")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "directory\n")
}

func TestStatTerse(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hello")
	r := runBuiltinIn(t, dir, "stat", "-t", "file.txt")
	assertExitCode(t, r, 0)
	parts := strings.Fields(r.stdout)
	if len(parts) < 5 {
		t.Errorf("terse output has too few fields: %q", r.stdout)
	}
	if parts[0] != "file.txt" {
		t.Errorf("first field should be filename, got %q", parts[0])
	}
	if parts[1] != "5" {
		t.Errorf("size should be 5, got %q", parts[1])
	}
}

func TestStatMissing(t *testing.T) {
	r := runBuiltin(t, "stat", "nonexistent.txt")
	assertExitCode(t, r, 1)
}

func TestStatNoOperand(t *testing.T) {
	r := runBuiltin(t, "stat")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "the following required arguments were not provided")
}

func TestStatMultiple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa")
	writeFile(t, dir, "b.txt", "bbb")
	r := runBuiltinIn(t, dir, "stat", "-c", "%n %s", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestStatFormatInode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "stat", "-c", "%i", "file.txt")
	assertExitCode(t, r, 0)
	n, err := strconv.ParseUint(strings.TrimSpace(r.stdout), 10, 64)
	if err != nil || n == 0 {
		t.Errorf("expected positive inode, got %q", r.stdout)
	}
}

func TestStatFormatBlockSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "stat", "--format=%B", "file.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "512\n")
}
