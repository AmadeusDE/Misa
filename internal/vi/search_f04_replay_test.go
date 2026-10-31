package vi

import "testing"

func TestF04SearchReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "operator forward wraps", input: "foo abc\n", keys: "$d/foo<CR>", want: replayState{lines: []string{"c"}, reg: []string{"foo ab"}, modified: true}},
		{name: "operator backward wraps", input: "foo foo\n", keys: "d?foo<CR>", want: replayState{lines: []string{"foo"}, reg: []string{"foo "}, modified: true}},
		{name: "backward N then n preserves direction", input: "x a x b x c\n", keys: "$?x<CR>Nn", want: replayState{lines: []string{"x a x b x c"}, col: 8}},
		{name: "backward n then N preserves direction", input: "x a x b x c\n", keys: "$?x<CR>nN", want: replayState{lines: []string{"x a x b x c"}, col: 8}},
		{name: "empty origin converts linewise", input: "\nfoo\ntail\n", keys: "d/foo<CR>", want: replayState{lines: []string{"foo", "tail"}, reg: []string{""}, linewise: true, modified: true}},
		{name: "nonblank origin to next col zero", input: "alpha\nfoo\ntail\n", keys: "lld/foo<CR>", want: replayState{lines: []string{"al", "foo", "tail"}, col: 1, reg: []string{"pha"}, modified: true}},
		{name: "forward", input: "one foo two foo\n", keys: "/foo<CR>", want: replayState{lines: []string{"one foo two foo"}, col: 4}},
		{name: "backward wraps", input: "one foo two foo\n", keys: "?foo<CR>", want: replayState{lines: []string{"one foo two foo"}, col: 12}},
		{name: "n forward wraps", input: "foo x foo\n", keys: "/foo<CR>n", want: replayState{lines: []string{"foo x foo"}}},
		{name: "N reverses temporarily", input: "foo x foo x foo\n", keys: "/foo<CR>Nn", want: replayState{lines: []string{"foo x foo x foo"}, col: 6}},
		{name: "count repeat", input: "a x x x x\n", keys: "/x<CR>3n", want: replayState{lines: []string{"a x x x x"}, col: 8}},
		{name: "count search", input: "a x x x\n", keys: "2/x<CR>", want: replayState{lines: []string{"a x x x"}, col: 4}},
		{name: "empty search reuses", input: "a x x\n", keys: "/x<CR>/<CR>", want: replayState{lines: []string{"a x x"}, col: 4}},
		{name: "empty line", input: "\nfoo\n", keys: "/foo<CR>", want: replayState{lines: []string{"", "foo"}, row: 1}},
		{name: "missing search", input: "abc\n", keys: "/zzz<CR>", want: replayState{lines: []string{"abc"}}},
		{name: "delete forward", input: "abc foo xyz\n", keys: "d/foo<CR>", want: replayState{lines: []string{"foo xyz"}, reg: []string{"abc "}, modified: true}},
		{name: "delete backward", input: "foo abc xyz\n", keys: "$d?foo<CR>", want: replayState{lines: []string{"z"}, reg: []string{"foo abc xy"}, modified: true}},
		{name: "delete repeat search", input: "x a x b x\n", keys: "/x<CR>dn", want: replayState{lines: []string{"x a x"}, col: 4, reg: []string{"x b "}, modified: true}},
		{name: "multiply operator search count", input: "a x x x x x x z\n", keys: "2d3/x<CR>", want: replayState{lines: []string{"x z"}, reg: []string{"a x x x x x "}, modified: true}},
		{name: "missing operator no undo", input: "abc\n", keys: "d/zzz<CR>u", want: replayState{lines: []string{"abc"}}},
		{name: "col zero converts linewise", input: "alpha\nfoo\ntail\n", keys: "d/foo<CR>", want: replayState{lines: []string{"foo", "tail"}, reg: []string{"alpha"}, linewise: true, modified: true}},
		{name: "cancel search count", input: "abc\n", keys: "2d/foo<Esc>x", want: replayState{lines: []string{"bc"}, reg: []string{"a"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
