package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorEditEntryThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	first, second := filepath.Join(home, "first"), filepath.Join(home, "second")
	for path, data := range map[string]string{first: "alpha\n", second: "beta\n"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	pane.misa.dir = home
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(first)
	send := func(s string) {
		t.Helper()
		for _, r := range s {
			m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
	enter := func() {
		t.Helper()
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Fatal("editor event quit Misa")
			}
		}
	}
	send("iX")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	send(":e second")
	enter()
	if view := ansi.Strip(pane.misa.viEditor.View()); !strings.Contains(view, "E37") || !strings.Contains(view, "Xalpha") {
		t.Errorf("guard view: %q", view)
	}
	send(":edit! second")
	enter()
	if view := ansi.Strip(pane.misa.viEditor.View()); !strings.Contains(view, "beta") || strings.Contains(view, "Xalpha") {
		t.Fatalf("switch view: %q", view)
	}
	send("u:q")
	enter()
	if m.activePane() != pane || pane.misa.viEditor != nil || pane.misa.screen != screenMenu {
		t.Fatal("clean :q did not return to same mux pane")
	}
	for path, want := range map[string]string{first: "alpha\n", second: "beta\n"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Errorf("%s: %q, %v", path, data, err)
		}
	}
}
