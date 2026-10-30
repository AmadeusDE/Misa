package aiconfig

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ai"
)

func openDefaults(t *testing.T) *Model {
	t.Helper()
	m := newModel(t)
	addLocal(t, m, "ollama", server(t, "", "alpha"))
	press(m, "D")
	if m.mode != modeDefaults {
		t.Fatalf("mode %v", m.mode)
	}
	return m
}

func TestDefaultsPanelEditsModels(t *testing.T) {
	m := openDefaults(t)
	mustContain(t, m, "Default model", "Chat model", "Help model", "Search engine", "SearXNG URL", "duckduckgo", "(not set)")
	press(m, "enter")
	typeText(m, "ollama/alpha")
	press(m, "enter")
	if got := ughConfig(t).Models.Default; got != "ollama/alpha" {
		t.Fatalf("default %q status %q", got, m.status)
	}
	press(m, "down", "enter")
	typeText(m, "ollama/beta")
	press(m, "enter", "down", "enter")
	typeText(m, "ollama/gamma")
	press(m, "enter")
	s, _ := ai.LoadSettings()
	if s.Models["chat"] != "ollama/beta" || s.Models["help"] != "ollama/gamma" {
		t.Fatalf("%+v", s.Models)
	}
	press(m, "x")
	if s, _ = ai.LoadSettings(); s.Models["help"] != "" {
		t.Fatalf("x must clear: %+v", s.Models)
	}
	m.defs.list.cur = rowDefault
	press(m, "x")
	if ughConfig(t).Models.Default != "" {
		t.Fatal("default not cleared")
	}
}

func TestDefaultsRejectsBadModels(t *testing.T) {
	m := openDefaults(t)
	for _, bad := range []string{"alpha", "nowhere/alpha", "ollama/"} {
		press(m, "enter")
		m.defs.in.SetValue(bad)
		press(m, "enter")
		if !m.failed || !m.defs.editing {
			t.Fatalf("%q accepted: %q", bad, m.status)
		}
		press(m, "esc")
	}
	if ughConfig(t).Models.Default != "" {
		t.Fatal("a rejected value was saved")
	}
}

func TestSearchEngineCyclesAndNeedsAURLForSearxng(t *testing.T) {
	m := openDefaults(t)
	m.defs.list.cur = rowEngine
	press(m, "enter")
	if m.defs.list.cur != rowURL || !m.defs.editing {
		t.Fatalf("searxng without a URL must ask for one: row %d editing %v", m.defs.list.cur, m.defs.editing)
	}
	if s, _ := ai.LoadSettings(); s.Search.Engine != "duckduckgo" {
		t.Fatalf("engine changed before the URL was known: %q", s.Search.Engine)
	}
	typeText(m, "not a url")
	press(m, "enter")
	if !m.failed || !m.defs.editing {
		t.Fatalf("a bad URL must be refused: %q", m.status)
	}
	m.defs.in.SetValue("http://localhost:8888")
	press(m, "enter")
	s, _ := ai.LoadSettings()
	if s.Search.Engine != "searxng" || s.Search.SearxngURL != "http://localhost:8888" || m.defs.editing {
		t.Fatalf("%+v status %q", s.Search, m.status)
	}
	m.defs.list.cur = rowEngine
	press(m, "enter")
	if s, _ = ai.LoadSettings(); s.Search.Engine != "off" {
		t.Fatalf("%+v", s.Search)
	}
	press(m, "enter")
	if s, _ = ai.LoadSettings(); s.Search.Engine != "duckduckgo" || s.Search.SearxngURL == "" {
		t.Fatalf("the URL must survive cycling: %+v", s.Search)
	}
	mustContain(t, m, "duckduckgo", "http://localhost:8888")
}

func TestSearxngURLCannotBeClearedWhileInUse(t *testing.T) {
	m := openDefaults(t)
	if err := ai.SaveSettings(ai.Settings{Search: ai.Search{Engine: "searxng", SearxngURL: "http://s:1"}}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.defs.list.cur = rowURL
	press(m, "x")
	if !m.failed || !strings.Contains(m.status, "searxng") {
		t.Fatalf("status %q", m.status)
	}
}

func TestDefaultsEscAndCancel(t *testing.T) {
	m := openDefaults(t)
	press(m, "enter")
	typeText(m, "ollama/zzz")
	press(m, "esc")
	if m.mode != modeDefaults || m.defs.editing || ughConfig(t).Models.Default != "" {
		t.Fatal("esc in an edit must cancel it")
	}
	press(m, "esc")
	if m.mode != modeProviders {
		t.Fatalf("mode %v", m.mode)
	}
}

func TestHelpOpensAndAnyKeyCloses(t *testing.T) {
	m := newModel(t)
	press(m, "?")
	mustContain(t, m, "Providers: keys", "any key closes help")
	press(m, "x")
	if m.mode != modeProviders {
		t.Fatalf("mode %v", m.mode)
	}
}
