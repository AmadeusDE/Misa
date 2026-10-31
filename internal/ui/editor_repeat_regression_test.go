package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorRepeatThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "sample")
	if err := os.WriteFile(path, []byte("one two three\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	send := func(key tea.KeyPressMsg) {
		t.Helper()
		_, cmd := m.Update(key)
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Error("editor event quit Misa")
			}
		}
	}
	text := func(s string) {
		for _, r := range s {
			send(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
	text("cwX")
	send(tea.KeyPressMsg{Code: tea.KeyEscape})
	text("w.u")
	// A single undo must remove the whole repeated change, then redo reapplies it.
	text(":w")
	send(tea.KeyPressMsg{Code: tea.KeyEnter})
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "X two three\n" {
		t.Fatalf("undo dot saved=%q err=%v", data, err)
	}
	send(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	text(":wq")
	send(tea.KeyPressMsg{Code: tea.KeyEnter})
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Error("editor exit did not return to same pane/menu")
	}
	if !slices.Equal(pane.misa.viRegister, []string{"two"}) || pane.misa.viRegLine {
		t.Errorf("retained register=%q linewise=%v", pane.misa.viRegister, pane.misa.viRegLine)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "X X three\n" {
		t.Errorf("saved=%q err=%v", data, err)
	}
}
