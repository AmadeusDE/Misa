package confine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSymlinksInASharedDirStayInside(t *testing.T) {
	outside, shared := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(shared, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(shared, "out")); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFile(filepath.Join(shared, "AGENTS.md")); err != nil || string(b) != "key" {
		t.Fatalf("an unshared dir reads normally: %q %v", b, err)
	}
	release := Add(shared)
	if _, err := ReadFile(filepath.Join(shared, "AGENTS.md")); err == nil {
		t.Fatal("read followed a symlink out of the shared dir")
	}
	if f, err := OpenFile(filepath.Join(shared, "out", "planted"), os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		f.Close()
		t.Fatal("write followed a symlinked directory out of the shared dir")
	}
	if err := MkdirAll(filepath.Join(shared, "out", "d"), 0o755); err == nil {
		t.Fatal("mkdir followed a symlink out")
	}
	if err := MkdirAll(filepath.Join(shared, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFile(filepath.Join(shared, "a", "b", "ok"), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	release()
	release()
	if _, _, ok := Shared(shared); ok {
		t.Fatal("still shared after release")
	}
}
