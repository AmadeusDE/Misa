package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorUndoSavedRevisionQuitThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "sample")
	if err := os.WriteFile(path, []byte("pear\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	for _, keys := range []string{"x:w", "xu:q"} {
		for _, r := range keys {
			m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Error("editor exit quit Misa")
			}
		}
	}
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Error("undo to saved state then :q did not return to the same pane/menu")
	}
	if !slices.Equal(pane.misa.viRegister, []string{"e"}) || pane.misa.viRegLine {
		t.Errorf("retained register=%q linewise=%v", pane.misa.viRegister, pane.misa.viRegLine)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "ear\n" {
		t.Errorf("saved=%q err=%v, want ear\\n", data, err)
	}
}
