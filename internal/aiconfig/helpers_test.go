package aiconfig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ai/aitest"
	uc "github.com/AmadeusDE/Misa/internal/ugh/config"
)

const secret = "sk-thisIsASecretKey1234567890abcd"

// newModel opens the configurator on an isolated ugh and Misa configuration.
func newModel(t *testing.T) *Model {
	t.Helper()
	aitest.Isolate(t)
	m := New()
	m.Resize(100, 30)
	return m
}

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

// press sends keys one by one, running any command they return to completion.
func press(m *Model, keys ...string) {
	for _, k := range keys {
		run(m, keyMsg(k))
	}
}

func typeText(m *Model, s string) {
	for _, r := range s {
		run(m, keyMsg(string(r)))
	}
}

// run feeds a message to the model and then every message its commands produce.
func run(m *Model, msg tea.Msg) {
	for cmd := m.Update(msg); cmd != nil; {
		next := cmd()
		if next == nil {
			return
		}
		cmd = m.Update(next)
	}
}

func selectProvider(t *testing.T, m *Model, name string) {
	t.Helper()
	m.mode = modeProviders
	m.list.cur = 0
	for i := 0; i < len(m.providers)+1 && m.selectedName() != name; i++ {
		press(m, "down")
	}
	if m.selectedName() != name {
		t.Fatalf("provider %s not in the list", name)
	}
}

func providerNamed(t *testing.T, name string) ai.ProviderInfo {
	t.Helper()
	all, err := ai.Providers()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no provider %s", name)
	return ai.ProviderInfo{}
}

func plain(m *Model) string { return ansi.Strip(m.View()) }

func mustContain(t *testing.T, m *Model, want ...string) {
	t.Helper()
	view := plain(m)
	for _, w := range want {
		if !strings.Contains(view, w) {
			t.Fatalf("view lacks %q:\n%s", w, view)
		}
	}
}

func mustNotContain(t *testing.T, m *Model, bad string) {
	t.Helper()
	if view := plain(m); strings.Contains(view, bad) || strings.Contains(m.status, bad) {
		t.Fatalf("view shows %q:\n%s", bad, view)
	}
}

func ughConfig(t *testing.T) uc.Config {
	t.Helper()
	cfg, err := uc.LoadIn(ai.UghRoots)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// server answers /models and /chat/completions like an OpenAI-compatible provider. A non-empty fail makes
// every request fail with that body.
func server(t *testing.T, fail string, models ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail != "" {
			http.Error(w, fail, http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/models"):
			var data []map[string]string
			for _, id := range models {
				data = append(data, map[string]string{"id": id})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": data})
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func addLocal(t *testing.T, m *Model, name string, srv *httptest.Server) {
	t.Helper()
	if err := ai.SaveProvider(name, uc.Provider{BaseURL: srv.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	m.reload()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func configToml(t *testing.T) string {
	t.Helper()
	path, err := uc.PathIn(ai.UghRoots)
	if err != nil {
		t.Fatal(err)
	}
	return readFile(t, filepath.Clean(path))
}
