package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type MuxConfig struct {
	AutoPane    *bool  `yaml:"auto_pane,omitempty"`
	BarAutoHide *bool  `yaml:"bar_auto_hide,omitempty"`
	BarPosition string `yaml:"bar_position,omitempty"`
	BarInfo     string `yaml:"bar_info,omitempty"`
	// FocusFollowsMouse focuses the pane under the pointer (default on).
	FocusFollowsMouse *bool `yaml:"focus_follows_mouse,omitempty"`
}

func findMuxConfig() string { return findConfigFile("mux.yaml") }

// findConfigFile looks for name next to the executable, then in
// ~/.config/misa. The working directory is never searched, so a cloned
// repository cannot supply config (autorun, aliases) by being entered.
func findConfigFile(name string) string {
	if execPath, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(execPath), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if dir, err := os.UserConfigDir(); err == nil {
		p := filepath.Join(dir, "misa", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func DefaultMuxPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "mux.yaml"
	}
	return filepath.Join(dir, "misa", "mux.yaml")
}

func LoadMux() MuxConfig {
	path := findMuxConfig()
	if path == "" {
		return MuxConfig{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MuxConfig{}
	}
	var cfg MuxConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "misa: mux config parse error in %s: %v\n", path, err)
		return MuxConfig{}
	}
	return cfg
}

func SaveMux(cfg MuxConfig) error {
	path := findMuxConfig()
	if path == "" {
		path = DefaultMuxPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
