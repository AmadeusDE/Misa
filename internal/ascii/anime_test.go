package ascii

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func withAnimeDir(t *testing.T, files map[string]string) {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg) // os.UserConfigDir on Windows
	if files == nil {
		return
	}
	dir := filepath.Join(cfg, "misa", "ascii", "anime")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnimeArtIsEmptyWithoutUserArt(t *testing.T) {
	withAnimeDir(t, nil)
	if names := ListAnimeNames(); len(names) != 0 {
		t.Errorf("ListAnimeNames() = %v, want none", names)
	}
	for _, sel := range []string{"", "random", "missing"} {
		if lines := LoadAnimeArt(sel); lines != nil {
			t.Errorf("LoadAnimeArt(%q) = %q, want nil", sel, lines)
		}
	}
}

func TestAnimeArtComesFromConfigDir(t *testing.T) {
	withAnimeDir(t, map[string]string{"cat.ascii": "/\\_/\\\n( o.o )\n", "credits.txt": "not art"})
	if names := ListAnimeNames(); !slices.Equal(names, []string{"cat"}) {
		t.Errorf("ListAnimeNames() = %v, want [cat]", names)
	}
	want := []string{"/\\_/\\", "( o.o )"}
	for _, sel := range []string{"cat", "random", ""} {
		if got := LoadAnimeArt(sel); !slices.Equal(got, want) {
			t.Errorf("LoadAnimeArt(%q) = %q, want %q", sel, got, want)
		}
	}
}
