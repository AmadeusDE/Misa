package builtins

import "testing"

func TestCommBasic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "apple\nbanana\ncherry\n")
	writeFile(t, dir, "b.txt", "banana\ndate\n")
	r := runBuiltinIn(t, dir, "comm", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "apple\n\t\tbanana\ncherry\n\tdate\n")
}

func TestCommSuppress1(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "apple\nbanana\n")
	writeFile(t, dir, "b.txt", "banana\ndate\n")
	r := runBuiltinIn(t, dir, "comm", "-1", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	// col2 (only in b) has no indent shift, col3 (common) has 1 tab
	assertStdout(t, r, "\tbanana\ndate\n")
}

func TestCommSuppress2(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "apple\nbanana\n")
	writeFile(t, dir, "b.txt", "banana\ndate\n")
	r := runBuiltinIn(t, dir, "comm", "-2", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	// col1 no indent, col3 has 1 tab (col2 suppressed)
	assertStdout(t, r, "apple\n\tbanana\n")
}

func TestCommSuppress3(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "apple\nbanana\n")
	writeFile(t, dir, "b.txt", "banana\ndate\n")
	r := runBuiltinIn(t, dir, "comm", "-3", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "apple\n\tdate\n")
}

func TestCommSuppress12(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "apple\nbanana\ncherry\n")
	writeFile(t, dir, "b.txt", "banana\ndate\n")
	r := runBuiltinIn(t, dir, "comm", "-12", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "banana\n")
}

func TestCommMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "x\n")
	r := runBuiltinIn(t, dir, "comm", "a.txt", "missing.txt")
	assertExitCode(t, r, 1)
}

func TestCommWrongArgCount(t *testing.T) {
	r := runBuiltin(t, "comm", "onlyonefile")
	assertExitCode(t, r, 1)
}

func TestCommDoubleDash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "apple\nbanana\n")
	writeFile(t, dir, "b.txt", "banana\ndate\n")
	r := runBuiltinIn(t, dir, "comm", "--", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "apple\n\t\tbanana\n\tdate\n")
}
