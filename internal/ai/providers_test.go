package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ai/aitest"
	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

func readUghConfig(t *testing.T, cfgHome string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(cfgHome, "ugh", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func find(t *testing.T, name string) ProviderInfo {
	t.Helper()
	ps, err := Providers()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("%s not listed in %+v", name, ps)
	return ProviderInfo{}
}

func TestProvidersListsBuiltinsAndCodex(t *testing.T) {
	aitest.Isolate(t)
	ps, err := Providers()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range ps {
		names = append(names, p.Name+":"+p.Kind)
	}
	want := "opencode-go:builtin opencode-zen:builtin openrouter:builtin codex:codex"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("got %s", got)
	}
	if p := find(t, "openrouter"); p.KeySource != "" || p.KeyEnv != "OPENROUTER_API_KEY" {
		t.Fatalf("openrouter: %+v", p)
	}
}

func TestSaveProviderKeepsTheRestOfTheFile(t *testing.T) {
	cfg := aitest.Isolate(t)
	dir := filepath.Join(cfg, "ugh")
	os.MkdirAll(dir, 0o700)
	hand := "# my config\n[ui]\ntheme = \"light\" # bright\n\n[models]\ndefault = \"openrouter/x\"\n"
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(hand), 0o600)
	if err := SaveProvider("acme", acme); err != nil {
		t.Fatal(err)
	}
	got := readUghConfig(t, cfg)
	if !strings.HasPrefix(got, hand) || !strings.HasSuffix(got, acmeText) {
		t.Fatalf("got:\n%s", got)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
	p := find(t, "acme")
	if p.Kind != "custom" || p.BaseURL != acme.BaseURL || p.KeyEnv != "ACME_KEY" || p.Custom.NoThinking[1] != "x" {
		t.Fatalf("acme: %+v", p)
	}
	acme2 := acme
	acme2.BaseURL = "https://other.test/v1"
	acme2.Headers = nil
	if err := SaveProvider("acme", acme2); err != nil {
		t.Fatal(err)
	}
	if got := readUghConfig(t, cfg); strings.Count(got, "[providers.acme]") != 1 || strings.Contains(got, "headers") ||
		!strings.Contains(got, "other.test") || !strings.HasPrefix(got, hand) {
		t.Fatalf("got:\n%s", got)
	}
}

func TestSaveProviderCreatesMissingConfigPrivately(t *testing.T) {
	cfg := aitest.Isolate(t)
	if err := SaveProvider("ollama", config.Provider{BaseURL: "http://localhost:11434/v1"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(cfg, "ugh"))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("dir: %v %v", info, err)
	}
	if !find(t, "ollama").Local {
		t.Fatal("ollama should be local")
	}
}

func TestSaveProviderValidates(t *testing.T) {
	cfg := aitest.Isolate(t)
	for name, p := range map[string]config.Provider{
		"codex":  {BaseURL: "https://a.test/v1"},
		"Bad":    {BaseURL: "https://a.test/v1"},
		"ok":     {BaseURL: "http://remote.test/v1"},
		"envbad": {BaseURL: "https://a.test/v1", KeyEnv: "lower"},
	} {
		if err := SaveProvider(name, p); err == nil {
			t.Errorf("%s saved", name)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg, "ugh", "config.toml")); err == nil {
		t.Fatal("rejected providers wrote a file")
	}
}

func TestSaveProviderRefusesFormsItCannotEdit(t *testing.T) {
	cfg := aitest.Isolate(t)
	dir := filepath.Join(cfg, "ugh")
	os.MkdirAll(dir, 0o700)
	hand := "[providers]\nacme = { base_url = \"https://inline.test/v1\" }\n"
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte(hand), 0o600)
	if err := SaveProvider("acme", acme); err == nil {
		t.Fatal("inline table redefinition should be refused")
	}
	if got := readUghConfig(t, cfg); got != hand {
		t.Fatalf("file changed:\n%s", got)
	}
}

