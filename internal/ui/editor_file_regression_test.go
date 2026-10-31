package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorFileRoundTripThroughMux(t *testing.T) {
	for _, input := range []string{"first\r\nlast", ""} {
		t.Run(input, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			m := newRoot(t, 80, 24)
			m.addMuxPane()
			pane := m.activePane()
			path := filepath.Join(t.TempDir(), "roundtrip")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			pane.misa.screen = screenMenu
			pane.misa.enterEditor(path)
			for _, r := range ":wq" {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd != nil {
				if _, quit := cmd().(tea.QuitMsg); quit {
					t.Error("editor write/exit quit Misa")
				}
			}
			if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
				t.Error("write/exit did not return to the same pane/menu")
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != input {
				t.Errorf("saved=%q err=%v, want %q", data, err, input)
			}
		})
	}
}
