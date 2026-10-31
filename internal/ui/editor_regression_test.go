package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorCountAndExitThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "sample")
	if err := os.WriteFile(path, []byte("one two three four five six seven\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	for _, r := range "2d3w" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	text, linewise := pane.misa.viEditor.Register()
	if !slices.Equal(text, []string{"one two three four five six "}) || linewise {
		t.Fatalf("register = %q, linewise=%v", text, linewise)
	}
	for _, r := range ":q" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if pane.misa.viEditor == nil {
		t.Fatal("unwritten edits must prevent :q")
	}
	for _, r := range ":q!" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("editor exit must not quit the mux")
		}
	}
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Fatal("editor exit did not return to the same mux pane/menu")
	}
	if !slices.Equal(pane.misa.viRegister, text) || pane.misa.viRegLine {
		t.Fatal("editor exit lost the pane register")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "one two three four five six seven\n" {
		t.Fatalf(":q! wrote the file: %q, %v", data, err)
	}
}
