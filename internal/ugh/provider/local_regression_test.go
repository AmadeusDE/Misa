package provider

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/roots"
)

func localRoots(t *testing.T, providers string) roots.Roots {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("OPENCODE_API_KEY", "")
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[auth]\nstore = \"file\"\n"+providers), 0o600); err != nil {
		t.Fatal(err)
	}
	return roots.Roots{Config: dir, State: t.TempDir()}
}

func TestLocalProviderResolvesWithoutKey(t *testing.T) {
	r := localRoots(t, "[providers.ollama]\nbase_url = \"http://localhost:11434/v1\"\n")
	p, model, err := ResolveIn(r, "ollama/llama3")
	if err != nil || p == nil || model != "llama3" {
		t.Fatalf("local provider without key: %v %v %q", p, err, model)
	}
}

func TestRemoteProviderStillNeedsKey(t *testing.T) {
	r := localRoots(t, "[providers.acme]\nbase_url = \"https://api.acme.test/v1\"\n")
	if _, _, err := ResolveIn(r, "acme/m"); err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("want missing credential error, got %v", err)
	}
}

func TestModelsInSendsNoAuthorizationForLocal(t *testing.T) {
	var got string
	var seen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		got, seen = req.Header.Get("Authorization"), true
		w.Write([]byte(`{"data":[{"id":"llama3"}]}`))
	}))
	defer srv.Close()
	r := localRoots(t, "[providers.local]\nbase_url = \""+srv.URL+"\"\n")
	ms, err := ModelsIn(t.Context(), r, "local")
	if err != nil || len(ms) != 1 || ms[0].Ref() != "local/llama3" {
		t.Fatalf("models %v %v", ms, err)
	}
	if !seen || got != "" {
		t.Fatalf("Authorization = %q (seen %v)", got, seen)
	}
}
