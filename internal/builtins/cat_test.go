package builtins

import "testing"

func TestCatSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "line1\nline2\nline3\n")
	r := runBuiltinIn(t, dir, "cat", "hello.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "line1\nline2\nline3\n")
}

func TestCatMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")
	r := runBuiltinIn(t, dir, "cat", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "aaa\nbbb\n")
}

func TestCatNumberLines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\nb\nc\n")
	r := runBuiltinIn(t, dir, "cat", "-n", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "     1\ta\n     2\tb\n     3\tc\n")
}

func TestCatNumberNonBlank(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\n\nb\n")
	r := runBuiltinIn(t, dir, "cat", "-b", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "     1\ta\n\n     2\tb\n")
}

func TestCatSqueezeBlank(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\n\n\n\nb\n")
	r := runBuiltinIn(t, dir, "cat", "-s", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\n\nb\n")
}

func TestCatShowEnds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "hello\n")
	r := runBuiltinIn(t, dir, "cat", "-E", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello$\n")
}

func TestCatShowEndsWithCR(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "hello\r\n")
	r := runBuiltinIn(t, dir, "cat", "-E", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello^M$\n")
}

func TestCatShowTabs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\tb\n")
	r := runBuiltinIn(t, dir, "cat", "-T", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a^Ib\n")
}

func TestCatMissingFile(t *testing.T) {
	r := runBuiltin(t, "cat", "nonexistent.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "nonexistent.txt")
}

func TestCatStdin(t *testing.T) {
	r := runBuiltinStdin(t, "cat", "from stdin\n", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "from stdin\n")
}
