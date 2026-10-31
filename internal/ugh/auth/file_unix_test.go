//go:build unix

package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func assertPrivate(t *testing.T, path string, dir bool) {
	t.Helper()
	st, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	want := os.FileMode(0600)
	if dir {
		want = 0700
	}
	if st.Mode().Perm() != want {
		t.Errorf("%s perm = %o, want %o", filepath.Base(path), st.Mode().Perm(), want)
	}
}

func TestFileRefusesGroupWorldAccessible(t *testing.T) {
	f := fileBackend{path: testStorePath(t)}
	c := newCredentials()
	c.Providers["openrouter"] = Credential{Type: "api_key", Key: "sk-or-v1-abcdef0123456789"}
	if err := f.Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := os.Chmod(f.path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(); err == nil {
		t.Error("group-readable file was accepted")
	}

	if err := os.Chmod(f.path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(f.path), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(); err == nil {
		t.Error("world-searchable directory was accepted")
	}
}

func TestFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.json")
	if err := os.WriteFile(real, []byte(`{"version":1,"providers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(dir, AppDir)
	if err := os.Mkdir(appDir, 0700); err != nil {
		t.Fatal(err)
	}
	f := fileBackend{path: filepath.Join(appDir, FileName)}
	if err := os.Symlink(real, f.path); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(); err == nil {
		t.Error("symlinked credentials file was accepted")
	}
}
