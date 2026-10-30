package ai

import (
	"net/url"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
)

func TestPresetsAreValidProviders(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Presets() {
		if seen[p.Name] {
			t.Errorf("duplicate preset %s", p.Name)
		}
		seen[p.Name] = true
		if err := config.ValidateProvider(p.Name, config.Provider{BaseURL: p.BaseURL, KeyEnv: p.KeyEnv}); err != nil {
			t.Errorf("%s: %v", p.Name, err)
		}
		u, _ := url.Parse(p.BaseURL)
		if p.Local != isLoopbackURL(p.BaseURL) || (p.Local && (p.KeyEnv != "" || p.KeyURL != "")) || (!p.Local && (p.KeyEnv == "" || u.Scheme != "https" || p.KeyURL == "")) {
			t.Errorf("%s: local/key fields inconsistent: %+v", p.Name, p)
		}
	}
	for _, want := range []string{"openai", "gemini", "anthropic", "deepseek", "groq", "mistral", "xai", "together", "fireworks", "cerebras", "ollama", "lmstudio", "llamacpp"} {
		if !seen[want] {
			t.Errorf("preset %s missing", want)
		}
	}
}
