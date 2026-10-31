package vi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Original event-path fixtures for Vim 9.1 nocompatible undo behavior.
func TestUndoF05Replay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "count undo", input: "abcd\n", keys: "xxx2u", want: replayState{lines: []string{"bcd"}, reg: []string{"c"}, modified: true}},
		{name: "count redo", input: "abcd\n", keys: "xxx3u2<C-r>", want: replayState{lines: []string{"cd"}, reg: []string{"c"}, modified: true}},
		{name: "new edit clears redo", input: "abcd\n", keys: "xxuiZ<Esc><C-r>", want: replayState{lines: []string{"Zbcd"}, reg: []string{"b"}, modified: true}},
		{name: "undo cursor changed start", input: "abcdef\n", keys: "llx$u", want: replayState{lines: []string{"abcdef"}, col: 2, reg: []string{"c"}}},
		{name: "redo cursor changed start", input: "abcdef\n", keys: "llxu$<C-r>", want: replayState{lines: []string{"abdef"}, col: 2, reg: []string{"c"}, modified: true}},
		{name: "line restore multiple changes", input: "abcd\n", keys: "xxU", want: replayState{lines: []string{"abcd"}, reg: []string{"b"}, modified: true}},
		{name: "undo line restore", input: "abcd\n", keys: "xxUu", want: replayState{lines: []string{"cd"}, reg: []string{"b"}, modified: true}},
		{name: "line restore other line no op", input: "abcd\nwxyz\n", keys: "xjU", want: replayState{lines: []string{"bcd", "wxyz"}, row: 1, reg: []string{"a"}, modified: true}},
		{name: "save edit undo clean", input: "abcd\n", keys: "x:w<CR>xu", want: replayState{lines: []string{"bcd"}, reg: []string{"b"}}},
		{name: "save undo dirty", input: "abcd\n", keys: "x:w<CR>u", want: replayState{lines: []string{"abcd"}, reg: []string{"a"}, modified: true}},
		{name: "save undo redo clean", input: "abcd\n", keys: "x:w<CR>u<C-r>", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}}},
		{name: "same text different revision stays dirty", input: "abcd\n", keys: "x:w<CR>ux", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "write then quit", input: "abcd\n", keys: "x:w<CR>:q<CR>", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, quit: true}},
		{name: "reload discards undo", skip: "TODO(T05): :e! reload command is not implemented", input: "abcd\n", keys: "x:e!<CR>u", want: replayState{lines: []string{"abcd"}, reg: []string{"a"}}},
		{name: "yank no undo unit", input: "abcd\n", keys: "xyyu", want: replayState{lines: []string{"abcd"}, reg: []string{"bcd"}, linewise: true}},
		{name: "failed find no undo unit", input: "abcd\n", keys: "xdfxu", want: replayState{lines: []string{"abcd"}, reg: []string{"a"}}},
		{name: "moves no undo unit", input: "abcd\n", keys: "xllu", want: replayState{lines: []string{"abcd"}, reg: []string{"a"}}},
		{name: "empty insert no undo unit", input: "abcd\n", keys: "xi<Esc>u", want: replayState{lines: []string{"abcd"}, reg: []string{"a"}}},
		{name: "insert backspace one unit", input: "abcd\n", keys: "iXY<BS>Z<Esc>u", want: replayState{lines: []string{"abcd"}}},
		{name: "insert newline one unit", input: "abcd\n", keys: "iX<CR>Y<Esc>u", want: replayState{lines: []string{"abcd"}}},
		{name: "count insert one unit", input: "abcd\n", keys: "3iX<Esc>u", want: replayState{lines: []string{"abcd"}}},
		{name: "change word one unit", input: "one two\n", keys: "cwX<Esc>u", want: replayState{lines: []string{"one two"}, reg: []string{"one"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

// Only the newest 200 snapshots are retained, including when the saved
// revision falls out of the history. Its identity must not alias a newer one.
func TestUndoF05HistoryLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limit.txt")
	if err := os.WriteFile(path, []byte("z\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	keys := "x:w<CR>" + strings.Repeat("ia<Esc>", 201) + "201u"
	for _, key := range replayKeys(t, keys) {
		e.Update(key)
	}
	if len(e.lines) != 1 || e.lines[0] != "a" || !e.modified {
		t.Errorf("after history eviction: lines=%q modified=%v; want [a], dirty", e.lines, e.modified)
	}
}
