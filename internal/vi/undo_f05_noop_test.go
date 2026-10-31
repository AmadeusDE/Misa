package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func newUndoF05Editor(t *testing.T, text string) *Editor {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	return e
}

func sendUndoF05(t *testing.T, e *Editor, keys string) {
	t.Helper()
	for _, key := range replayKeys(t, keys) {
		e.Update(key)
	}
}

func assertUndoF05(t *testing.T, e *Editor, lines []string, dirty bool) {
	t.Helper()
	if !reflect.DeepEqual(e.lines, lines) || e.modified != dirty {
		t.Errorf("lines=%q dirty=%v; want %q dirty=%v", e.lines, e.modified, lines, dirty)
	}
}

func TestUndoF05Noops(t *testing.T) {
	cases := []struct{ name, dirtyNoop, cleanNoop string }{
		{"replace empty line", "jrX", "jrX"},
		{"delete empty line", "jx", "jx"},
		{"join last line", "jJ", "jJ"},
		{"replace same character", "rZ", "ra"},
		{"substitute missing", ":s/Q/R/<CR>", ":s/Q/R/<CR>"},
		{"substitute identical", ":s/a/a/<CR>", ":s/a/a/<CR>"},
		{"insert backspace", "iQ<BS><Esc>", "iQ<BS><Esc>"},
		{"insert newline backspace", "i<CR><BS><Esc>", "i<CR><BS><Esc>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("undo previous edit", func(t *testing.T) {
				e := newUndoF05Editor(t, "abc\n\n")
				sendUndoF05(t, e, "iZ<Esc>"+tc.dirtyNoop)
				assertUndoF05(t, e, []string{"Zabc", ""}, true)
				sendUndoF05(t, e, "u")
				assertUndoF05(t, e, []string{"abc", ""}, false)
			})
			t.Run("preserve redo", func(t *testing.T) {
				e := newUndoF05Editor(t, "abc\n\n")
				sendUndoF05(t, e, "iZ<Esc>u"+tc.cleanNoop)
				assertUndoF05(t, e, []string{"abc", ""}, false)
				sendUndoF05(t, e, "<C-r>")
				assertUndoF05(t, e, []string{"Zabc", ""}, true)
			})
		})
	}
}

func TestUndoF05ExRangeSubstituteUnit(t *testing.T) {
	e := newUndoF05Editor(t, "cat\ncat\ncat\n")
	sendUndoF05(t, e, ":1,2s/cat/dog/<CR>")
	assertUndoF05(t, e, []string{"dog", "dog", "cat"}, true)
	sendUndoF05(t, e, "u")
	assertUndoF05(t, e, []string{"cat", "cat", "cat"}, false)
	sendUndoF05(t, e, "<C-r>")
	assertUndoF05(t, e, []string{"dog", "dog", "cat"}, true)
}

func TestUndoF05FailedSaveStaysDirty(t *testing.T) {
	e := newUndoF05Editor(t, "abc\n")
	sendUndoF05(t, e, "iZ<Esc>")
	// Saving to an existing directory fails even when tests run as root.
	sendUndoF05(t, e, ":w "+filepath.Dir(e.filename)+"<CR>")
	assertUndoF05(t, e, []string{"Zabc"}, true)
	sendUndoF05(t, e, ":q<CR>")
	if e.Quit {
		t.Error("failed save must not allow :q to discard the edit")
	}
}

func TestUndoF05OuterTransactionPublicEdits(t *testing.T) {
	e := newUndoF05Editor(t, "abcd\n")
	e.beginEdit()
	sendUndoF05(t, e, "xx")
	e.commitEdit()
	assertUndoF05(t, e, []string{"cd"}, true)
	sendUndoF05(t, e, "u")
	assertUndoF05(t, e, []string{"abcd"}, false)
	sendUndoF05(t, e, "<C-r>")
	assertUndoF05(t, e, []string{"cd"}, true)
}
