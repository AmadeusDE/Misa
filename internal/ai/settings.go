package ai

import (
	"errors"
	"fmt"
	"os"

	"github.com/AmadeusDE/Misa/internal/config"
	uc "github.com/AmadeusDE/Misa/internal/ugh/config"
	"gopkg.in/yaml.v3"
)

const (
	FeatureChat = "chat"
	FeatureHelp = "help"
)

// Settings is Misa's ai.yaml (internal/config search order). Saved with SaveSettings.
type Settings struct {
	Models map[string]string `yaml:"models,omitempty"` // feature -> "provider/model"; "default" applies to every feature
	Search Search            `yaml:"search,omitempty"`
}

// Search configures the web search tool.
type Search struct {
	Engine     string `yaml:"engine,omitempty"` // "duckduckgo" (default, no key) | "searxng" | "off"
	SearxngURL string `yaml:"searxng_url,omitempty"`
}

// SettingsPath overrides where ai.yaml is read and written; empty means the Misa config search order.
var SettingsPath string

func settingsFile() string {
	switch {
	case SettingsPath != "":
		return SettingsPath
	case config.FindFile("ai.yaml") != "":
		return config.FindFile("ai.yaml")
	}
	return config.UserFile("ai.yaml")
}

// LoadSettings reads ai.yaml. A missing file gives the defaults.
func LoadSettings() (Settings, error) {
	var s Settings
	path := settingsFile()
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &s); err != nil {
			return Settings{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	if s.Search.Engine == "" {
		s.Search.Engine = "duckduckgo"
	}
	return s, nil
}

// SaveSettings writes ai.yaml.
func SaveSettings(s Settings) error {
	switch s.Search.Engine {
	case "", "duckduckgo", "off":
	case "searxng":
		if s.Search.SearxngURL == "" {
			return errors.New("search engine searxng needs searxng_url")
		}
	default:
		return fmt.Errorf("search engine must be duckduckgo, searxng or off, got %q", s.Search.Engine)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return writeFileAtomic(settingsFile(), data, 0o644, 0o755)
}

// ModelFor is the ref for a feature: Settings.Models[feature], else Models["default"], else ugh's
// models.default, else an error that tells the user to run the configurator.
func ModelFor(feature string) (string, error) {
	s, err := LoadSettings()
	if err != nil {
		return "", err
	}
	for _, key := range []string{feature, "default"} {
		if ref := s.Models[key]; ref != "" {
			return ref, nil
		}
	}
	cfg, err := uc.LoadIn(UghRoots)
	if err != nil {
		return "", err
	}
	if cfg.Models.Default != "" {
		return cfg.Models.Default, nil
	}
	return "", fmt.Errorf("no model chosen for %s: run `aiconfig` to pick one", feature)
}
