package vi

import "testing"

func TestRepeatOperatorF06(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "delete count override", input: "abcdefghij\n", keys: "2x3.", want: replayState{lines: []string{"fghij"}, reg: []string{"cde"}, modified: true}},
		{name: "put retains count", input: "ab\n", keys: "x3p.", want: replayState{lines: []string{"baaaaaa"}, col: 6, reg: []string{"a"}, modified: true}},
		{name: "operator repeat", input: "one two three four\n", keys: "dw.", want: replayState{lines: []string{"three four"}, reg: []string{"two "}, modified: true}},
		{name: "yank preserves repeat", input: "abcdef\n", keys: "xyll.", want: replayState{lines: []string{"bdef"}, col: 1, reg: []string{"c"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
