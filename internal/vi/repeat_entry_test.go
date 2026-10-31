package vi

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestRepeatEntryStandalone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("one two three\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	m := &standaloneModel{editor: e}
	for _, key := range replayKeys(t, "cwX<Esc>w.u") {
		m.Update(key)
	}
	if !slices.Equal(e.lines, []string{"X two three"}) {
		t.Fatalf("undo dot: %q", e.lines)
	}
	var cmd tea.Cmd
	for _, key := range replayKeys(t, "<C-r>:wq<CR>") {
		_, cmd = m.Update(key)
	}
	if !e.Quit || cmd == nil {
		t.Fatalf("Quit=%v command present=%v", e.Quit, cmd != nil)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("standalone exit did not emit tea.QuitMsg")
	}
	if reg, linewise := e.Register(); !slices.Equal(reg, []string{"two"}) || linewise {
		t.Errorf("register=%q linewise=%v", reg, linewise)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "X X three\n" {
		t.Errorf("saved=%q err=%v", data, err)
	}
}
