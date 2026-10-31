package vi

import "testing"

func TestInsertReplaceF06Replay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "counted insert cursor", input: "abc\n", keys: "3iX<Esc>", want: replayState{lines: []string{"XXXabc"}, col: 2, modified: true}},
		{name: "replace restores", input: "abcd\n", keys: "RXY<BS>Z<Esc>", want: replayState{lines: []string{"XZcd"}, col: 1, modified: true}},
		{name: "replace past end", input: "ab\n", keys: "RXYZ<Esc>", want: replayState{lines: []string{"XYZ"}, col: 2, modified: true}},
		{name: "replace backspace boundary", input: "abcd\n", keys: "lR<BS>X<Esc>", want: replayState{lines: []string{"aXcd"}, col: 1, modified: true}},
		{name: "replace newline char count", input: "abcdef\n", keys: "l3r<CR>", want: replayState{lines: []string{"a", "ef"}, row: 1, modified: true}},
		{name: "change word dot", input: "one two\n", keys: "cwX<Esc>w.", want: replayState{lines: []string{"X X"}, col: 2, reg: []string{"two"}, modified: true}},
		{name: "insert stream dot", input: "abc def\n", keys: "iXY<BS>Z<Esc>w.", want: replayState{lines: []string{"XZabc XZdef"}, col: 7, modified: true}},
		{name: "replace dot", input: "abcd efgh\n", keys: "RXY<BS>Z<Esc>w.", want: replayState{lines: []string{"XZcd XZgh"}, col: 6, modified: true}},
		{name: "change end dot", input: "one two\nthree four\n", keys: "c$X<Esc>j.", want: replayState{lines: []string{"X", "X"}, row: 1, reg: []string{"three four"}, modified: true}},
		{name: "append newline dot", input: "abc\n", keys: "A<CR>X<Esc>.", want: replayState{lines: []string{"abc", "X", "X"}, row: 2, modified: true}},
		{name: "insert newline", input: "ab\n", keys: "li<CR><Esc>", want: replayState{lines: []string{"a", "b"}, row: 1, modified: true}},
		{name: "replace restore visible", input: "abcd\n", keys: "RXY<BS><Esc>", want: replayState{lines: []string{"Xbcd"}, modified: true}},
		{name: "replace restore extension", input: "ab\n", keys: "RXYZ<BS><Esc>", want: replayState{lines: []string{"XY"}, col: 1, modified: true}},
		{name: "dot undo atomic", input: "one two\n", keys: "cwX<Esc>w.u", want: replayState{lines: []string{"X two"}, col: 2, reg: []string{"two"}, modified: true}},
		{name: "empty insert preserves repeat", input: "abcd\n", keys: "xi<Esc>.", want: replayState{lines: []string{"cd"}, reg: []string{"b"}, modified: true}},
		{name: "failed replace preserves repeat", input: "abcd\n", keys: "x99rZ.", want: replayState{lines: []string{"cd"}, reg: []string{"b"}, modified: true}},
		{name: "count open dot", input: "a\n", keys: "3oX<Esc>.", want: replayState{lines: []string{"a", "X", "X", "X", "X", "X", "X"}, row: 6, modified: true}},
		{name: "count open above dot", input: "a\n", keys: "3OX<Esc>.", want: replayState{lines: []string{"X", "X", "X", "X", "X", "X", "a"}, modified: true}},
		{name: "change word count override", input: "one two three four\n", keys: "cwX<Esc>w2.", want: replayState{lines: []string{"X X four"}, col: 2, reg: []string{"two three"}, modified: true}},
		{name: "newline stream dot", input: "ab cd\n", keys: "iX<CR>Y<Esc>w.", want: replayState{lines: []string{"X", "Yab X", "Ycd"}, row: 2, modified: true}},
		{name: "replace newline dot", input: "abcdef\n", keys: "2r<CR>l.", want: replayState{lines: []string{"", "c", "f"}, row: 2, modified: true}},
		{name: "explicit register put dot", input: "one two\n", keys: "\"ayw$\"ap.", want: replayState{lines: []string{"one twoone one "}, col: 14, reg: []string{"one "}, modified: true}},
		{name: "counted replace override", input: "abcde fghij\n", keys: "2RX<Esc>w3.", want: replayState{lines: []string{"XXcde XXXij"}, col: 8, modified: true}},
		{name: "replace char dot", input: "abcd efgh\n", keys: "2rXw.", want: replayState{lines: []string{"XXcd XXgh"}, col: 6, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
