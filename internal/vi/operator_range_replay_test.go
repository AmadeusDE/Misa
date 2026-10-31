package vi

import "testing"

// Original public-event fixtures for Vim 9.1, nocompatible, default options.
// Registers use the editor's representation: one string with embedded newlines
// for characterwise ranges, one string per line for linewise ranges.
func TestOperatorRangeReplay(t *testing.T) {
	fixtures := []replayFixture{
		{name: "dl exclusive", input: "abcdef\n", keys: "lldl", want: replayState{lines: []string{"abdef"}, col: 2, reg: []string{"c"}, modified: true}},
		{name: "dh backward", input: "abcdef\n", keys: "llldh", want: replayState{lines: []string{"abdef"}, col: 2, reg: []string{"c"}, modified: true}},
		{name: "d0 backward", input: "abcdef\n", keys: "llld0", want: replayState{lines: []string{"def"}, reg: []string{"abc"}, modified: true}},
		{name: "d dollar inclusive", input: "abcdef\n", keys: "lld$", want: replayState{lines: []string{"ab"}, col: 1, reg: []string{"cdef"}, modified: true}},
		{name: "counted dollar", input: "abcd\nefgh\nlast\n", keys: "ll2d$", want: replayState{lines: []string{"ab", "last"}, col: 1, reg: []string{"cd\nefgh"}, modified: true}},
		{name: "underscore linewise", input: "first\nsecond\nthird\n", keys: "jld_", want: replayState{lines: []string{"first", "third"}, row: 1, reg: []string{"second"}, linewise: true, modified: true}},
		{name: "counted underscore linewise", input: "first\nsecond\nthird\nfourth\n", keys: "jl2d_", want: replayState{lines: []string{"first", "fourth"}, row: 1, reg: []string{"second", "third"}, linewise: true, modified: true}},
		{name: "dj", input: "one\ntwo\nthree\nfour\nfive\n", keys: "jldj", want: replayState{lines: []string{"one", "four", "five"}, row: 1, reg: []string{"two", "three"}, linewise: true, modified: true}},
		{name: "dk", input: "one\ntwo\nthree\nfour\nfive\n", keys: "jjldk", want: replayState{lines: []string{"one", "four", "five"}, row: 1, reg: []string{"two", "three"}, linewise: true, modified: true}},
		{name: "2dj", input: "one\ntwo\nthree\nfour\nfive\n", keys: "jl2dj", want: replayState{lines: []string{"one", "five"}, row: 1, reg: []string{"two", "three", "four"}, linewise: true, modified: true}},
		{name: "dG from middle", input: "one\ntwo\nthree\nfour\n", keys: "jldG", want: replayState{lines: []string{"one"}, reg: []string{"two", "three", "four"}, linewise: true, modified: true}},
		{name: "dgg from middle", input: "one\ntwo\nthree\nfour\n", keys: "jjldgg", want: replayState{lines: []string{"four"}, reg: []string{"one", "two", "three"}, linewise: true, modified: true}},
		{name: "dw", input: "one two three\n", keys: "dw", want: replayState{lines: []string{"two three"}, reg: []string{"one "}, modified: true}},
		{name: "de inclusive", input: "one two\n", keys: "de", want: replayState{lines: []string{" two"}, reg: []string{"one"}, modified: true}},
		{name: "db backward", input: "one two three\n", keys: "wwdb", want: replayState{lines: []string{"one three"}, col: 4, reg: []string{"two "}, modified: true}},
		{name: "dw at eol does not join", input: "one\ntwo\n", keys: "dw", want: replayState{lines: []string{"", "two"}, reg: []string{"one"}, modified: true}},
		{name: "dw at eof includes last character", input: "one\n", keys: "dw", want: replayState{lines: []string{""}, reg: []string{"one"}, modified: true}},
		{name: "cw changes word not trailing blank", input: "one two\n", keys: "cw", want: replayState{lines: []string{" two"}, mode: ModeInsert, reg: []string{"one"}, modified: true}},
		{name: "cw on single blank", input: "one two\n", keys: "lllcw", want: replayState{lines: []string{"onetwo"}, col: 3, mode: ModeInsert, reg: []string{" "}, modified: true}},
		{name: "ce", input: "one two\n", keys: "ce", want: replayState{lines: []string{" two"}, mode: ModeInsert, reg: []string{"one"}, modified: true}},
		{name: "c dollar", input: "abcdef\n", keys: "llc$", want: replayState{lines: []string{"ab"}, col: 2, mode: ModeInsert, reg: []string{"cdef"}, modified: true}},
		{name: "cc", input: "one\ntwo\n", keys: "cc", want: replayState{lines: []string{"", "two"}, mode: ModeInsert, reg: []string{"one"}, linewise: true, modified: true}},
		{name: "2cc", input: "one\ntwo\nthree\n", keys: "2cc", want: replayState{lines: []string{"", "three"}, mode: ModeInsert, reg: []string{"one", "two"}, linewise: true, modified: true}},
		{name: "yw", input: "one two\n", keys: "yw", want: replayState{lines: []string{"one two"}, reg: []string{"one "}}},
		{name: "ye", input: "one two\n", keys: "ye", want: replayState{lines: []string{"one two"}, reg: []string{"one"}}},
		{name: "y dollar", input: "abcdef\n", keys: "lly$", want: replayState{lines: []string{"abcdef"}, col: 2, reg: []string{"cdef"}}},
		{name: "exclusive column zero becomes linewise", input: "one.\ntwo\n", keys: "d)", want: replayState{lines: []string{"two"}, reg: []string{"one."}, linewise: true, modified: true}},
		{name: "exclusive column zero after first nonblank", input: "one.\ntwo\n", keys: "ld)", want: replayState{lines: []string{"o", "two"}, reg: []string{"ne."}, modified: true}},
		{name: "multiline character change undo", input: "one\ntwo\nthree\n", keys: "ll2c$X<Esc>u", want: replayState{lines: []string{"one", "two", "three"}, col: 2, reg: []string{"e\ntwo"}}},
		{name: "multiline character change redo", input: "one\ntwo\nthree\n", keys: "ll2c$X<Esc>u<C-r>", want: replayState{lines: []string{"onX", "three"}, col: 2, reg: []string{"e\ntwo"}, modified: true}},
		{name: "multiline change undo", input: "one\ntwo\nthree\n", keys: "2ccX<Esc>u", want: replayState{lines: []string{"one", "two", "three"}, reg: []string{"one", "two"}, linewise: true}},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestPreferredColumnReplay(t *testing.T) {
	fixtures := []replayFixture{
		{name: "vertical restores column after short line", input: "abcdef\nx\nabcdef\n", keys: "lllljj", want: replayState{lines: []string{"abcdef", "x", "abcdef"}, row: 2, col: 4}},
		{name: "dollar keeps end through vertical motions", input: "abc\nx\nabcdef\n", keys: "$jjk", want: replayState{lines: []string{"abc", "x", "abcdef"}, row: 1}},
		{name: "dollar follows longer line", input: "abc\nx\nabcdef\n", keys: "$jj", want: replayState{lines: []string{"abc", "x", "abcdef"}, row: 2, col: 5}},
		{name: "dj then vertical motion", input: "abcdef\none\ntwo\nx\nabcdef\n", keys: "lllldjj", want: replayState{lines: []string{"two", "x", "abcdef"}, row: 1, col: 0, reg: []string{"abcdef", "one"}, linewise: true, modified: true}},
		{name: "dk then vertical motion", input: "abcdef\none\ntwo\nx\nabcdef\n", keys: "jjlldkj", want: replayState{lines: []string{"abcdef", "x", "abcdef"}, row: 2, reg: []string{"one", "two"}, linewise: true, modified: true}},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
