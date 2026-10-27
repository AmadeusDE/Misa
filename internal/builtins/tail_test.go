package builtins

import "testing"

func TestTailDefault(t *testing.T) {
	dir := t.TempDir()
	var lines string
	for i := 1; i <= 20; i++ {
		lines += "line" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "\n"
	}
	writeFile(t, dir, "f.txt", lines)
	r := runBuiltinIn(t, dir, "tail", "f.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "line20")
	assertStdoutContains(t, r, "line11")
	if len(r.stdout) == len(lines) {
		t.Error("expected fewer lines than full file")
	}
}

func TestTailN5(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")
	r := runBuiltinIn(t, dir, "tail", "-n", "3", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "8\n9\n10\n")
}

func TestTailSmallFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\nb\n")
	r := runBuiltinIn(t, dir, "tail", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestTailMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")
	r := runBuiltinIn(t, dir, "tail", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "==> a.txt <==")
	assertStdoutContains(t, r, "==> b.txt <==")
}

func TestTailMissing(t *testing.T) {
	r := runBuiltin(t, "tail", "nonexistent.txt")
	assertExitCode(t, r, 1)
}

func TestTailStdin(t *testing.T) {
	r := runBuiltinStdin(t, "tail", "line1\nline2\nline3\n", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "line1\nline2\nline3\n")
}
