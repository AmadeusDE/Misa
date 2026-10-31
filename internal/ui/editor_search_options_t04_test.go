package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Search options must affect the editor reached through a mux pane, not just
// a directly instantiated editor.
func TestT04EditorSearchOptionsThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "sample")
	if err := os.WriteFile(path, []byte("start\nFOO\nFoo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	send := func(s string) {
		t.Helper()
		for _, r := range s {
			key := tea.KeyPressMsg{Code: r, Text: string(r)}
			if r == '\n' {
				key = tea.KeyPressMsg{Code: tea.KeyEnter}
			}
			if _, cmd := m.Update(key); cmd != nil {
				if _, quit := cmd().(tea.QuitMsg); quit {
					t.Fatal("event quit Misa")
				}
			}
		}
	}
	send(":set ignorecase\n/\\CFoo\nx:w\n")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "start\nFOO\noo\n" {
		t.Fatalf("case override edit saved %q", data)
	}
	send("gg/foo\nx:w\n")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "start\nOO\noo\n" {
		t.Fatalf("case-insensitive edit saved %q; view %q", data, ansi.Strip(pane.misa.viEditor.View()))
	}
	send(":q\n")
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Fatal("editor did not return to its pane")
	}
}
