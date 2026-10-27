package builtins

import (
	"testing"
)

func TestRevStdin(t *testing.T) {
	r := runBuiltinStdin(t, "rev", "hello\nworld\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "olleh\ndlrow\n")
}

func TestRevFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "abc\n123\n")
	r := runBuiltinIn(t, dir, "rev", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "cba\n321\n")
}

func TestRevEmpty(t *testing.T) {
	r := runBuiltinStdin(t, "rev", "")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestRevUnicode(t *testing.T) {
	r := runBuiltinStdin(t, "rev", "日本語\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "語本日\n")
}

func TestRevMissing(t *testing.T) {
	r := runBuiltin(t, "rev", "nonexistent")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "rev:")
}

func TestRevMissingNewline(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "line 1\n\nline 3")
	r := runBuiltinIn(t, dir, "rev", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1 enil\n\n3 enil")
}

func TestRevNulCharacter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "lin\x00e 1\n\nline 3\n")
	r := runBuiltinIn(t, dir, "rev", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1 e\x00nil\n\n3 enil\n")
}
