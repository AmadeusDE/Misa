package aiconfig

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/ai"
	uc "github.com/AmadeusDE/Misa/internal/ugh/config"
)

func TestListShowsKindsAndKeyStatus(t *testing.T) {
	m := newModel(t)
	t.Setenv("OPENROUTER_API_KEY", secret)
	m.reload()
	mustContain(t, m, "AI providers", "openrouter", "opencode-go", "builtin", "env sk-", "missing", "codex", "no login")
	mustNotContain(t, m, secret)
}

func TestListMarksDefaultAndFeatureProviders(t *testing.T) {
	m := newModel(t)
	srv := server(t, "", "alpha")
	addLocal(t, m, "ollama", srv)
	if err := ai.SetDefaultModel("openrouter/some/model"); err != nil {
		t.Fatal(err)
	}
	if err := setFeature(ai.FeatureChat, "ollama/alpha"); err != nil {
		t.Fatal(err)
	}
	m.reload()
	mustContain(t, m, "chat ollama/alpha", "default openrouter/some/model")
	for name, want := range map[string]string{"openrouter": "default", "ollama": "chat"} {
		row := rowOf(m, name)
		if !strings.Contains(row, want) || (name == "ollama" && !strings.Contains(row, "no key needed")) {
			t.Errorf("row %q lacks %q", row, want)
		}
	}
}

func TestSetAndRemoveKeyMasked(t *testing.T) {
	m := newModel(t)
	selectProvider(t, m, "openrouter")
	press(m, "k")
	if m.mode != modeKey {
		t.Fatalf("mode = %v", m.mode)
	}
	run(m, tea.PasteMsg{Content: secret})
	view := plain(m)
	if strings.Contains(view, secret) || !strings.Contains(view, "••••") {
		t.Fatalf("the key field must be masked:\n%s", view)
	}
	press(m, "enter")
	if m.mode != modeProviders {
		t.Fatalf("mode = %v, status %q", m.mode, m.status)
	}
	info := providerNamed(t, "openrouter")
	if info.KeySource != "store" || info.KeyMasked == "" || strings.Contains(info.KeyMasked, secret) {
		t.Fatalf("key not stored: %+v", info)
	}
	mustContain(t, m, "key sk-")
	mustNotContain(t, m, secret)
	press(m, "K")
	if got := providerNamed(t, "openrouter"); got.KeySource != "" {
		t.Fatalf("key still there: %+v", got)
	}
	mustContain(t, m, "removed the stored key")
	press(m, "K")
	if !m.failed || !strings.Contains(m.status, "no stored key") {
		t.Fatalf("status %q", m.status)
	}
}

func TestKeyPromptRejectsEmptyAndCancels(t *testing.T) {
	m := newModel(t)
	selectProvider(t, m, "openrouter")
	press(m, "k", "enter")
	if m.mode != modeKey || !m.failed {
		t.Fatalf("an empty key must stay in the prompt: %v %q", m.mode, m.status)
	}
	typeText(m, "abc")
	press(m, "esc")
	if m.mode != modeProviders || m.key.in.Value() != "" {
		t.Fatalf("esc must leave and forget the text: %v %q", m.mode, m.key.in.Value())
	}
	if providerNamed(t, "openrouter").KeySource != "" {
		t.Fatal("cancelled prompt stored a key")
	}
}

func TestEnvKeyOutranksStoredKeyNote(t *testing.T) {
	m := newModel(t)
	t.Setenv("OPENROUTER_API_KEY", secret)
	m.reload()
	selectProvider(t, m, "openrouter")
	press(m, "K")
	if !m.failed || !strings.Contains(m.status, "OPENROUTER_API_KEY") {
		t.Fatalf("status %q", m.status)
	}
	press(m, "k")
	run(m, tea.PasteMsg{Content: "sk-another-key-value-9876543210"})
	press(m, "enter")
	if !strings.Contains(m.status, "takes precedence") {
		t.Fatalf("status %q", m.status)
	}
}

func TestLoginProvidersExplainTheLogin(t *testing.T) {
	m := newModel(t)
	selectProvider(t, m, "codex")
	for _, k := range []string{"enter", "k", "d"} {
		press(m, k)
		if !m.failed || m.mode != modeProviders || m.status == "" {
			t.Fatalf("key %s: mode %v status %q", k, m.mode, m.status)
		}
	}
	press(m, "enter")
	if !strings.Contains(m.status, "ugh login codex") {
		t.Fatalf("status %q", m.status)
	}
}

