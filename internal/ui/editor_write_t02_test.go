package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorT02RangeSaveAsThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	part := filepath.Join(dir, "part")
	saved := filepath.Join(dir, "saved")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	for _, line := range []string{":2,3w " + part, "iX", ":saveas " + saved, ":q"} {
		for _, r := range line {
			m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		var key tea.KeyPressMsg
		if line == "iX" {
			key = tea.KeyPressMsg{Code: tea.KeyEscape}
		} else {
			key = tea.KeyPressMsg{Code: tea.KeyEnter}
		}
		_, cmd := m.Update(key)
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Error("editor quit Misa")
			}
		}
	}
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Error("editor did not return to same pane")
	}
	for p, want := range map[string]string{path: "one\ntwo\nthree\n", part: "two\nthree\n", saved: "Xone\ntwo\nthree\n"} {
		b, err := os.ReadFile(p)
		if err != nil || string(b) != want {
			t.Errorf("%s=%q err=%v want=%q", p, b, err, want)
		}
	}
}
