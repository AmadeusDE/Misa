package vi

import (
	"reflect"
	"testing"
)

func TestIncSearchPreviewAndRestore(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"start", "other", "foo bar", "food", "last"}
	e.Resize(8, 4)
	e.row, e.col, e.topLine, e.leftCol = 0, 0, 0, 2
	e.searchPat, e.searchDir = "old", -1
	e.jumpList, e.jumpIdx = []pos{{4, 1}}, 1
	send := func(keys string) {
		for _, k := range replayKeys(t, keys) {
			e.Update(k)
		}
	}
	send(":set incsearch<CR>/fo")
	if e.row != 2 || e.col != 0 {
		t.Fatalf("preview = %d,%d", e.row, e.col)
	}
	if e.searchPat != "old" || e.searchDir != -1 || !reflect.DeepEqual(e.jumpList, []pos{{4, 1}}) {
		t.Fatal("preview changed committed search or jumps")
	}
	send("z") // no match restores the origin
	if e.row != 0 || e.col != 0 || e.topLine != 0 || e.leftCol != 2 {
		t.Fatalf("no match did not restore origin: %d,%d viewport %d,%d", e.row, e.col, e.topLine, e.leftCol)
	}
	send("<BS>")
	if e.row != 2 || e.col != 0 {
		t.Fatalf("backspace did not recover preview: %d,%d", e.row, e.col)
	}
	send("[\\(") // incomplete Vim grouping must not leak an error
	if e.row != 0 || e.col != 0 || e.statusMsg != "" {
		t.Fatalf("invalid preview: %d,%d status %q", e.row, e.col, e.statusMsg)
	}
	send("<Esc>")
	if e.row != 0 || e.col != 0 || e.topLine != 0 || e.leftCol != 2 || e.searchPat != "old" || e.searchDir != -1 || e.jumpIdx != 1 {
		t.Fatalf("cancel failed to restore state: %+v", e)
	}
}

func TestIncSearchCommitFromOrigin(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "count", input: "a x x x\n", keys: ":set incsearch<CR>2/x<CR>", want: replayState{lines: []string{"a x x x"}, col: 4}},
		{name: "operator", input: "abc foo xyz\n", keys: ":set incsearch<CR>d/foo<CR>", want: replayState{lines: []string{"foo xyz"}, reg: []string{"abc "}, modified: true}},
		{name: "backward", input: "foo x foo\n", keys: ":set incsearch<CR>?foo<CR>", want: replayState{lines: []string{"foo x foo"}, col: 6}},
		{name: "empty reuse", input: "a x x\n", keys: ":set incsearch<CR>/x<CR>/<CR>", want: replayState{lines: []string{"a x x"}, col: 4}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestIncSearchDisabled(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"a foo"}
	e.Resize(80, 24)
	for _, k := range replayKeys(t, "/foo") {
		e.Update(k)
	}
	if e.row != 0 || e.col != 0 {
		t.Fatalf("disabled preview moved cursor: %d,%d", e.row, e.col)
	}
}
