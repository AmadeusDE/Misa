package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// F09: the embedded editor stays inside its pane (split and full width) when
// the text has tabs, wide characters, controls and very long lines.
func TestEditorDisplayCellsThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "wide")
	text := "\t日本語のテキスト\tend\né \x01 \xff " + strings.Repeat("世", 60) + "\n" + strings.Repeat("0123456789", 20) + "\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	send := func(msg tea.Msg) {
		t.Helper()
		_, cmd := m.Update(msg)
		if cmd != nil {
			if _, ok := cmd().(tea.QuitMsg); ok {
				t.Fatal("event quit Misa")
			}
		}
	}
	for _, size := range [][2]int{{80, 24}, {30, 10}, {9, 4}, {5, 3}, {3, 2}, {1, 1}, {60, 20}} {
		send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, k := range []string{"$", "j", "$", "j", "$", "0", "k"} {
			send(tea.KeyPressMsg{Code: rune(k[0]), Text: k})
			ed := pane.misa.viEditor
			if ed == nil {
				t.Fatal("editor closed")
			}
			for i, line := range strings.Split(ed.View(), "\n") {
				if w := ansi.StringWidth(line); w > size[0] {
					t.Fatalf("size %v key %q: editor line %d is %d cells wide", size, k, i, w)
				}
			}
			for i, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
				if size[0] < 5 || size[1] < 3 {
					break // below this the mux chrome itself (bar, borders) overflows
				}
				if w := ansi.StringWidth(line); w > size[0] {
					t.Fatalf("size %v key %q: screen line %d is %d cells wide: %q", size, k, i, w, line)
				}
			}
		}
	}
}
