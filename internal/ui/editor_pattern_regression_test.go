package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorVimPatternsThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "sample")
	if err := os.WriteFile(path, []byte("cat 12 ünï 345\nfoo\n"), 0600); err != nil {
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
	enter := func() { send(tea.KeyPressMsg{Code: tea.KeyEnter}) }
	view := func() string { return ansi.Strip(pane.misa.viEditor.View()) }

	text(`:s/\d\+/N/g`)
	enter()
	if v := view(); !strings.Contains(v, "cat N ünï N") {
		t.Fatalf("pattern substitution missing from view: %q", v)
	}
	text(`/\zs`)
	enter()
	if v := view(); !strings.Contains(v, `Unsupported pattern construct: \zs`) {
		t.Fatalf("unsupported construct not reported: %q", v)
	}
	text(`:g/\<foo\>/d`)
	enter()
	if v := view(); strings.Contains(v, "foo") {
		t.Fatalf("global pattern did not delete: %q", v)
	}
	text("u:q!")
	enter()
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Fatal("did not return to pane")
	}
}
