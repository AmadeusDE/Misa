package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/policy"
)

func TestParseModelsContextAndSort(t *testing.T) {
	ms, err := parseModels("openrouter", strings.NewReader(
		`{"data":[{"id":"b","context_length":128000},{"id":"a","context_window":32000},{"id":""},{"id":"c"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 || ms[0].Ref() != "openrouter/a" || ms[0].Context != 32000 || ms[1].Context != 128000 || ms[2].Context != 0 {
		t.Fatalf("got %+v", ms)
	}
}

// TestScopedRoundTripAndContextCache exercises the scoped-models round trip
// and the model context cache. It pins the documented scope contract: an
// absent scope is the policy default state, never unrestricted, and its refs
// are the policy default root order for the current credentials — empty
// without any credential rather than a route that cannot work (see
// defaultModelRef in cmd/ugh). The auth backend is forced to the file store so
// an ambient OS keyring cannot leak real credentials into the assertions.
func TestScopedRoundTripAndContextCache(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("HOME", base)
	if err := os.MkdirAll(filepath.Join(base, "ugh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "ugh", "config.toml"), []byte("auth.store = \"file\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cacheData = nil

	// No credentials: an absent scope is policy defaults (state absent), not
	// the unrestricted "*", and the default root scope is empty.
	if res := Scope(); res.State != policy.ScopeStateAbsent {
		t.Fatalf("fresh scope state = %q, want %q (policy defaults, not unrestricted)", res.State, policy.ScopeStateAbsent)
	}
	if got := Scoped(); len(got) != 0 {
		t.Fatalf("credential-less scope = %v, want empty (no default rather than a route that cannot work)", got)
	}

	// With a credential the fresh scope is the policy default root order
	// (OpenRouter MiMo Pro first).
	t.Setenv("OPENROUTER_API_KEY", "sk-or-test-0000000000")
	got := Scoped()
	if len(got) == 0 || got[0] == "*" {
		t.Fatalf("fresh scope = %v, want the default policy scope", got)
	}
	if want := DefaultRootScope(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("fresh scope = %v, want the default policy scope %v", got, want)
	}
	if got[0] != "openrouter/xiaomi/mimo-v2.6-pro" {
		t.Fatalf("fresh scope starts with %q, want OpenRouter MiMo Pro", got[0])
	}

	if err := SaveScoped([]string{"openrouter/a", "codex/gpt-6-sol"}); err != nil {
		t.Fatal(err)
	}
	if got := Scoped(); len(got) != 2 || got[0] != "openrouter/a" {
		t.Fatalf("scope = %v", got)
	}
	if err := SaveScoped(nil); err != nil || len(Scoped()) != 0 {
		t.Fatalf("clear: %v %v", err, Scoped())
	}
	storeModels("openrouter", []Model{{Provider: "openrouter", ID: "a", Context: 1000}})
	if w, ok := ContextWindow("openrouter/a"); !ok || w != 1000 {
		t.Fatalf("ctx = %d %v", w, ok)
	}
	if _, ok := ContextWindow("opencode-go/a"); ok {
		t.Fatal("provider must match")
	}
	if w, ok := ContextWindow("a"); !ok || w != 1000 {
		t.Fatalf("bare ctx = %d %v", w, ok)
	}
}

func TestCompactModelPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if CompactModel() != "" {
		t.Fatal("unexpected default")
	}
	if err := SaveCompactModel("openrouter/x"); err != nil || CompactModel() != "openrouter/x" {
		t.Fatalf("save: %v %q", err, CompactModel())
	}
	if err := SaveCompactModel(""); err != nil || CompactModel() != "" {
		t.Fatalf("clear: %v %q", err, CompactModel())
	}
	if err := SaveCompactModel(""); err != nil {
		t.Fatalf("clearing twice: %v", err)
	}
}

func TestParseModelsReasoning(t *testing.T) {
	body := `{"data":[
		{"id":"z-ai/glm-5.3-flash","reasoning":{"mandatory":true,"supported_efforts":["max","high","low"]}},
		{"id":"xiaomi/mimo-v2.6-pro","reasoning":{"mandatory":false}},
		{"id":"plain"}]}`
	ms, err := parseModels("openrouter", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Model{}
	for _, m := range ms {
		got[m.ID] = m
	}
	if g := got["z-ai/glm-5.3-flash"]; !g.ThinkingRequired || strings.Join(g.Thinking, ",") != "max,high,low" {
		t.Errorf("glm = %+v", g)
	}
	if len(got["xiaomi/mimo-v2.6-pro"].Thinking) != 0 || len(got["plain"].Thinking) != 0 {
		t.Errorf("efforts invented: %+v", ms)
	}
	SetModelsForTest(t, "openrouter", ms)
	if e, req, ok := Thinking("openrouter/z-ai/glm-5.3-flash"); !ok || !req || len(e) != 3 {
		t.Errorf("Thinking = %v %v %v", e, req, ok)
	}
	if _, _, ok := Thinking("openrouter/xiaomi/mimo-v2.6-pro"); ok {
		t.Error("no efforts reported, but Thinking claimed some")
	}
}
