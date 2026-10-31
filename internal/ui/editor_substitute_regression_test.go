package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorSubstituteConfirmationThroughMux(t *testing.T) {
	for _, cancel := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: tea.KeyEscape}} {
		t.Run(cancel.String(), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			m := newRoot(t, 80, 24)
			m.addMuxPane()
			pane := m.activePane()
			path := filepath.Join(t.TempDir(), "sample")
			if err := os.WriteFile(path, []byte("aa aa aa\n"), 0600); err != nil {
				t.Fatal(err)
			}
			pane.misa.screen = screenMenu
			pane.misa.enterEditor(path)
			send := func(k tea.KeyPressMsg) {
				t.Helper()
				_, cmd := m.Update(k)
				if cmd != nil {
					if _, ok := cmd().(tea.QuitMsg); ok {
						t.Fatal("event quit Misa")
					}
				}
			}
			text := func(s string) {
				for _, r := range s {
					send(tea.KeyPressMsg{Code: r, Text: string(r)})
				}
			}
			text("yl:%s/aa/x/gc")
			send(tea.KeyPressMsg{Code: tea.KeyEnter})
			m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
			text("yy")
			send(cancel)
			if pane.misa.viEditor == nil || m.activePane() != pane {
				t.Fatal("confirmation closed editor or pane")
			}
			if view := pane.misa.viEditor.View(); !strings.Contains(ansi.Strip(view), "x x aa") {
				t.Fatalf("confirmation result missing from view: %q", view)
			}
			text("u:q")
			send(tea.KeyPressMsg{Code: tea.KeyEnter})
			if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
				t.Fatal("undo and clean :q did not return to pane")
			}
			if !slices.Equal(pane.misa.viRegister, []string{"a"}) || pane.misa.viRegLine {
				t.Errorf("register=%q linewise=%v", pane.misa.viRegister, pane.misa.viRegLine)
			}
		})
	}
}
