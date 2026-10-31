package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorMotionWriteQuitThroughMux(t *testing.T) {
	for _, tc := range []struct {
		name, input, keys, saved, reg string
		enter                         bool
	}{
		{"find", "abxc\n", "dfx", "c\n", "abx", false},
		{"search", "aa foo\n", "d/foo", "foo\n", "aa ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			m := newRoot(t, 80, 24)
			m.addMuxPane()
			pane := m.activePane()
			path := filepath.Join(t.TempDir(), "sample")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			pane.misa.screen = screenMenu
			pane.misa.enterEditor(path)
			for _, r := range tc.keys {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			if tc.enter {
				m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
			reg, linewise := pane.misa.viEditor.Register()
			if !slices.Equal(reg, []string{tc.reg}) || linewise {
				t.Errorf("register=%q linewise=%v, want characterwise %q", reg, linewise, tc.reg)
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
			if !slices.Equal(pane.misa.viRegister, []string{tc.reg}) || pane.misa.viRegLine {
				t.Errorf("retained register=%q linewise=%v", pane.misa.viRegister, pane.misa.viRegLine)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != tc.saved {
				t.Errorf("saved=%q err=%v, want %q", data, err, tc.saved)
			}
		})
	}
}
