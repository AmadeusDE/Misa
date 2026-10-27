package builtins

import (
	"testing"
)

func TestTacStdin(t *testing.T) {
	r := runBuiltinStdin(t, "tac", "a\nb\nc\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "c\nb\na\n")
}

func TestTacFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "first\nsecond\nthird\n")
	r := runBuiltinIn(t, dir, "tac", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "third\nsecond\nfirst\n")
}

func TestTacSingleLine(t *testing.T) {
	r := runBuiltinStdin(t, "tac", "only\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "only\n")
}

func TestTacMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n")
	writeFile(t, dir, "b.txt", "3\n4\n")
	r := runBuiltinIn(t, dir, "tac", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "2\n1\n4\n3\n")
}

func TestTacMissing(t *testing.T) {
	r := runBuiltin(t, "tac", "nonexistent")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "tac:")
}
