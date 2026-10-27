package builtins

import "testing"

func TestFoldDefault(t *testing.T) {
	short := "hello world\n"
	r := runBuiltinStdin(t, "fold", short)
	assertExitCode(t, r, 0)
	assertStdout(t, r, short)
}

func TestFoldCustomWidth(t *testing.T) {
	r := runBuiltinStdin(t, "fold", "abcdefghij\n", "-w", "5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcde\nfghij\n")
}

func TestFoldSpaces(t *testing.T) {
	r := runBuiltinStdin(t, "fold", "hello world foo\n", "-w", "10", "-s")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "hello ")
}

func TestFoldFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "1234567890abcdef\n")
	r := runBuiltinIn(t, dir, "fold", "-w", "10", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1234567890\nabcdef\n")
}

func TestFoldMissing(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "fold", "nope.txt")
	assertExitCode(t, r, 1)
}

func TestFoldMultiLine(t *testing.T) {
	r := runBuiltinStdin(t, "fold", "abcde\n12345678901234567890\n", "-w", "10")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcde\n1234567890\n1234567890\n")
}
