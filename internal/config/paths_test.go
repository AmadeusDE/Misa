package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsShareOneDirectory(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg)
	dir := filepath.Join(cfg, "misa")
	for name, got := range map[string]string{
		"config.yaml": DefaultPath(),
		"mux.yaml":    DefaultMuxPath(),
		"fetch.yaml":  DefaultFetchPath(),
	} {
		if want := filepath.Join(dir, name); got != want {
			t.Errorf("default path for %s = %q, want %q", name, got, want)
		}
	}
}
