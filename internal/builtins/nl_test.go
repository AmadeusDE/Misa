package builtins

import "testing"

func TestNlDefault(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\n\nworld\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "     1\thello\n       \n     2\tworld\n")
}

func TestNlBodyAll(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\n\nworld\n", "-b", "a")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "     1\thello\n     2\t\n     3\tworld\n")
}

func TestNlBodyNone(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\nworld\n", "-b", "n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "       hello\n       world\n")
}

func TestNlCustomSeparator(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\nworld\n", "-s", ": ")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "     1: hello\n     2: world\n")
}

func TestNlCustomWidth(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\n", "-w", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "  1\thello\n")
}

func TestNlFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "line1\nline2\n")
	r := runBuiltinIn(t, dir, "nl", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "     1\tline1\n     2\tline2\n")
}

func TestNlMissingFile(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "nl", "nope.txt")
	assertExitCode(t, r, 1)
}

func TestNlInvalidNumberFormat(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\n", "-ninvalid")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid value 'invalid'")
}

func TestNlWidthZero(t *testing.T) {
	r := runBuiltinStdin(t, "nl", "hello\n", "-w", "0")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid line number field width")
}
