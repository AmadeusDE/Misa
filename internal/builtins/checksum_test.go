package builtins

import (
	"strings"
	"testing"
)

func TestMd5sumFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	r := runBuiltinIn(t, dir, "md5sum", "hello.txt")
	assertExitCode(t, r, 0)
	// md5("hello\n") = b1946ac92492d2347c6235b4d2611184
	assertStdoutContains(t, r, "b1946ac92492d2347c6235b4d2611184")
	assertStdoutContains(t, r, "hello.txt")
}

func TestSha256sumFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	r := runBuiltinIn(t, dir, "sha256sum", "hello.txt")
	assertExitCode(t, r, 0)
	// sha256("hello\n") = 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	assertStdoutContains(t, r, "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")
}

func TestSha1sumFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	r := runBuiltinIn(t, dir, "sha1sum", "hello.txt")
	assertExitCode(t, r, 0)
	// sha1("hello\n") = f572d396fae9206628714fb2ce00f72e94f2258f
	assertStdoutContains(t, r, "f572d396fae9206628714fb2ce00f72e94f2258f")
}

func TestSha512sumFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	r := runBuiltinIn(t, dir, "sha512sum", "hello.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "hello.txt")
	// Just verify it produces something that looks like a sha512 hash (128 hex chars)
	parts := strings.Fields(r.stdout)
	if len(parts) < 1 || len(parts[0]) != 128 {
		t.Errorf("expected 128-char hex hash, got %q", parts[0])
	}
}

func TestChecksumTwoSpaces(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "data")
	r := runBuiltinIn(t, dir, "md5sum", "f.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "  f.txt")
}

func TestChecksumCheckMode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	writeFile(t, dir, "sums.txt", "b1946ac92492d2347c6235b4d2611184  hello.txt\n")
	r := runBuiltinIn(t, dir, "md5sum", "-c", "sums.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "OK")
}

func TestChecksumCheckModeFail(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	writeFile(t, dir, "sums.txt", "0000000000000000000000000000000  hello.txt\n")
	r := runBuiltinIn(t, dir, "md5sum", "-c", "sums.txt")
	// invalid line (wrong hash format length won't match)
	if r.code == 0 {
		t.Error("expected non-zero exit for bad checksum")
	}
}

func TestChecksumStdin(t *testing.T) {
	r := runBuiltinStdin(t, "md5sum", "hello\n", "-")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "b1946ac92492d2347c6235b4d2611184")
}

func TestChecksumMissingFile(t *testing.T) {
	r := runBuiltin(t, "sha256sum", "nonexistent.txt")
	assertExitCode(t, r, 1)
}
