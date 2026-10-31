package vi

import "testing"

func TestT04CaseSearchAndSubstitute(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "default sensitive search", input: "a FOO foo\n", keys: "/foo<CR>", want: replayState{lines: []string{"a FOO foo"}, col: 6}},
		{name: "ignorecase search", input: "a FOO foo\n", keys: ":set ic<CR>/foo<CR>", want: replayState{lines: []string{"a FOO foo"}, col: 2}},
		{name: "sensitive atom", input: "a FOO foo\n", keys: ":set ic<CR>/foo\\C<CR>", want: replayState{lines: []string{"a FOO foo"}, col: 6}},
		{name: "insensitive atom", input: "a FOO foo\n", keys: "/foo\\c<CR>", want: replayState{lines: []string{"a FOO foo"}, col: 2}},
		{name: "unicode ignorecase", input: "a ÉTÉ été\n", keys: ":set ic<CR>/été<CR>", want: replayState{lines: []string{"a ÉTÉ été"}, col: 2}},
		{name: "option disabled", input: "a FOO foo\n", keys: ":set ic<CR>:set noic<CR>/foo<CR>", want: replayState{lines: []string{"a FOO foo"}, col: 6}},
		{name: "repeat uses current option", input: "a foo FOO foo\n", keys: "/foo<CR>:set ic<CR>n", want: replayState{lines: []string{"a foo FOO foo"}, col: 6}},
		{name: "backward search", input: "a foo FOO x\n", keys: ":set ic<CR>$?foo<CR>", want: replayState{lines: []string{"a foo FOO x"}, col: 6}},
		{name: "substitute ignorecase", input: "FOO foo\n", keys: ":set ic<CR>:s/foo/X/g<CR>", want: replayState{lines: []string{"X X"}, modified: true}},
		{name: "substitute sensitive atom", input: "FOO foo\n", keys: ":set ic<CR>:s/foo\\C/X/g<CR>", want: replayState{lines: []string{"FOO X"}, modified: true}},
		{name: "substitute insensitive atom", input: "FOO foo\n", keys: ":s/foo\\c/X/g<CR>", want: replayState{lines: []string{"X X"}, modified: true}},
		{name: "substitute insensitive flag", input: "FOO foo\n", keys: ":s/foo/X/gi<CR>", want: replayState{lines: []string{"X X"}, modified: true}},
		{name: "sensitive atom overrides i flag", input: "FOO foo\n", keys: ":s/foo\\C/X/gi<CR>", want: replayState{lines: []string{"FOO X"}, modified: true}},
		{name: "reused pattern keeps case atom", input: "FOO foo\n", keys: "/foo\\c<CR>:s//X/g<CR>", want: replayState{lines: []string{"X X"}, col: 4, modified: true}},
		{name: "confirm ignorecase", input: "FOO foo\n", keys: ":set ic<CR>:s/foo/X/gc<CR>yy", want: replayState{lines: []string{"X X"}, modified: true}},
		{name: "global ignorecase", input: "FOO\nbar\n", keys: ":set ic<CR>:g/foo/d<CR>", want: replayState{lines: []string{"bar"}, reg: []string{"FOO"}, linewise: true, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
