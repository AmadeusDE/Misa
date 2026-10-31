package vi

import "testing"

func TestOperatorRangeEdgesReplay(t *testing.T) {
	fixtures := []replayFixture{
		{name: "dh at column zero preserves register", input: "abc\ndef\n", keys: "yljdh", want: replayState{lines: []string{"abc", "def"}, row: 1, reg: []string{"a"}}},
		{name: "d0 at column zero is a no-op", input: "abc\n", keys: "d0", want: replayState{lines: []string{"abc"}}},
		{name: "dl includes final character", input: "abc\n", keys: "$dl", want: replayState{lines: []string{"ab"}, col: 1, reg: []string{"c"}, modified: true}},
		{name: "cw consumes whitespace like dw", input: "one  two\n", keys: "lllcw", want: replayState{lines: []string{"onetwo"}, col: 3, mode: ModeInsert, reg: []string{"  "}, modified: true}},
		{name: "cw at word end does not change next word", input: "one two\n", keys: "llcw", want: replayState{lines: []string{"on two"}, col: 2, mode: ModeInsert, reg: []string{"e"}, modified: true}},
		{name: "counted cw", input: "one two three\n", keys: "2cw", want: replayState{lines: []string{" three"}, mode: ModeInsert, reg: []string{"one two"}, modified: true}},
		{name: "right indent exclusive range", input: "abc\ndef\n", keys: ">l", want: replayState{lines: []string{"\tabc", "def"}, col: 1, modified: true}},
		{name: "left indent vertical range", input: "\tabc\n\tdef\n\tghi\n", keys: "<lt>j", want: replayState{lines: []string{"abc", "def", "\tghi"}, modified: true}},
		{name: "counted dw at line boundary stays characterwise", input: "one\ntwo\nthree\n", keys: "2dw", want: replayState{lines: []string{"", "three"}, reg: []string{"one\ntwo"}, modified: true}},
		{name: "yw at line boundary excludes newline", input: "one\ntwo\n", keys: "yw", want: replayState{lines: []string{"one", "two"}, reg: []string{"one"}}},
		{name: "indent excludes column zero destination", input: "one.\ntwo\n", keys: ">)", want: replayState{lines: []string{"\tone.", "two"}, col: 1, modified: true}},
		{name: "operator count on G is absolute", input: "one\ntwo\nthree\nfour\n", keys: "3dG", want: replayState{lines: []string{"four"}, reg: []string{"one", "two", "three"}, linewise: true, modified: true}},
		{name: "operator count on gg is absolute", input: "one\ntwo\nthree\nfour\n", keys: "G2dgg", want: replayState{lines: []string{"one"}, reg: []string{"two", "three", "four"}, linewise: true, modified: true}},
		{name: "backward yank places cursor at range start", input: "one two\n", keys: "wyb", want: replayState{lines: []string{"one two"}, reg: []string{"one "}}},
		{name: "backward word across newline stays characterwise", input: "one\ntwo\n", keys: "jdb", want: replayState{lines: []string{"two"}, reg: []string{"one\n"}, modified: true}},
		{name: "backward word preserves earlier text and removes newline", input: "red blue\ngreen\n", keys: "jdb", want: replayState{lines: []string{"red green"}, col: 4, reg: []string{"blue\n"}, modified: true}},
		{name: "empty line dollar delete is no-op", input: "abc\n\n", keys: "yljd$", want: replayState{lines: []string{"abc", ""}, row: 1, reg: []string{"a"}}},
		{name: "backward no-op does not consume undo", input: "abc\n", keys: "xdhu", want: replayState{lines: []string{"abc"}, reg: []string{"a"}}},
		{name: "empty dollar no-op does not consume undo", input: "abc\n\n", keys: "xjd$u", want: replayState{lines: []string{"abc", ""}, reg: []string{"a"}}},
		{name: "empty line dollar change enters insert", input: "\n", keys: "c$X<Esc>", want: replayState{lines: []string{"X"}, modified: true}},
		{name: "invalid operator g prefix does not navigate", input: "alpha\nalpha beta\n", keys: "jdgd", want: replayState{lines: []string{"alpha", "alpha beta"}, row: 1}},
		{name: "counted vertical change before insert", input: "one\ntwo\nthree\nfour\n", keys: "2cj", want: replayState{lines: []string{"", "four"}, mode: ModeInsert, reg: []string{"one", "two", "three"}, linewise: true, modified: true}},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
