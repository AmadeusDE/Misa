package vi

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Exercise search options through the same Bubble Tea adapter as the standalone editor.
func TestT04StandaloneSearchOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("start\nFOO\nFoo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := newStandaloneEditor(path, dir, nil)
	m := &standaloneModel{editor: e}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 15})
	send := func(keys string) {
		t.Helper()
		for _, key := range replayKeys(t, keys) {
			if _, cmd := m.Update(key); cmd != nil {
				if _, quit := cmd().(tea.QuitMsg); quit {
					t.Fatal("unexpected quit")
				}
			}
		}
	}
	send(":set ignorecase incsearch<CR>/foo")
	if e.row != 1 {
		t.Fatalf("incremental case-insensitive search row = %d, want 1", e.row)
	}
	send("<Esc>")
	if e.row != 0 || e.col != 0 {
		t.Fatalf("cancelled search cursor = (%d,%d), want (0,0)", e.row, e.col)
	}
	send("/\\CFoo<CR>")
	if e.row != 2 {
		t.Fatalf("case override search row = %d, want 2", e.row)
	}
	send("gg/foo<CR>")
	if e.row != 1 {
		t.Fatalf("case-insensitive search row = %d, want 1", e.row)
	}
}
