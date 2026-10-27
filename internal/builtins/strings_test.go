package builtins

import (
	"strings"
	"testing"
)

func TestStringsBasic(t *testing.T) {
	dir := t.TempDir()
	content := "\x00\x01\x02hello world\x00\x03\x04goodbye\x00"
	writeFile(t, dir, "bin.dat", content)
	r := runBuiltinIn(t, dir, "strings", "bin.dat")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "hello world")
	assertStdoutContains(t, r, "goodbye")
}

func TestStringsMinLength(t *testing.T) {
	dir := t.TempDir()
	content := "\x00abc\x00defgh\x00ij\x00"
	writeFile(t, dir, "bin.dat", content)
	r := runBuiltinIn(t, dir, "strings", "-n", "4", "bin.dat")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "defgh")
	if strings.Contains(r.stdout, "abc") {
		t.Error("abc should be filtered out (len < 4)")
	}
	if strings.Contains(r.stdout, "ij") {
		t.Error("ij should be filtered out (len < 4)")
	}
}

func TestStringsStdin(t *testing.T) {
	r := runBuiltinStdin(t, "strings", "\x00\x00hello\x00\x00")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "hello")
}

func TestStringsMissing(t *testing.T) {
	r := runBuiltin(t, "strings", "nonexistent.bin")
	assertExitCode(t, r, 1)
}

func TestStringsShortStringsFiltered(t *testing.T) {
	dir := t.TempDir()
	content := "\x00ab\x00cd\x00ef\x00"
	writeFile(t, dir, "bin.dat", content)
	r := runBuiltinIn(t, dir, "strings", "bin.dat")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}
