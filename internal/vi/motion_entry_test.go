package vi

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMotionEntryStandaloneWriteQuit(t *testing.T) {
	for _, tc := range []struct{ name, input, keys, saved, reg string }{
		{"find", "abxc\n", "dfx", "c\n", "abx"},
		{"search", "aa foo\n", "d/foo<CR>", "foo\n", "aa "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
			path := filepath.Join(dir, "sample")
			if err := os.WriteFile(path, []byte(tc.input), 0600); err != nil {
				t.Fatal(err)
			}
			e := New(path, dir)
			m := &standaloneModel{editor: e}
			for _, key := range replayKeys(t, tc.keys) {
				m.Update(key)
			}
			reg, linewise := e.Register()
			if !slices.Equal(reg, []string{tc.reg}) || linewise {
				t.Errorf("register=%q linewise=%v, want characterwise %q", reg, linewise, tc.reg)
			}
			if e.col != 0 {
				t.Errorf("column=%d, want 0", e.col)
			}
			var cmd tea.Cmd
			for _, key := range replayKeys(t, ":wq<CR>") {
				_, cmd = m.Update(key)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != tc.saved {
				t.Errorf("saved=%q err=%v, want %q", data, err, tc.saved)
			}
			if !e.Quit || cmd == nil {
				t.Fatalf("Quit=%v command present=%v", e.Quit, cmd != nil)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Error("standalone exit did not emit tea.QuitMsg")
			}
			reg, linewise = e.Register()
			if !slices.Equal(reg, []string{tc.reg}) || linewise {
				t.Errorf("retained register=%q linewise=%v", reg, linewise)
			}
		})
	}
}
