package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigIgnoresWorkingDirectory(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg) // os.UserConfigDir on Windows
	cwd := t.TempDir()
	t.Chdir(cwd)
	for _, name := range []string{"config.yaml", "mux.yaml", "fetch.yaml", "gui.yaml"} {
		if err := os.WriteFile(filepath.Join(cwd, name), []byte("autorun: [evil]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findConfigFile(name); got != "" {
			t.Errorf("findConfigFile(%q) = %q, want none", name, got)
		}
	}
	if got := findFetchConfig(); got != "" {
		t.Errorf("findFetchConfig() = %q, want none", got)
	}
	if got := Load().Autorun; len(got) != 0 {
		t.Errorf("Load().Autorun = %v, want none", got)
	}

	user := filepath.Join(cfg, "misa", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("autorun: [hello]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findConfig(); got != user {
		t.Errorf("findConfig() = %q, want %q", got, user)
	}
}
