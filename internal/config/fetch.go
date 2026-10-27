package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type FetchConfig struct {
	Info         string `yaml:"info,omitempty"`
	Emotes       bool   `yaml:"emotes,omitempty"`
	Theme        string `yaml:"theme,omitempty"`
	ColorMode    string `yaml:"color_mode,omitempty"`
	Ascii        string `yaml:"ascii,omitempty"`
	AsciiType    string `yaml:"ascii_type,omitempty"`
	AsciiDir     string `yaml:"ascii_dir,omitempty"`
	PalettesFile string `yaml:"palettes_file,omitempty"`
	MaxHeight    int    `yaml:"max_height,omitempty"`
}

func findFetchConfig() string { return findConfigFile("fetch.yaml") }

func DefaultFetchPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "fetch.yaml"
	}
	return filepath.Join(dir, "misa", "fetch.yaml")
}

func LoadFetch() FetchConfig {
	path := findFetchConfig()
	if path == "" {
		return FetchConfig{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FetchConfig{}
	}
	var cfg FetchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "misa: fetch config parse error in %s: %v\n", path, err)
		return FetchConfig{}
	}
	return cfg
}

func SaveFetch(cfg FetchConfig) error {
	path := findFetchConfig()
	if path == "" {
		path = DefaultFetchPath()
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
