package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ai/aitest"
)

func TestSettingsRoundTrip(t *testing.T) {
	cfg := aitest.Isolate(t)
	s, err := LoadSettings()
	if err != nil || s.Search.Engine != "duckduckgo" || len(s.Models) != 0 {
		t.Fatalf("defaults: %+v %v", s, err)
	}
	s.Models = map[string]string{FeatureChat: "ollama/llama3"}
	s.Search = Search{Engine: "searxng", SearxngURL: "http://localhost:8888"}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg, "misa", "ai.yaml")); err != nil {
		t.Fatalf("ai.yaml not in the misa config dir: %v", err)
	}
	got, err := LoadSettings()
	if err != nil || got.Models[FeatureChat] != "ollama/llama3" || got.Search != s.Search {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

func TestSettingsPathOverride(t *testing.T) {
	aitest.Isolate(t)
	SettingsPath = filepath.Join(t.TempDir(), "sub", "ai.yaml")
	t.Cleanup(func() { SettingsPath = "" })
	if err := SaveSettings(Settings{Models: map[string]string{"help": "a/b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SettingsPath); err != nil {
		t.Fatal(err)
	}
}

func TestSaveSettingsValidates(t *testing.T) {
	aitest.Isolate(t)
	for _, s := range []Search{{Engine: "bing"}, {Engine: "searxng"}} {
		if err := SaveSettings(Settings{Search: s}); err == nil {
			t.Errorf("%+v saved", s)
		}
	}
}

func TestLoadSettingsReportsBadYAML(t *testing.T) {
	aitest.Isolate(t)
	SettingsPath = filepath.Join(t.TempDir(), "ai.yaml")
	t.Cleanup(func() { SettingsPath = "" })
	os.WriteFile(SettingsPath, []byte("models: [unclosed"), 0o644)
	if _, err := LoadSettings(); err == nil || !strings.Contains(err.Error(), "ai.yaml") {
		t.Fatalf("err = %v", err)
	}
}

func TestModelForOrder(t *testing.T) {
	cfg := aitest.Isolate(t)
	if _, err := ModelFor(FeatureChat); err == nil || !strings.Contains(err.Error(), "aiconfig") {
		t.Fatalf("no model should point at the configurator, got %v", err)
	}
	if err := SetDefaultModel("ugh/default"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ModelFor(FeatureChat); got != "ugh/default" {
		t.Fatalf("ugh default: %q", got)
	}
	SaveSettings(Settings{Models: map[string]string{"default": "misa/all", FeatureHelp: "misa/help"}})
	if got, _ := ModelFor(FeatureChat); got != "misa/all" {
		t.Fatalf("misa default: %q", got)
	}
	if got, _ := ModelFor(FeatureHelp); got != "misa/help" {
		t.Fatalf("feature: %q", got)
	}
	if _, err := os.Stat(filepath.Join(cfg, "ugh", "config.toml")); err != nil {
		t.Fatal(err)
	}
}
