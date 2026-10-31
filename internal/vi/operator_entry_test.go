package vi

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOperatorEntryStandaloneDeleteRightWriteQuit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("abcd\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	m := &standaloneModel{editor: e}
	for _, key := range replayKeys(t, "dl") {
		m.Update(key)
	}
	reg, linewise := e.Register()
	if !slices.Equal(reg, []string{"a"}) || linewise {
		t.Errorf("register=%q linewise=%v, want characterwise a", reg, linewise)
	}
	var cmd tea.Cmd
	for _, key := range replayKeys(t, ":wq<CR>") {
		_, cmd = m.Update(key)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "bcd\n" {
		t.Errorf("saved=%q err=%v, want bcd\\n", data, err)
	}
	if !e.Quit || cmd == nil {
		t.Fatalf("Quit=%v command present=%v", e.Quit, cmd != nil)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("standalone exit did not emit tea.QuitMsg")
	}
}
