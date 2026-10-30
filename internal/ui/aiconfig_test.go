package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/procmon"
)

type fakeConfigurator struct{ quit bool }

func (f *fakeConfigurator) Init() tea.Cmd          { return nil }
func (f *fakeConfigurator) Update(tea.Msg) tea.Cmd { f.quit = true; return nil }
func (f *fakeConfigurator) View() string           { return "configurator" }
func (f *fakeConfigurator) Resize(w, h int)        {}
func (f *fakeConfigurator) IsQuit() bool           { return f.quit }

func withConfigurator(t *testing.T, make func() procmon.Viewer) {
	t.Helper()
	old := newAIConfig
	newAIConfig = make
	t.Cleanup(func() { newAIConfig = old })
}

func newAITestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	return m
}

func TestAIConfigCommandOpensTheConfigurator(t *testing.T) {
	withConfigurator(t, func() procmon.Viewer { return &fakeConfigurator{} })
	m := newAITestModel(t)
	handled, _ := m.handleBuiltinCmd("aiconfig")
	if !handled || m.screen != screenProcMon || m.procViewer == nil {
		t.Fatalf("handled %v screen %v", handled, m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.screen == screenProcMon || m.procViewer != nil {
		t.Fatal("quitting the configurator must return to the prompt")
	}
}

func TestAIConfigWithoutAISupportSaysSo(t *testing.T) {
	withConfigurator(t, nil)
	m := newAITestModel(t)
	handled, _ := m.handleBuiltinCmd("aiconfig")
	if !handled || m.screen == screenProcMon || !bufferHas(m, "built without AI support (-tags nough)") {
		t.Fatalf("handled %v screen %v buffer %q", handled, m.screen, m.shellBuffer)
	}
}

func TestMainMenuAndSettingsOfferAIProviders(t *testing.T) {
	withConfigurator(t, func() procmon.Viewer { return &fakeConfigurator{} })
	m := newAITestModel(t)
	found := false
	for _, it := range buildRootItems(t.TempDir(), nil) {
		if mi, ok := it.(menuItem); ok && mi.title == "AI providers" {
			found = true
			mi.action(m)
			if m.screen != screenProcMon {
				t.Fatalf("screen %v", m.screen)
			}
		}
	}
	if !found {
		t.Fatal("no AI providers entry in the main menu")
	}
	m = newAITestModel(t)
	m.enterSettings()
	for _, d := range settingDefs() {
		if d.key == "ai_providers" {
			d.onPick(m)
		}
	}
	if m.screen != screenProcMon || m.procMonReturn != screenSettings {
		t.Fatalf("screen %v return %v", m.screen, m.procMonReturn)
	}
	m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.screen != screenSettings || m.settings == nil {
		t.Fatalf("closing the configurator returns to Settings: screen %v", m.screen)
	}
}

func TestMenuEntryWithoutAISupportShowsAStatus(t *testing.T) {
	withConfigurator(t, nil)
	m := newAITestModel(t)
	m.enterAIConfig()
	if m.screen == screenProcMon || !strings.Contains(m.status, "nough") {
		t.Fatalf("screen %v status %q", m.screen, m.status)
	}
}
