package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorRangeWriteQuitThroughMux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	path := filepath.Join(t.TempDir(), "sample")
	if err := os.WriteFile(path, []byte("abcd\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pane.misa.screen = screenMenu
	pane.misa.enterEditor(path)
	for _, r := range "dl" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	reg, linewise := pane.misa.viEditor.Register()
	if !slices.Equal(reg, []string{"a"}) || linewise {
		t.Errorf("register=%q linewise=%v, want characterwise a", reg, linewise)
	}
	for _, r := range ":wq" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Error("editor exit quit Misa")
		}
	}
	if pane.misa.viEditor != nil || pane.misa.screen != screenMenu || m.activePane() != pane {
		t.Error("editor did not return to the same pane/menu")
	}
	if !slices.Equal(pane.misa.viRegister, []string{"a"}) || pane.misa.viRegLine {
		t.Errorf("retained register=%q linewise=%v", pane.misa.viRegister, pane.misa.viRegLine)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "bcd\n" {
		t.Errorf("saved=%q err=%v, want bcd\\n", data, err)
	}
}
