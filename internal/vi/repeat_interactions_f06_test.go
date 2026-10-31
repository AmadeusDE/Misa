package vi

import (
	"slices"
	"testing"
)

func TestRepeatUndoUnitF06(t *testing.T) {
	for _, f := range []struct{ name, input, before, dot string }{
		{"delete words", "one two three four five six seven eight nine ten\n", "d3w", "2."},
		{"change words", "one two three four five six\n", "cwX<Esc>w", "2."},
		{"insert stream", "abc def\n", "ia<BS>b<CR>c<Esc>w", "."},
		{"append newline", "abc\n", "AX<CR>Y<Esc>", "."},
		{"open below", "abc\n", "oX<Esc>", "3."},
		{"open above", "abc\n", "OX<Esc>", "3."},
		{"replace chars", "abcdefghi\n", "3rxl", "."},
		{"replace stream", "abcd efgh\n", "RXY<BS>Z<Esc>w", "."},
		{"put after", "abc\n", "x3p", "."},
		{"put before", "abc\n", "x3P", "."},
	} {
		t.Run(f.name, func(t *testing.T) {
			e := New("", t.TempDir())
			e.lines = []string{f.input[:len(f.input)-1]}
			e.Resize(80, 24)
			for _, k := range replayKeys(t, f.before) {
				e.Update(k)
			}
			before := slices.Clone(e.lines)
			for _, k := range replayKeys(t, f.dot) {
				e.Update(k)
			}
			after := slices.Clone(e.lines)
			if slices.Equal(before, after) {
				t.Fatalf("dot was a no-op: %q", after)
			}
			e.Update(keyFromName("u"))
			if !slices.Equal(e.lines, before) {
				t.Fatalf("undo: got %q want %q", e.lines, before)
			}
			e.Update(keyFromName("ctrl+r"))
			if !slices.Equal(e.lines, after) {
				t.Fatalf("redo: got %q want %q", e.lines, after)
			}
		})
	}
}

func TestRepeatCountAndRegisterF06(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "insert backspace dot", input: "xy\n", keys: "ia<BS>b<Esc>.", want: replayState{lines: []string{"bbxy"}, modified: true}},
		{name: "empty buffer insert dot", input: "", keys: "iX<Esc>.", want: replayState{lines: []string{"XX"}, modified: true}},
		{name: "delete words count replacement persists", input: "a b c d e f g h i j k l\n", keys: "d3w2..", want: replayState{lines: []string{"h i j k l"}, reg: []string{"f g "}, modified: true}},
		{name: "three x retains count", input: "abcdefghij\n", keys: "3x.", want: replayState{lines: []string{"ghij"}, reg: []string{"def"}, modified: true}},
		{name: "failed find retains last", input: "abcdef\n", keys: "xdfz.", want: replayState{lines: []string{"cdef"}, reg: []string{"b"}, modified: true}},
		{name: "named put retains selection after yank", input: "abc def\n", keys: "\"aywl\"ap0yl$.", want: replayState{lines: []string{"ababc c defabc "}, col: 14, reg: []string{"abc "}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestRepeatPendingResetHistoryF06(t *testing.T) {
	e := New("", t.TempDir())
	for _, k := range replayKeys(t, "iX") {
		e.Update(k)
	}
	if e.repeatPending.command != "i" {
		t.Fatal("fixture did not start pending insert")
	}
	e.resetHistory()
	if e.repeatPending.command != "" {
		t.Fatal("history reset retained a pending change")
	}
}
