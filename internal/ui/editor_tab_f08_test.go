package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorTabCtrlIThroughMux(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyTab}, {Code: 'i', Mod: tea.ModCtrl}} {
		t.Run(key.String(), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			m := newRoot(t, 80, 24)
			m.addMuxPane()
			pane := m.activePane()
			path := filepath.Join(home, "sample")
			if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
				t.Fatal(err)
			}
			pane.misa.screen = screenMenu
			pane.misa.enterEditor(path)
			send := func(msg tea.Msg) {
				m.Update(msg)
				if m.activePane() != pane || pane.misa.screen != screenEditor {
					t.Fatal("left editor")
				}
			}
			text := func(s string) {
				for _, r := range s {
					send(tea.KeyPressMsg{Code: r, Text: string(r)})
				}
			}
			text("i")
			send(key)
			send(tea.KeyPressMsg{Code: tea.KeyEscape})
			text(":w")
			send(tea.KeyPressMsg{Code: tea.KeyEnter})
			b, err := os.ReadFile(path)
			if err != nil || string(b) != "\toriginal\n" {
				t.Fatalf("saved %q err %v", b, err)
			}
		})
	}
}
