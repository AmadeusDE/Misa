package vi

import "testing"

func TestF04StructuralReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "nested pair", input: "(a(b)c)z\n", keys: "%", want: replayState{lines: []string{"(a(b)c)z"}, col: 6}},
		{name: "nested square", input: "[a[b]c]z\n", keys: "%", want: replayState{lines: []string{"[a[b]c]z"}, col: 6}},
		{name: "nested brace", input: "{a{b}c}z\n", keys: "%", want: replayState{lines: []string{"{a{b}c}z"}, col: 6}},
		{name: "scan unmatched retains column", input: "ab(cd\n", keys: "l%", want: replayState{lines: []string{"ab(cd"}, col: 1}},
		{name: "no bracket retains column", input: "abcde\n", keys: "ll%", want: replayState{lines: []string{"abcde"}, col: 2}},
		{name: "pair across empty lines", input: "(a\n\n\nb)z\n", keys: "%", want: replayState{lines: []string{"(a", "", "", "b)z"}, row: 3, col: 1}},
		{name: "delete backward pair inclusive", input: "x(abc)z\n", keys: "5ld%", want: replayState{lines: []string{"xz"}, col: 1, reg: []string{"(abc)"}, modified: true}},
		{name: "counted percent delete linewise", input: "a\nb\nc\nd\ne\n", keys: "d50%", want: replayState{lines: []string{"d", "e"}, reg: []string{"a", "b", "c"}, linewise: true, modified: true}},
		{name: "invalid percent motion", input: "a\nb\nc\n", keys: "j101%", want: replayState{lines: []string{"a", "b", "c"}, row: 1}},
		{name: "invalid percent operator", input: "a\nb\nc\n", keys: "d101%", want: replayState{lines: []string{"a", "b", "c"}}},
		{name: "unknown exact mark", input: "abc\ndef\n", keys: "jl`z", want: replayState{lines: []string{"abc", "def"}, row: 1, col: 1}},
		{name: "unknown line mark", input: "abc\ndef\n", keys: "jl'z", want: replayState{lines: []string{"abc", "def"}, row: 1, col: 1}},
		{name: "scan to pair", input: "ab(cd)z\n", keys: "%", want: replayState{lines: []string{"ab(cd)z"}, col: 5}},
		{name: "multiline pair", input: "(a\n(b)\nc)z\n", keys: "%", want: replayState{lines: []string{"(a", "(b)", "c)z"}, row: 2, col: 1}},
		{name: "missing pair", input: "(abc\n", keys: "%", want: replayState{lines: []string{"(abc"}}},
		{name: "delete pair inclusive", input: "(abc)z\n", keys: "d%", want: replayState{lines: []string{"z"}, reg: []string{"(abc)"}, modified: true}},
		{name: "delete pair undo", input: "(abc)z\n", keys: "d%u", want: replayState{lines: []string{"(abc)z"}, reg: []string{"(abc)"}}},
		{name: "percent count", input: "a\nb\nc\nd\ne\n", keys: "50%", want: replayState{lines: []string{"a", "b", "c", "d", "e"}, row: 2}},
		{name: "counted gg", input: "a\nb\nc\nd\n", keys: "3gg", want: replayState{lines: []string{"a", "b", "c", "d"}, row: 2}},
		{name: "counted G", input: "a\nb\nc\nd\n", keys: "2G", want: replayState{lines: []string{"a", "b", "c", "d"}, row: 1}},
		{name: "delete gg", input: "a\nb\nc\nd\n", keys: "3Gdgg", want: replayState{lines: []string{"d"}, reg: []string{"a", "b", "c"}, linewise: true, modified: true}},
		{name: "delete counted G", input: "a\nb\nc\nd\n", keys: "d3G", want: replayState{lines: []string{"d"}, reg: []string{"a", "b", "c"}, linewise: true, modified: true}},
		{name: "yank G no undo", input: "a\nb\nc\n", keys: "yGu", want: replayState{lines: []string{"a", "b", "c"}, reg: []string{"a", "b", "c"}, linewise: true}},
		{name: "exact mark", input: "  abc\ndef\n", keys: "llmaj`a", want: replayState{lines: []string{"  abc", "def"}, col: 2}},
		{name: "line mark first nonblank", input: "  abc\ndef\n", keys: "llllmaj'a", want: replayState{lines: []string{"  abc", "def"}, col: 2}},
		{name: "delete exact mark exclusive", input: "abcdef\n", keys: "llma$d`a", want: replayState{lines: []string{"abf"}, col: 2, reg: []string{"cde"}, modified: true}},
		{name: "delete line mark", input: "a\nb\nc\nd\n", keys: "majjd'a", want: replayState{lines: []string{"d"}, reg: []string{"a", "b", "c"}, linewise: true, modified: true}},
		{name: "yank exact mark exclusive", input: "abcdef\n", keys: "llma$y`a", want: replayState{lines: []string{"abcdef"}, col: 2, reg: []string{"cde"}}},
		{name: "unknown mark operator fails", input: "abc\n", keys: "d`zu", want: replayState{lines: []string{"abc"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
