package builtins

import (
	"testing"
)

func TestWcDefault(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "hello world\nfoo bar baz\n")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "2")
	assertStdoutContains(t, r, "5")
	assertStdoutContains(t, r, "24")
}

func TestWcLines(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "a\nb\nc\n", "-l")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestWcWords(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "one two three\n", "-w")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestWcBytes(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "abc\n", "-c")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "4\n")
}

func TestWcFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "one two\nthree\n")
	r := runBuiltinIn(t, dir, "wc", "-l", "f.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "2")
	assertStdoutContains(t, r, "f.txt")
}

func TestWcMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "a b\n")
	writeFile(t, dir, "b.txt", "c d e\n")
	r := runBuiltinIn(t, dir, "wc", "-w", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "a.txt")
	assertStdoutContains(t, r, "b.txt")
	assertStdoutContains(t, r, "total")
}

func TestWcMaxLineLength(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "short\na longer line\n", "-L")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "13")
}

func TestWcColumnOrderCharsBeforeBytes(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "ab\n", "-cm")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "      3       3\n")
}

func TestWcColumnOrderAllCounters(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "ab\n", "-lwmcL")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "      1       1       3       3       2\n")
}

func TestWcWidthSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\n")
	r := runBuiltinIn(t, dir, "wc", "-lwc", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1 1 2 f.txt\n")
}

func TestWcSingleCounterNoPadding(t *testing.T) {
	r := runBuiltinStdin(t, "wc", "a b c\n", "-w")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestWcErrorMessage(t *testing.T) {
	r := runBuiltin(t, "wc", "bogusfile")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "bogusfile")
	assertStderrContains(t, r, "No such file or directory")
}
