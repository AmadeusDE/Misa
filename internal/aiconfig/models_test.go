package aiconfig

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ai"
)

func openModelsOf(t *testing.T, m *Model, name string) {
	t.Helper()
	selectProvider(t, m, name)
	cmd := m.Update(keyMsg("m"))
	if cmd == nil || m.mode != modeModels || !m.models.loading {
		t.Fatalf("m must start loading in the background: cmd %v mode %v", cmd, m.mode)
	}
	mustContain(t, m, "loading models")
	if next := m.Update(cmd()); next != nil {
		t.Fatal("unexpected follow-up command")
	}
}

func TestModelsAreFetchedFilteredAndPicked(t *testing.T) {
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "zeta", "alpha", "alpine"))
	openModelsOf(t, m, "ollama")
	mustContain(t, m, "Models of ollama", "3 models", "alpha", "alpine", "zeta")
	press(m, "/")
	typeText(m, "alp")
	mustContain(t, m, "2 of 3")
	mustNotContain(t, m, "zeta")
	press(m, "enter", "down", "enter")
	if got := ughConfig(t).Models.Default; got != "ollama/alpine" {
		t.Fatalf("default %q status %q", got, m.status)
	}
	press(m, "up", "c")
	press(m, "down", "h")
	s, err := ai.LoadSettings()
	if err != nil || s.Models["chat"] != "ollama/alpha" || s.Models["help"] != "ollama/alpine" {
		t.Fatalf("%+v %v", s, err)
	}
	mustContain(t, m, "help model set to ollama/alpine", "default", "chat", "help")
	if ref, _ := ai.ModelFor(ai.FeatureChat); ref != "ollama/alpha" {
		t.Fatalf("chat model %q", ref)
	}
	press(m, "esc")
	if m.mode != modeModels || m.models.filter.Value() != "" {
		t.Fatal("the first esc clears the filter, not the screen")
	}
}

func TestModelsFilterEscClears(t *testing.T) {
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "alpha", "zeta"))
	openModelsOf(t, m, "ollama")
	press(m, "/")
	typeText(m, "zz")
	mustContain(t, m, "no models match")
	press(m, "esc")
	mustContain(t, m, "2 models")
	press(m, "esc")
	if m.mode != modeProviders {
		t.Fatalf("mode %v", m.mode)
	}
}

func TestModelsErrorIsShownInline(t *testing.T) {
	m := newModel(t)
	srv := server(t, "denied for "+secret)
	if err := ai.SetKey("openrouter", secret); err != nil {
		t.Fatal(err)
	}
	addLocal(t, m, "ollama", srv)
	openModelsOf(t, m, "ollama")
	if m.models.err == "" {
		t.Fatal("no error shown")
	}
	mustNotContain(t, m, secret)
	press(m, "enter")
	if ughConfig(t).Models.Default != "" {
		t.Fatal("enter on an empty list must change nothing")
	}
}

func TestStaleModelsAreDropped(t *testing.T) {
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "alpha"))
	selectProvider(t, m, "ollama")
	first := m.Update(keyMsg("m"))
	press(m, "esc")
	m.Update(keyMsg("m"))
	m.Update(first())
	if !m.models.loading {
		t.Fatal("an older reply must not finish a newer request")
	}
}

func TestModelsRefreshKeepsFilter(t *testing.T) {
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "alpha", "beta"))
	openModelsOf(t, m, "ollama")
	press(m, "/")
	typeText(m, "be")
	press(m, "enter")
	run(m, keyMsg("r"))
	mustContain(t, m, "1 of 2", "beta")
	if !strings.Contains(plain(m), "filter: be") {
		t.Fatalf("filter lost:\n%s", plain(m))
	}
}
