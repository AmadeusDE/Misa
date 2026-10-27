package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecurePrivateDirTightensExistingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := securePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(dir)
	if st.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o", st.Mode().Perm())
	}
}
