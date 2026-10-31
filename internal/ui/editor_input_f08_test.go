package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Exercise public Bubble Tea events through the root mux, not Editor.Update.
func TestEditorInputThroughMux(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(send func(tea.Msg), text func(string))
		want string
	}{
		{"normal paste is text", func(send func(tea.Msg), text func(string)) {
			send(tea.PasteMsg{Content: "d:hello\nworld"})
		}, "d:hello\nworldoriginal\n"},
		{"insert paste and committed text", func(send func(tea.Msg), text func(string)) {
			text("i")
			send(tea.KeyPressMsg{Code: '界', Text: "世界"})
			send(tea.PasteMsg{Content: "a\nb"})
			send(tea.KeyPressMsg{Code: tea.KeyEscape})
		}, "世界a\nboriginal\n"},
		{"paste undo transaction", func(send func(tea.Msg), text func(string)) {
			text("iX")
			send(tea.PasteMsg{Content: "a\nb"})
			send(tea.KeyPressMsg{Code: tea.KeyEscape})
			text("u")
		}, "original\n"},
		{"paste redo transaction", func(send func(tea.Msg), text func(string)) {
			text("iX")
			send(tea.PasteMsg{Content: "a\nb"})
			send(tea.KeyPressMsg{Code: tea.KeyEscape})
			text("u")
			send(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
		}, "Xa\nboriginal\n"},
		{"command paste does not submit", func(send func(tea.Msg), text func(string)) {
			text(":")
			send(tea.PasteMsg{Content: "q!\n"})
			send(tea.KeyPressMsg{Code: tea.KeyEscape})
		}, "original\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
				t.Helper()
				m.Update(msg)
				if m.activePane() != pane || pane.misa.screen != screenEditor || pane.misa.viEditor == nil {
					t.Fatal("input left the editor pane")
				}
			}
			text := func(s string) {
				for _, r := range s {
					send(tea.KeyPressMsg{Code: r, Text: string(r)})
				}
			}
			tc.run(send, text)
			text(":w")
			send(tea.KeyPressMsg{Code: tea.KeyEnter})
			data, err := os.ReadFile(path)
			if err != nil || string(data) != tc.want {
				t.Fatalf("saved %q, want %q (err %v)", data, tc.want, err)
			}
		})
	}
}
