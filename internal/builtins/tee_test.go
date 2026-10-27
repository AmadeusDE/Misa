package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTeeStdout(t *testing.T) {
	r := runBuiltinStdin(t, "tee", "hello world\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello world\n")
}

func TestTeeToFile(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinStdinIn(t, dir, "tee", "some data\n", "out.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "some data\n")
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "some data\n" {
		t.Errorf("file content = %q", data)
	}
}

func TestTeeAppend(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "out.txt", "existing\n")
	r := runBuiltinStdinIn(t, dir, "tee", "added\n", "-a", "out.txt")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing\nadded\n" {
		t.Errorf("file content = %q", data)
	}
}

func TestTeeMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinStdinIn(t, dir, "tee", "data\n", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	for _, name := range []string{"a.txt", "b.txt"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "data\n" {
			t.Errorf("%s content = %q", name, data)
		}
	}
}
