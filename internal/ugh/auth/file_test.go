package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func testStorePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), AppDir, FileName)
}

func TestFileRoundtripAndPermissions(t *testing.T) {
	f := fileBackend{path: testStorePath(t)}
	// Nothing stored yet: empty, and no directory created by a read.
	c, err := f.Load()
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if len(c.Providers) != 0 {
		t.Fatalf("expected empty store, got %d", len(c.Providers))
	}

	want := Credential{Type: "api_key", Key: "sk-or-v1-abcdef0123456789"}
	c.Providers["openrouter"] = want
	if err := f.Save(c); err != nil {
		t.Fatalf("save: %v", err)
	}

	dir := filepath.Dir(f.path)
	assertPrivate(t, dir, true)
	assertPrivate(t, f.path, false)
	// Atomic write leaves no staging files behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != FileName {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir entries = %v, want only %s", names, FileName)
	}

	got, err := f.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Providers["openrouter"] != want {
		t.Errorf("roundtrip mismatch: %+v", got.Providers["openrouter"])
	}
}

func TestFileRefusesCorruption(t *testing.T) {
	f := fileBackend{path: testStorePath(t)}
	if err := f.Save(newCredentials()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(); err == nil {
		t.Error("corrupt credentials file was accepted")
	}
}
