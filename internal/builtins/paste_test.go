package builtins

import "testing"

func TestPasteTwoFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n3\n")
	writeFile(t, dir, "b.txt", "a\nb\nc\n")
	r := runBuiltinIn(t, dir, "paste", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\ta\n2\tb\n3\tc\n")
}

func TestPasteCustomDelimiter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n")
	writeFile(t, dir, "b.txt", "a\nb\n")
	r := runBuiltinIn(t, dir, "paste", "-d", ",", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1,a\n2,b\n")
}

func TestPasteSerial(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n3\n")
	r := runBuiltinIn(t, dir, "paste", "-s", "a.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\t2\t3\n")
}

func TestPasteUnevenFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n3\n")
	writeFile(t, dir, "b.txt", "a\n")
	r := runBuiltinIn(t, dir, "paste", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\ta\n2\t\n3\t\n")
}

func TestPasteSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	r := runBuiltinIn(t, dir, "paste", "a.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "one\ntwo\n")
}

func TestPasteMissingFile(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "paste", "nope.txt")
	assertExitCode(t, r, 1)
}

func TestPasteSerialMultiple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n")
	writeFile(t, dir, "b.txt", "a\nb\nc\n")
	r := runBuiltinIn(t, dir, "paste", "-s", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\t2\na\tb\tc\n")
}

func TestPasteNullDelimiter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "hello\nworld\n")
	writeFile(t, dir, "b.txt", "foo\nbar\n")
	r := runBuiltinIn(t, dir, "paste", "-d", `\0`, "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hellofoo\nworldbar\n")
}

func TestPasteSerialAlternatingDelims(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "1\n2\n3\n4\n")
	r := runBuiltinIn(t, dir, "paste", "-s", "-d", `\t\n`, "a.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\t2\n3\t4\n")
}
