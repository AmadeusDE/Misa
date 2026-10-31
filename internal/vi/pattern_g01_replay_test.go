package vi

import (
	"strings"
	"testing"
)

func TestG01SearchReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "dot matches any", input: "foo bar\n", keys: "/o.b<CR>", want: replayState{lines: []string{"foo bar"}, col: 2}},
		{name: "star repetition", input: "a bxxx c\n", keys: "/bx*<CR>", want: replayState{lines: []string{"a bxxx c"}, col: 2}},
		{name: "word boundaries", input: "foobar bar\n", keys: "/\\<lt>bar\\><CR>", want: replayState{lines: []string{"foobar bar"}, col: 7}},
		{name: "alternation leftmost", input: "xx cat dog\n", keys: "/dog\\|cat<CR>", want: replayState{lines: []string{"xx cat dog"}, col: 3}},
		{name: "very magic group", input: "a abab\n", keys: "/\\v(ab){2}<CR>", want: replayState{lines: []string{"a abab"}, col: 2}},
		{name: "class and plus", input: "ab 123\n", keys: "/\\d\\+<CR>", want: replayState{lines: []string{"ab 123"}, col: 3}},
		{name: "collection", input: "ab-9\n", keys: "/[0-9]<CR>", want: replayState{lines: []string{"ab-9"}, col: 3}},
		{name: "start anchor across lines", input: "xb\nbb\n", keys: "/^b<CR>", want: replayState{lines: []string{"xb", "bb"}, row: 1}},
		{name: "end anchor lands on last char", input: "ab\ncd\n", keys: "/$<CR>", want: replayState{lines: []string{"ab", "cd"}, col: 1}},
		{name: "end anchor advances", input: "ab\ncd\n", keys: "/$<CR>n", want: replayState{lines: []string{"ab", "cd"}, row: 1, col: 1}},
		{name: "backward", input: "a1 b22 c\n", keys: "$?\\d\\+<CR>", want: replayState{lines: []string{"a1 b22 c"}, col: 5}},
		{name: "backward overlapping starts", input: "a22\n", keys: "$?\\d\\+<CR>", want: replayState{lines: []string{"a22"}, col: 1}},
		{name: "backward escaped question is literal", input: "a?b ab\n", keys: "$?a\\?b<CR>", want: replayState{lines: []string{"a?b ab"}}},
		{name: "n and N", input: "x1 x2 x3\n", keys: "/x\\d<CR>nN", want: replayState{lines: []string{"x1 x2 x3"}, col: 3}},
		{name: "counted", input: "x1 x2 x3\n", keys: "2/x\\d<CR>", want: replayState{lines: []string{"x1 x2 x3"}, col: 6}},
		{name: "empty pattern reuses regex", input: "a1 b2 c3\n", keys: "/\\d<CR>/<CR>", want: replayState{lines: []string{"a1 b2 c3"}, col: 4}},
		{name: "wraps", input: "a1 b\n", keys: "$/\\d<CR>", want: replayState{lines: []string{"a1 b"}, col: 1}},
		{name: "unicode columns", input: "ααα ββ γγ\n", keys: "/β\\+<CR>", want: replayState{lines: []string{"ααα ββ γγ"}, col: 4}},
		{name: "unicode word boundary", input: "été xété été\n", keys: "/\\<lt>été\\><CR>", want: replayState{lines: []string{"été xété été"}, col: 9}},
		{name: "ignore case atom", input: "a FOO\n", keys: "/foo\\c<CR>", want: replayState{lines: []string{"a FOO"}, col: 2}},
		{name: "operator with pattern", input: "abc 12 def\n", keys: "d/\\d\\+<CR>", want: replayState{lines: []string{"12 def"}, reg: []string{"abc "}, modified: true}},
		{name: "operator backward pattern", input: "ab12cd\n", keys: "$d?\\d<CR>", want: replayState{lines: []string{"ab1d"}, col: 3, reg: []string{"2c"}, modified: true}},
		{name: "literal star at start", input: "a *b\n", keys: "/*b<CR>", want: replayState{lines: []string{"a *b"}, col: 2}},
		{name: "literal caret in middle", input: "a^b ab\n", keys: "/a^b<CR>", want: replayState{lines: []string{"a^b ab"}}},
		{name: "nomagic literal dot", input: "abc a.c\n", keys: "/\\Ma.c<CR>", want: replayState{lines: []string{"abc a.c"}, col: 4}},
		{name: "very nomagic", input: "abc a.c\n", keys: "/\\Va.c<CR>", want: replayState{lines: []string{"abc a.c"}, col: 4}},
		{name: "cancel leaves pending state clean", input: "ab\n", keys: "d/a.<Esc>x", want: replayState{lines: []string{"b"}, reg: []string{"a"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestG01SubstituteReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "dot matches any", input: "abc\n", keys: ":s/a.c/X/<CR>", want: replayState{lines: []string{"X"}, modified: true}},
		{name: "global digits", input: "a1 b22 c\n", keys: ":s/\\d\\+/N/g<CR>", want: replayState{lines: []string{"aN bN c"}, modified: true}},
		{name: "first only", input: "a1 b22\n", keys: ":s/\\d\\+/N/<CR>", want: replayState{lines: []string{"aN b22"}, modified: true}},
		{name: "group repeat", input: "xababy\n", keys: ":s/\\(ab\\)\\+/-/<CR>", want: replayState{lines: []string{"x-y"}, modified: true}},
		{name: "very magic alternation", input: "cat dog cow\n", keys: ":s/\\v(cat|cow)/X/g<CR>", want: replayState{lines: []string{"X dog X"}, modified: true}},
		{name: "word boundary", input: "a ab a\n", keys: ":s/\\<lt>a\\>/X/g<CR>", want: replayState{lines: []string{"X ab X"}, modified: true}},
		{name: "start anchor", input: "aaa\n", keys: ":s/^a/X/g<CR>", want: replayState{lines: []string{"Xaa"}, modified: true}},
		{name: "end anchor", input: "aaa\n", keys: ":s/a$/X/g<CR>", want: replayState{lines: []string{"aaX"}, modified: true}},
		{name: "prefix every line", input: "a\nb\n", keys: ":%s/^/> /<CR>", want: replayState{lines: []string{"> a", "> b"}, row: 0, modified: true}},
		{name: "append every line", input: "a\nb\n", keys: ":%s/$/;/<CR>", want: replayState{lines: []string{"a;", "b;"}, modified: true}},
		{name: "zero width makes progress", input: "abc\n", keys: ":s/x*/-/g<CR>", want: replayState{lines: []string{"-a-b-c"}, modified: true}},
		{name: "zero width after match skipped", input: "abc\n", keys: ":s/b*/-/g<CR>", want: replayState{lines: []string{"-a-c"}, modified: true}},
		{name: "zero width unicode", input: "éa\n", keys: ":s/x*/-/g<CR>", want: replayState{lines: []string{"-é-a"}, modified: true}},
		{name: "non greedy", input: "<a><b>\n", keys: ":s/<lt>.\\{-}>/X/<CR>", want: replayState{lines: []string{"X<b>"}, modified: true}},
		{name: "greedy", input: "<a><b>\n", keys: ":s/<lt>.*>/X/<CR>", want: replayState{lines: []string{"X"}, modified: true}},
		{name: "other delimiter escaped", input: "a#b\n", keys: ":s#a\\#b#X#<CR>", want: replayState{lines: []string{"X"}, modified: true}},
		{name: "unicode column and ignore case", input: "Été été\n", keys: ":s/été/X/gi<CR>", want: replayState{lines: []string{"X X"}, modified: true}},
		{name: "case atom", input: "Foo foo\n", keys: ":s/\\cfoo/X/g<CR>", want: replayState{lines: []string{"X X"}, modified: true}},
		{name: "nomagic", input: "a.c abc\n", keys: ":s/\\Ma.c/X/g<CR>", want: replayState{lines: []string{"X abc"}, modified: true}},
		{name: "empty pattern reuses search", input: "a1 b2\n", keys: "/\\d<CR>:s//N/g<CR>", want: replayState{lines: []string{"aN bN"}, col: 1, modified: true}},
		{name: "search after substitute uses its pattern", input: "ab\nb\n", keys: ":s/b/X/<CR>/a<CR>", want: replayState{lines: []string{"aX", "b"}, modified: true}},
		{name: "range limits", input: "a\na\na\n", keys: ":2,3s/a/b/<CR>", want: replayState{lines: []string{"a", "b", "b"}, modified: true}},
		{name: "undo", input: "a1\n", keys: ":s/\\d/N/<CR>u", want: replayState{lines: []string{"a1"}, col: 1}},
		{name: "not found", input: "abc\n", keys: ":s/\\d/N/<CR>", want: replayState{lines: []string{"abc"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestG01SubstituteConfirmReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "patterns with decisions", input: "a1 b22 c333\n", keys: ":s/\\d\\+/N/gc<CR>nyy", want: replayState{lines: []string{"a1 bN cN"}, modified: true}},
		{name: "all", input: "a1 b22\n", keys: ":s/\\d\\+/N/gc<CR>a", want: replayState{lines: []string{"aN bN"}, modified: true}},
		{name: "last", input: "a1 b22 c\n", keys: ":s/\\d\\+/N/gc<CR>l", want: replayState{lines: []string{"aN b22 c"}, modified: true}},
		{name: "quit", input: "a1 b22\n", keys: ":s/\\d\\+/N/gc<CR>q", want: replayState{lines: []string{"a1 b22"}, col: 1}},
		{name: "anchored once", input: "aaa\n", keys: ":s/^a/X/gc<CR>y", want: replayState{lines: []string{"Xaa"}, modified: true}},
		{name: "unicode", input: "ααβ αβ\n", keys: ":s/α\\+β/X/gc<CR>yy", want: replayState{lines: []string{"X X"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestG01GlobalReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "delete matching", input: "a1\nb\nc\n", keys: ":g/\\d/d<CR>", want: replayState{lines: []string{"b", "c"}, reg: []string{"a1"}, linewise: true, modified: true}},
		{name: "alternation", input: "a\nb\nx\n", keys: ":g/b\\|c/d<CR>", want: replayState{lines: []string{"a", "x"}, row: 1, reg: []string{"b"}, linewise: true, modified: true}},
		{name: "invert", input: "a1\nb\nc2\n", keys: ":v/\\d/d<CR>", want: replayState{lines: []string{"a1", "c2"}, row: 1, reg: []string{"b"}, linewise: true, modified: true}},
		{name: "anchors", input: "ab\nba\n", keys: ":g/^a/d<CR>", want: replayState{lines: []string{"ba"}, reg: []string{"ab"}, linewise: true, modified: true}},
		{name: "word", input: "a cat\nconcat\n", keys: ":g/\\<lt>cat\\>/d<CR>", want: replayState{lines: []string{"concat"}, reg: []string{"a cat"}, linewise: true, modified: true}},
		{name: "escaped delimiter in pattern", input: "a/b\nab\n", keys: ":g/a\\/b/d<CR>", want: replayState{lines: []string{"ab"}, reg: []string{"a/b"}, linewise: true, modified: true}},
		{name: "nested substitute", input: "x1\ny\nx2\n", keys: ":g/x\\d/s/\\d/N/<CR>", want: replayState{lines: []string{"xN", "y", "xN"}, row: 0, modified: true}},
		{name: "empty pattern reuses search", input: "a1\nb\n", keys: "/\\d<CR>:g//d<CR>", want: replayState{lines: []string{"b"}, reg: []string{"a1"}, linewise: true, modified: true}},
		{name: "ignore case atom", input: "FOO\nbar\n", keys: ":g/foo\\c/d<CR>", want: replayState{lines: []string{"bar"}, reg: []string{"FOO"}, linewise: true, modified: true}},
		{name: "unicode", input: "été\nete\n", keys: ":g/é\\+/d<CR>", want: replayState{lines: []string{"ete"}, reg: []string{"été"}, linewise: true, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestG01UnsupportedAndInvalidPatterns(t *testing.T) {
	for _, c := range []struct{ name, keys, status string }{
		{"search zs", "/a\\zsb<CR>", `Unsupported pattern construct: \zs`},
		{"search backref", "/\\(a\\)\\1<CR>", `Unsupported pattern construct: \1`},
		{"search lookahead", "/a\\@=<CR>", `Unsupported pattern construct: \@=`},
		{"search multiline", "/a\\nb<CR>", `Unsupported pattern construct: \n`},
		{"search underscore class", "/\\_s<CR>", `Unsupported pattern construct: \_s`},
		{"search percent", "/\\%d65<CR>", `Unsupported pattern construct: \%d`},
		{"search tilde", "/~<CR>", `Unsupported pattern construct: ~`},
		{"search unmatched", "/\\(a<CR>", `E54`},
		{"backward unsupported", "?a\\ze<CR>", `Unsupported pattern construct: \ze`},
		{"substitute zs", ":s/a\\zsb/X/<CR>", `Unsupported pattern construct: \zs`},
		{"confirm zs", ":s/a\\zsb/X/c<CR>", `Unsupported pattern construct: \zs`},
		{"global lookbehind", ":g/a\\@<lt>=b/d<CR>", `Unsupported pattern construct: \@<=`},
		{"invert unsupported", ":v/\\%[ab]/d<CR>", `Unsupported pattern construct: \%[`},
		{"substitute unmatched", ":s/a\\)/X/<CR>", `E55`},
		{"substitute no previous", ":s//X/<CR>", `E35`},
		{"global no previous", ":g//d<CR>", `E35`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newUndoF05Editor(t, "ab\nab\n")
			sendUndoF05(t, e, c.keys)
			if !strings.Contains(e.statusMsg, c.status) {
				t.Errorf("status = %q, want %q", e.statusMsg, c.status)
			}
			assertUndoF05(t, e, []string{"ab", "ab"}, false)
			if e.mode != ModeNormal || e.searchPat != "" {
				t.Errorf("mode=%v searchPat=%q; a rejected pattern must not become the last pattern", e.mode, e.searchPat)
			}
		})
	}
}

func TestG01RejectedSearchAbortsOperator(t *testing.T) {
	e := newUndoF05Editor(t, "abc\n")
	sendUndoF05(t, e, "d/a\\zsb<CR>x")
	assertUndoF05(t, e, []string{"bc"}, true)
}

func TestG01PathologicalPatternFails(t *testing.T) {
	e := newUndoF05Editor(t, strings.Repeat("a", 40)+"\n")
	sendUndoF05(t, e, "/\\(a*\\)*b<CR>")
	if !strings.Contains(e.statusMsg, "too complex") {
		t.Errorf("status = %q", e.statusMsg)
	}
	sendUndoF05(t, e, ":s/\\(a*\\)*b/X/<CR>")
	if !strings.Contains(e.statusMsg, "too complex") || e.lines[0] != strings.Repeat("a", 40) {
		t.Errorf("substitute: status %q line %q", e.statusMsg, e.lines[0])
	}
}