func TestRemoveProvider(t *testing.T) {
	cfg := aitest.Isolate(t)
	SaveProvider("acme", acme)
	SaveProvider("zed", config.Provider{BaseURL: "https://zed.test/v1"})
	if err := SetKey("acme", "sk-acme-0123456789abcd"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveProvider("acme"); err != nil {
		t.Fatal(err)
	}
	got := readUghConfig(t, cfg)
	if strings.Contains(got, "acme") || !strings.Contains(got, "[providers.zed]") {
		t.Fatalf("got:\n%s", got)
	}
	if err := RemoveKey("acme"); err == nil {
		t.Fatal("key should have gone with the provider")
	}
	if err := RemoveProvider("openrouter"); err == nil || !strings.Contains(err.Error(), "built in") {
		t.Fatalf("built-in: %v", err)
	}
	if err := RemoveProvider("nope"); err == nil {
		t.Fatal("unknown provider removed")
	}
}

func TestKeysAreMaskedAndNeverInErrors(t *testing.T) {
	aitest.Isolate(t)
	SaveProvider("acme", acme)
	const key = "sk-acme-0123456789abcd"
	if err := SetKey("acme", "  "+key+"\n"); err != nil {
		t.Fatal(err)
	}
	p := find(t, "acme")
	if p.KeySource != "store" || p.KeyMasked != "sk-acm…abcd" || strings.Contains(p.KeyMasked, "0123456789") {
		t.Fatalf("store: %+v", p)
	}
	t.Setenv("ACME_KEY", "env-secret-0123456789xyz")
	if p := find(t, "acme"); p.KeySource != "env" || p.KeyMasked != "env-se…9xyz" {
		t.Fatalf("env wins: %+v", p)
	}
	t.Setenv("ACME_KEY", "")
	if err := RemoveKey("acme"); err != nil {
		t.Fatal(err)
	}
	if p := find(t, "acme"); p.KeySource != "" || p.KeyMasked != "" {
		t.Fatalf("removed: %+v", p)
	}
	for _, bad := range []struct{ prov, key string }{{"codex", key}, {"unknown", key}, {"acme", ""}, {"acme", "a\x01b"}} {
		err := SetKey(bad.prov, bad.key)
		if err == nil || (bad.key != "" && strings.Contains(err.Error(), bad.key)) {
			t.Errorf("SetKey(%q): %v", bad.prov, err)
		}
	}
}

func TestSetDefaultModel(t *testing.T) {
	cfg := aitest.Isolate(t)
	if err := SetDefaultModel("nope"); err == nil {
		t.Fatal("ref without provider accepted")
	}
	if err := SetDefaultModel("acme/m1"); err != nil {
		t.Fatal(err)
	}
	if err := SetDefaultModel("acme/m2"); err != nil {
		t.Fatal(err)
	}
	if got := readUghConfig(t, cfg); got != "[models]\ndefault = \"acme/m2\"\n" {
		t.Fatalf("got:\n%s", got)
	}
	if err := SetDefaultModel(""); err != nil {
		t.Fatal(err)
	}
	if got, err := ModelFor(FeatureChat); err == nil {
		t.Fatalf("cleared default still resolves: %q", got)
	}
}

func localServer(t *testing.T, models ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			var data []map[string]string
			for _, m := range models {
				data = append(data, map[string]string{"id": m})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": data})
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLocalPresetEndToEnd(t *testing.T) {
	aitest.Isolate(t)
	srv := localServer(t, "zeta", "alpha")
	if err := SaveProvider("ollama", config.Provider{BaseURL: srv.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	ids, err := Models(context.Background(), "ollama")
	if err != nil || strings.Join(ids, ",") != "alpha,zeta" {
		t.Fatalf("models %v %v", ids, err)
	}
	c, err := Open("ollama/alpha")
	if err != nil {
		t.Fatalf("a local provider without a key must open: %v", err)
	}
	text, used, err := c.Complete(context.Background(), "be brief", []llm.Message{{Role: llm.RoleUser, Text: "hi"}})
	if err != nil || text != "ok" || used.In != 3 || used.Out != 1 {
		t.Fatalf("complete: %q %+v %v", text, used, err)
	}
	if d, err := Test(context.Background(), "ollama/alpha"); err != nil || d <= 0 {
		t.Fatalf("test: %v %v", d, err)
	}
}

func TestOpenRemoteWithoutKeyFails(t *testing.T) {
	aitest.Isolate(t)
	SaveProvider("acme", acme)
	if _, err := Open("acme/m"); err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("err = %v", err)
	}
	if _, err := OpenFor(FeatureChat); err == nil || !strings.Contains(err.Error(), "aiconfig") {
		t.Fatalf("OpenFor err = %v", err)
	}
}

func TestModelsCodexAndUnknown(t *testing.T) {
	aitest.Isolate(t)
	if ids, err := Models(context.Background(), "codex"); err != nil || len(ids) == 0 {
		t.Fatalf("codex: %v %v", ids, err)
	}
	if _, err := Models(context.Background(), "nope"); err == nil {
		t.Fatal("unknown provider listed")
	}
}
