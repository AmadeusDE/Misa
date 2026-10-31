package vi

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUndoEntryStandaloneSavedRevisionQuit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("pear\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	m := &standaloneModel{editor: e}
	for _, key := range replayKeys(t, "x:w<CR>xu") {
		m.Update(key)
	}
	if e.modified {
		t.Error("undo to saved revision must be clean")
	}
	var cmd tea.Cmd
	for _, key := range replayKeys(t, ":q<CR>") {
		_, cmd = m.Update(key)
	}
	if !e.Quit || cmd == nil {
		t.Fatalf("Quit=%v command present=%v, want clean quit", e.Quit, cmd != nil)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("standalone exit did not emit tea.QuitMsg")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "ear\n" {
		t.Errorf("saved=%q err=%v, want ear\\n", data, err)
	}
}
