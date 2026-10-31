package builtins

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDuBasic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", strings.Repeat("x", 4096))
	r := runBuiltinIn(t, dir, "du", ".")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, ".")
}

func TestDuSummarize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "hello")
	writeFile(t, dir, "sub/b.txt", "world")
	r := runBuiltinIn(t, dir, "du", "-s", ".")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line with -s, got %d: %q", len(lines), r.stdout)
	}
}

func TestDuHumanReadable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", strings.Repeat("x", 1024*1024))
	r := runBuiltinIn(t, dir, "du", "-sh", ".")
	assertExitCode(t, r, 0)
	// Should contain a human-readable size with K, M, or G
	out := r.stdout
	if !strings.ContainsAny(out, "KMG") {
		t.Errorf("expected human-readable size, got %q", out)
	}
}

func TestDuTotal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "data")
	r := runBuiltinIn(t, dir, "du", "-sc", ".")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "total")
}

func TestDuMissingTarget(t *testing.T) {
	r := runBuiltin(t, "du", "nonexistent_dir_12345")
	assertExitCode(t, r, 1)
}

// TestDuCountLinks verifies -l counts a hard-linked file's size at every link,
// instead of deduplicating by inode as the default behavior does.
func TestDuCountLinks(t *testing.T) {
	skipWithoutHardlinks(t)
	dir := t.TempDir()
	file1 := writeFile(t, dir, "file1", strings.Repeat("x", 1024))
	if err := os.Link(file1, filepath.Join(dir, "file1.1")); err != nil {
		t.Fatal(err)
	}

	withoutL := runBuiltinIn(t, dir, "du", "-sk", ".")
	assertExitCode(t, withoutL, 0)
	withL := runBuiltinIn(t, dir, "du", "-skl", ".")
	assertExitCode(t, withL, 0)

	sizeOf := func(r runResult) int64 {
		fields := strings.Fields(r.stdout)
		if len(fields) == 0 {
			t.Fatalf("unexpected du output: %q", r.stdout)
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			t.Fatalf("could not parse du size %q: %v", fields[0], err)
		}
		return n
	}

	base := sizeOf(withoutL)
	linked := sizeOf(withL)
	if linked <= base {
		t.Errorf("du -l should count the hard link's size again: without -l got %d, with -l got %d", base, linked)
	}
}

func TestDuNegativeMaxDepthIsError(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "du", "-d", "-1", ".")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid maximum depth '-1'")
}

func TestDuBlockSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", strings.Repeat("x", 4096))
	r := runBuiltinIn(t, dir, "du", "-s", "--block-size=1", ".")
	assertExitCode(t, r, 0)
	fields := strings.Fields(r.stdout)
	if len(fields) == 0 {
		t.Fatalf("unexpected du output: %q", r.stdout)
	}
	n, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		t.Fatalf("could not parse du size %q: %v", fields[0], err)
	}
	if n < 4096 {
		t.Errorf("expected --block-size=1 to report at least the file's own bytes, got %d", n)
	}
}

func TestDuApparentSize(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hi")
	r := runBuiltinIn(t, dir, "du", "-s", "-b", ".")
	assertExitCode(t, r, 0)
	fields := strings.Fields(r.stdout)
	if len(fields) == 0 {
		t.Fatalf("unexpected du output: %q", r.stdout)
	}
	// -b is --apparent-size --block-size=1: a 2-byte file's own contribution
	// should not be rounded up to a full disk block.
	n, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		t.Fatalf("could not parse du size %q: %v", fields[0], err)
	}
	if n >= 512 {
		t.Errorf("expected -b apparent size to reflect the tiny file content, got %d", n)
	}
}

func TestDuExclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep/file", "a")
	writeFile(t, dir, "skip/file", "b")
	r := runBuiltinIn(t, dir, "du", "--exclude=skip", "-a", ".")
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, "skip") {
		t.Errorf("expected --exclude=skip to omit the skip subtree, got %q", r.stdout)
	}
	if !strings.Contains(r.stdout, "keep") {
		t.Errorf("expected --exclude=skip to leave keep alone, got %q", r.stdout)
	}
}

func TestDuSeparateDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/file", strings.Repeat("x", 4096))
	full := runBuiltinIn(t, dir, "du", "-s", ".")
	assertExitCode(t, full, 0)
	separate := runBuiltinIn(t, dir, "du", "-S", ".")
	assertExitCode(t, separate, 0)

	sizeOf := func(out string) int64 {
		fields := strings.Fields(out)
		n, _ := strconv.ParseInt(fields[0], 10, 64)
		return n
	}
	// With -S, "." should not include "sub"'s file contribution, so its own
	// line should report a smaller value than the fully recursive -s total.
	var subLine string
	for _, line := range strings.Split(strings.TrimSpace(separate.stdout), "\n") {
		if strings.HasSuffix(line, "\t.") {
			subLine = line
		}
	}
	if subLine == "" {
		t.Fatalf("expected a line for '.' in -S output, got %q", separate.stdout)
	}
	if sizeOf(subLine) >= sizeOf(full.stdout) {
		t.Errorf("expected -S to report a smaller size for '.' than the full recursive total: -S=%d full=%d", sizeOf(subLine), sizeOf(full.stdout))
	}
}

func TestDuInodes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/a", "x")
	writeFile(t, dir, "sub/b", "y")
	r := runBuiltinIn(t, dir, "du", "--inodes", "-s", ".")
	assertExitCode(t, r, 0)
	// ".", "sub", "sub/a", "sub/b" = 4 inodes.
	assertStdoutContains(t, r, "4\t.")
}

func TestDuInaccessibleDirStillReportsOwnSize(t *testing.T) {
	skipOnWindows(t, "a Windows directory cannot be made unreadable with a mode")
	if os.Geteuid() == 0 {
		t.Skip("permission checks don't apply as root")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(sub, 0755)

	r := runBuiltinIn(t, dir, "du", "-a", ".")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stdout, "sub") {
		t.Errorf("expected du to still report a size for the unreadable dir itself, got stdout=%q stderr=%q", r.stdout, r.stderr)
	}
}