func TestDeleteAsksFirstAndBuiltinsStay(t *testing.T) {
	m := newModel(t)
	srv := server(t, "", "alpha")
	addLocal(t, m, "ollama", srv)
	selectProvider(t, m, "ollama")
	press(m, "d")
	if m.mode != modeConfirm {
		t.Fatalf("mode = %v", m.mode)
	}
	mustContain(t, m, "Delete provider ollama")
	press(m, "n")
	if _, ok := m.provider("ollama"); !ok || m.mode != modeProviders {
		t.Fatal("n must keep the provider")
	}
	press(m, "d", "y")
	if _, ok := m.provider("ollama"); ok {
		t.Fatal("provider not deleted")
	}
	if strings.Contains(configToml(t), "ollama") {
		t.Fatalf("still in config.toml:\n%s", configToml(t))
	}
	selectProvider(t, m, "openrouter")
	press(m, "d")
	if m.mode != modeProviders || !m.failed || !strings.Contains(m.status, "built in") {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
}

func TestTestRunsAsACommand(t *testing.T) {
	m := newModel(t)
	srv := server(t, "", "alpha")
	addLocal(t, m, "ollama", srv)
	selectProvider(t, m, "ollama")
	cmd := m.Update(keyMsg("t"))
	if cmd == nil || !strings.Contains(m.status, "testing ollama") {
		t.Fatalf("t must start a background test: cmd %v status %q", cmd, m.status)
	}
	if err := m.Update(cmd()); err != nil || m.failed || !strings.Contains(m.status, "ollama/alpha answered in") {
		t.Fatalf("status %q failed %v", m.status, m.failed)
	}
}

func TestTestFailureNeverShowsTheKey(t *testing.T) {
	m := newModel(t)
	srv := server(t, "bad key "+secret)
	if err := ai.SaveProvider("acme", uc.Provider{BaseURL: srv.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	if err := ai.SetKey("acme", secret); err != nil {
		t.Fatal(err)
	}
	m.reload()
	if err := ai.SetDefaultModel("acme/m"); err != nil {
		t.Fatal(err)
	}
	m.reload()
	selectProvider(t, m, "acme")
	press(m, "t")
	if !m.failed || !strings.Contains(m.status, "test failed") {
		t.Fatalf("status %q", m.status)
	}
	mustNotContain(t, m, secret)
}

func TestStaleTestResultIsDropped(t *testing.T) {
	m := newModel(t)
	run(m, testDoneMsg{gen: 99, name: "x", err: nil})
	if m.status != "" {
		t.Fatalf("status %q", m.status)
	}
}

func TestQuitKeys(t *testing.T) {
	m := newModel(t)
	press(m, "q")
	if !m.IsQuit() {
		t.Fatal("q must quit")
	}
	m = newModel(t)
	press(m, "a", "ctrl+c")
	if !m.IsQuit() {
		t.Fatal("ctrl+c must quit from any screen")
	}
}

func TestEveryScreenFitsTheTerminal(t *testing.T) {
	m := newModel(t)
	srv := server(t, "", "alpha", "beta")
	addLocal(t, m, "ollama", srv)
	screens := map[string]func(){
		"providers": func() {},
		"presets":   func() { press(m, "a") },
		"form":      func() { press(m, "a", "enter") },
		"key":       func() { selectProvider(t, m, "openrouter"); press(m, "k") },
		"confirm":   func() { selectProvider(t, m, "ollama"); press(m, "d") },
		"models":    func() { selectProvider(t, m, "ollama"); press(m, "m") },
		"defaults":  func() { press(m, "D") },
		"help":      func() { press(m, "?") },
	}
	for name, enter := range screens {
		for _, size := range [][2]int{{100, 30}, {60, 12}, {30, 6}} {
			m.mode = modeProviders
			m.Resize(size[0], size[1])
			enter()
			lines := strings.Split(m.View(), "\n")
			if len(lines) != max(size[1], 3) {
				t.Errorf("%s at %v: %d lines", name, size, len(lines))
			}
			for _, l := range lines {
				if w := len([]rune(ansi.Strip(l))); w > size[0] {
					t.Errorf("%s at %v: line of %d columns: %q", name, size, w, ansi.Strip(l))
				}
			}
		}
	}
}

func rowOf(m *Model, name string) string {
	for _, l := range strings.Split(plain(m), "\n") {
		if strings.Contains(l, name+" ") && strings.Contains(l, "http") {
			return l
		}
	}
	return ""
}
