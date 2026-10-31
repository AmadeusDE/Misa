package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/cardui"
)

func cardKey(k rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: k, Text: string(k)} }
func TestCardGamesShellAndMenu(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	names := []string{"solitaire", "freecell", "spider", "hearts"}
	for _, mode := range []string{"multicall", "inprocess"} {
		for _, name := range names {
			m := New(t.TempDir())
			m.nestedMode = true
			m.width, m.height = 80, 24
			m.screen = screenShell
			m.builtinMode = mode
			handled, cmd := m.handleBuiltinCmd(name + " --seed 42")
			if !handled || cmd == nil || m.screen != screenProcMon {
				t.Fatal(mode, name, "not intercepted")
			}
			g := m.procViewer.(*cardui.Game)
			if !strings.Contains(g.View(), "v1:"+name+":42:") {
				t.Fatal("wrong game")
			}
			m.updateProcMon(cardKey('q'))
			m.updateProcMon(cardKey('y'))
			if m.screen != screenShell || m.procViewer != nil {
				t.Fatal("shell return")
			}
		}
	}
	items := cardGameMenuItems()
	for i, item := range items {
		m := New(t.TempDir())
		m.nestedMode = true
		m.width, m.height = 80, 24
		m.screen = screenMenu
		cmd := item.(menuItem).action(m)
		if cmd == nil || m.screen != screenProcMon || !strings.Contains(m.procViewer.View(), "v1:"+names[i]+":") {
			t.Fatal("menu", i)
		}
		m.updateProcMon(cardKey('q'))
		m.updateProcMon(cardKey('y'))
		if m.screen != screenMenu {
			t.Fatal("menu return")
		}
	}
}
func TestPaneCloseStopsCardGame(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.handleBuiltinCmd("hearts --seed 42")
	g := m.procViewer.(*cardui.Game)
	m.closeTerminals()
	if !g.IsQuit() {
		t.Fatal("pane close left game running")
	}
}
