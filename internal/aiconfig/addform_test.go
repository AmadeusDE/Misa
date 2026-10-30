package aiconfig

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func pickPresetNamed(t *testing.T, m *Model, name string) {
	t.Helper()
	press(m, "a")
	for i, p := range m.pick.presets {
		if p.Name == name {
			m.pick.list.cur = i
			press(m, "enter")
			return
		}
	}
	t.Fatalf("no preset %s", name)
}

func TestAddFromPresetThenKey(t *testing.T) {
	m := newModel(t)
	pickPresetNamed(t, m, "openai")
	if m.mode != modeForm {
		t.Fatalf("mode = %v", m.mode)
	}
	mustContain(t, m, "Add provider", "openai", "https://api.openai.com/v1", "OPENAI_API_KEY", "platform.openai.com")
	press(m, "ctrl+s")
	if m.mode != modeKey {
		t.Fatalf("a hosted provider asks for its key next: mode %v status %q", m.mode, m.status)
	}
	run(m, tea.PasteMsg{Content: secret})
	mustNotContain(t, m, secret)
	press(m, "enter")
	info := providerNamed(t, "openai")
	if info.Kind != "custom" || info.KeySource != "store" || info.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("%+v", info)
	}
	if got := configToml(t); !strings.Contains(got, "[providers.openai]") || strings.Contains(got, secret) {
		t.Fatalf("config.toml:\n%s", got)
	}
	if m.selectedName() != "openai" || m.mode != modeProviders {
		t.Fatalf("selected %q mode %v", m.selectedName(), m.mode)
	}
}

func TestAddLocalPresetSkipsTheKeyPrompt(t *testing.T) {
	m := newModel(t)
	pickPresetNamed(t, m, "ollama")
	press(m, "ctrl+s")
	if m.mode != modeProviders || m.failed {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	if info := providerNamed(t, "ollama"); !info.Local || info.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("%+v", info)
	}
}

func TestAddSkipsTheKeyPromptWhenTheEnvironmentHasOne(t *testing.T) {
	m := newModel(t)
	t.Setenv("OPENAI_API_KEY", secret)
	pickPresetNamed(t, m, "openai")
	press(m, "ctrl+s")
	if m.mode != modeProviders {
		t.Fatalf("mode %v", m.mode)
	}
}

func TestEscInTheKeyPromptKeepsTheProvider(t *testing.T) {
	m := newModel(t)
	pickPresetNamed(t, m, "groq")
	press(m, "ctrl+s", "esc")
	if m.mode != modeProviders || !strings.Contains(m.status, "without a key") {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	if _, ok := m.provider("groq"); !ok {
		t.Fatal("provider lost")
	}
}

func TestCustomFormWithHeadersAndNoThinking(t *testing.T) {
	m := newModel(t)
	press(m, "a")
	m.pick.list.cur = len(m.pick.presets)
	press(m, "enter")
	typeText(m, "Acme")
	press(m, "enter")
	typeText(m, "https://api.acme.example/v1")
	press(m, "enter")
	typeText(m, "ACME_KEY")
	press(m, "tab")
	typeText(m, "X-Team: red; X-Org: 7")
	press(m, "tab")
	typeText(m, "acme-small, acme-tiny")
	press(m, "enter")
	if m.mode != modeKey {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	press(m, "esc")
	got := providerNamed(t, "acme").Custom
	if got.KeyEnv != "ACME_KEY" || got.Headers["X-Team"] != "red" || got.Headers["X-Org"] != "7" ||
		strings.Join(got.NoThinking, "|") != "acme-small|acme-tiny" {
		t.Fatalf("%+v", got)
	}
}

func TestFormErrorsStayInTheForm(t *testing.T) {
	m := newModel(t)
	press(m, "a")
	m.pick.list.cur = len(m.pick.presets)
	press(m, "enter")
	typeText(m, "bad name")
	press(m, "tab")
	typeText(m, "https://x.example/v1")
	press(m, "ctrl+s")
	if m.mode != modeForm || !m.failed || !strings.Contains(m.status, "invalid name") {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	mustContain(t, m, "invalid name")
	m.form.fields[fieldName].in.SetValue("ok")
	m.form.fields[fieldHeaders].in.SetValue("nonsense")
	press(m, "ctrl+s")
	if m.mode != modeForm || !strings.Contains(m.status, "Name: value") {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	m.form.fields[fieldHeaders].in.SetValue("")
	m.form.fields[fieldURL].in.SetValue("http://remote.example/v1")
	press(m, "ctrl+s")
	if m.mode != modeForm || !strings.Contains(m.status, "https") {
		t.Fatalf("plain http to a remote host must be refused: %q", m.status)
	}
}

func TestAddRefusesAnExistingName(t *testing.T) {
	m := newModel(t)
	pickPresetNamed(t, m, "ollama")
	press(m, "ctrl+s")
	pickPresetNamed(t, m, "ollama")
	mustContain(t, m, "Add provider")
	press(m, "ctrl+s")
	if m.mode != modeForm || !strings.Contains(m.status, "already exists") {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
}

func TestEditCustomProviderKeepsItsName(t *testing.T) {
	m := newModel(t)
	pickPresetNamed(t, m, "ollama")
	press(m, "ctrl+s")
	selectProvider(t, m, "ollama")
	press(m, "enter")
	if m.mode != modeForm || !m.form.editing {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	mustContain(t, m, "Edit provider ollama", "(fixed)")
	m.form.fields[fieldURL].in.SetValue("http://localhost:9999/v1")
	press(m, "ctrl+s")
	if m.mode != modeProviders || providerNamed(t, "ollama").BaseURL != "http://localhost:9999/v1" {
		t.Fatalf("mode %v status %q", m.mode, m.status)
	}
	press(m, "enter", "esc")
	if m.mode != modeProviders {
		t.Fatalf("esc from an edit returns to the list: %v", m.mode)
	}
}

func TestFormTabSkipsLockedNameAndWraps(t *testing.T) {
	m := newModel(t)
	pickPresetNamed(t, m, "ollama")
	press(m, "ctrl+s")
	selectProvider(t, m, "ollama")
	press(m, "enter")
	if m.form.cur != fieldURL {
		t.Fatalf("an edit starts on the URL: %d", m.form.cur)
	}
	press(m, "shift+tab")
	if m.form.cur != fieldURL {
		t.Fatalf("the fixed name must be skipped: %d", m.form.cur)
	}
	press(m, "tab", "tab", "tab", "tab", "tab")
	if m.form.cur != fieldNoThinking {
		t.Fatalf("cur %d", m.form.cur)
	}
}

func TestParseHeaders(t *testing.T) {
	got, err := parseHeaders(" A: 1 ;B:two:three;; ")
	if err != nil || got["A"] != "1" || got["B"] != "two:three" || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := parseHeaders(": x"); err == nil {
		t.Fatal("a header needs a name")
	}
	if formatHeaders(map[string]string{"B": "2", "A": "1"}) != "A: 1; B: 2" {
		t.Fatal("headers must format in a stable order")
	}
}
