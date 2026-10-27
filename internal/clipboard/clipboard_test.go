package clipboard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeExe(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestTermuxClipboardWhenOnPath(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "clip")
	writeExe(t, filepath.Join(dir, "termux-clipboard-set"), "#!/bin/sh\n/bin/cat > '"+store+"'\n")
	writeExe(t, filepath.Join(dir, "termux-clipboard-get"), "#!/bin/sh\n/bin/cat '"+store+"'\n")
	t.Setenv("PATH", dir)

	if err := Write("hello termux"); err != nil {
		t.Fatal(err)
	}
	got, err := Read()
	if err != nil || got != "hello termux" {
		t.Fatalf("read %q err %v", got, err)
	}
	raw, err := os.ReadFile(store)
	if err != nil || string(raw) != "hello termux" {
		t.Fatalf("stored %q err %v", raw, err)
	}
}

func TestNoToolReturnsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	oldW, oldR := writeSystem, readSystem
	writeSystem = func(string) error { return errors.New("no xclip") }
	readSystem = func() (string, error) { return "", errors.New("no xclip") }
	t.Cleanup(func() {
		writeSystem = oldW
		readSystem = oldR
	})
	if err := Write("x"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("write err %v", err)
	}
	if _, err := Read(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("read err %v", err)
	}
}

func TestSystemClipboardWhenTermuxAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	old := writeSystem
	var got string
	writeSystem = func(text string) error {
		got = text
		return nil
	}
	t.Cleanup(func() { writeSystem = old })
	if err := Write("via atotto"); err != nil {
		t.Fatal(err)
	}
	if got != "via atotto" {
		t.Fatalf("wrote %q", got)
	}
}
