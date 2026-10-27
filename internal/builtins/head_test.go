package builtins

import (
	"fmt"
	"strings"
	"testing"
)

func twentyLines() string {
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestHeadDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", twentyLines())
	r := runBuiltinIn(t, dir, "head", "f.txt")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 10 {
		t.Errorf("got %d lines, want 10", len(lines))
	}
	assertStdoutContains(t, r, "line 1")
	assertStdoutContains(t, r, "line 10")
	if strings.Contains(r.stdout, "line 11") {
		t.Error("should not contain line 11")
	}
}

func TestHeadN(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", twentyLines())
	r := runBuiltinIn(t, dir, "head", "-n", "5", "f.txt")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5", len(lines))
	}
}

func TestHeadBytes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "abcdefghijklmnop")
	r := runBuiltinIn(t, dir, "head", "-c", "10", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcdefghij")
}

func TestHeadMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")
	r := runBuiltinIn(t, dir, "head", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "==> a.txt <==")
	assertStdoutContains(t, r, "==> b.txt <==")
	assertStdoutContains(t, r, "aaa")
	assertStdoutContains(t, r, "bbb")
}

func TestHeadStdin(t *testing.T) {
	r := runBuiltinStdin(t, "head", "one\ntwo\nthree\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "one\ntwo\nthree\n")
}

func TestHeadMissing(t *testing.T) {
	r := runBuiltin(t, "head", "nonexistent")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "head:")
}

func TestHeadNegativeN(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", b.String())
	r := runBuiltinIn(t, dir, "head", "-n", "-9", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "line 1\nline 2\nline 3\n")
}

func TestHeadNegativeBytes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "abcdefghij")
	r := runBuiltinIn(t, dir, "head", "-c", "-5", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcde")
}

func TestHeadLegacyDashNumber(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", twentyLines())
	r := runBuiltinIn(t, dir, "head", "-3", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "line 1\nline 2\nline 3\n")
}

func TestHeadDashZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", twentyLines())
	r := runBuiltinIn(t, dir, "head", "-0", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestHeadZeroTerminated(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\x00b\x00c\x00d\x00e\x00")
	r := runBuiltinIn(t, dir, "head", "-z", "-n", "3", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\x00b\x00c\x00")
}

func TestHeadErrorMessage(t *testing.T) {
	r := runBuiltin(t, "head", "no_such_file_xyz")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
	assertStderrContains(t, r, "no_such_file_xyz")
}

func TestHeadNoTrailingSeparator(t *testing.T) {
	r := runBuiltinStdin(t, "head", "abc")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc")
}

func TestHeadNegativeZero(t *testing.T) {
	r := runBuiltinStdin(t, "head", "a\nb\n", "--lines=-0")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestHeadRSuffix(t *testing.T) {
	r := runBuiltinStdin(t, "head", "", "-c", "1024R")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestHeadInvalidNumError(t *testing.T) {
	r := runBuiltinStdin(t, "head", "test", "-c", "-³")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "head: invalid number of bytes: '³'")
}

func TestHeadHugeNegativeCount(t *testing.T) {
	// Huge negative count should not panic; outputs nothing because the offset exceeds line count
	r := runBuiltinStdin(t, "head", "hello\n", "-n", "-9223372036854775807")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}
