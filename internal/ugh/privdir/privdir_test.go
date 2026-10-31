//go:build unix

package privdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesAPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ugh-1")
	if err := Ensure(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm() != 0o700 {
		t.Fatalf("dir = %v, %v", fi.Mode(), err)
	}
	if err := Ensure(dir); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
}

func TestEnsureRefusesDirectoriesOthersControl(t *testing.T) {
	base := t.TempDir()
	open := filepath.Join(base, "open")
	os.Mkdir(open, 0o777)
	os.Chmod(open, 0o777)
	link := filepath.Join(base, "link")
	os.Symlink(t.TempDir(), link)
	file := filepath.Join(base, "file")
	os.WriteFile(file, nil, 0o600)
	for _, p := range []string{open, link, file} {
		if err := Ensure(p); err == nil {
			t.Errorf("%s accepted", filepath.Base(p))
		}
	}
}

func TestUnderFallsBackAndReportsFailure(t *testing.T) {
	base := t.TempDir()
	os.Mkdir(filepath.Join(base, "ugh-x"), 0o777)
	os.Chmod(filepath.Join(base, "ugh-x"), 0o777)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir, err := Under(base, "ugh-x")
	if err != nil || filepath.Dir(dir) != cache {
		t.Fatalf("Under = %q, %v; want a directory in the cache", dir, err)
	}

	blocked := filepath.Join(t.TempDir(), "cache")
	os.WriteFile(blocked, nil, 0o600)
	t.Setenv("XDG_CACHE_HOME", blocked)
	if dir, err := Under(base, "ugh-x"); err == nil || dir != "" {
		t.Fatalf("Under = %q, %v; want an error and no path", dir, err)
	}
}
