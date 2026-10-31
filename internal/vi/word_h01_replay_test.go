package vi

import "testing"

// Original public-key fixtures for Vim's default word/WORD motions. Columns
// are rune indices, not byte offsets or screen cells. No external Vim or user
// configuration is consulted.
func TestH01WordMotionReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "W skips punctuation within WORD", input: "ab,cd  ef!gh\n", keys: "W", want: replayState{lines: []string{"ab,cd  ef!gh"}, col: 7}},
		{name: "w stops at punctuation", input: "ab,cd  ef!gh\n", keys: "w", want: replayState{lines: []string{"ab,cd  ef!gh"}, col: 2}},
		{name: "B skips punctuation within WORD", input: "ab,cd  ef!gh\n", keys: "$B", want: replayState{lines: []string{"ab,cd  ef!gh"}, col: 7}},
		{name: "B from WORD start", input: "ab,cd  ef!gh\n", keys: "WB", want: replayState{lines: []string{"ab,cd  ef!gh"}}},
		{name: "E skips punctuation within WORD", input: "ab,cd  ef!gh\n", keys: "E", want: replayState{lines: []string{"ab,cd  ef!gh"}, col: 4}},
		{name: "E from end advances", input: "ab,cd  ef!gh\n", keys: "EE", want: replayState{lines: []string{"ab,cd  ef!gh"}, col: 11}},
		{name: "ge finds punctuation end", input: "ab,cd ef\n", keys: "Wge", want: replayState{lines: []string{"ab,cd ef"}, col: 4}},
		{name: "ge differs inside WORD", input: "ab,cd ef\n", keys: "4lge", want: replayState{lines: []string{"ab,cd ef"}, col: 2}},
		{name: "gE skips punctuation", input: "ab,cd ef\n", keys: "WgE", want: replayState{lines: []string{"ab,cd ef"}, col: 4}},
		{name: "gE from inside WORD", input: "xy ab,cd ef\n", keys: "W4lgE", want: replayState{lines: []string{"xy ab,cd ef"}, col: 1}},
		{name: "tabs and multibyte WORD", input: "é,猫\tλ!x\tfin\n", keys: "2W", want: replayState{lines: []string{"é,猫\tλ!x\tfin"}, col: 8}},
		{name: "multibyte E", input: "é,猫\tλ!x\n", keys: "E", want: replayState{lines: []string{"é,猫\tλ!x"}, col: 2}},
		{name: "back across tab", input: "é,猫\tλ!x\n", keys: "$B", want: replayState{lines: []string{"é,猫\tλ!x"}, col: 4}},
		{name: "w stops at blank line", input: "one\n\ntwo\n", keys: "w", want: replayState{lines: []string{"one", "", "two"}, row: 1}},
		{name: "b stops at blank line", input: "one\n\ntwo\n", keys: "Gb", want: replayState{lines: []string{"one", "", "two"}, row: 1}},
		{name: "W stops at blank line", input: "one\n\ntwo\n", keys: "W", want: replayState{lines: []string{"one", "", "two"}, row: 1}},
		{name: "count W through blank line", input: "one\n\ntwo\n", keys: "2W", want: replayState{lines: []string{"one", "", "two"}, row: 2}},
		{name: "B stops at blank line", input: "one\n\ntwo\n", keys: "GB", want: replayState{lines: []string{"one", "", "two"}, row: 1}},
		{name: "count gE", input: "aa bb cc dd\n", keys: "$2gE", want: replayState{lines: []string{"aa bb cc dd"}, col: 4}},
		{name: "count ge", input: "a,b c d\n", keys: "$2ge", want: replayState{lines: []string{"a,b c d"}, col: 2}},
		{name: "dW", input: "ab,cd  ef\n", keys: "dW", want: replayState{lines: []string{"ef"}, reg: []string{"ab,cd  "}, modified: true}},
		{name: "dE inclusive", input: "ab,cd ef\n", keys: "dE", want: replayState{lines: []string{" ef"}, reg: []string{"ab,cd"}, modified: true}},
		{name: "dB backward", input: "ab,cd ef\n", keys: "WdB", want: replayState{lines: []string{"ef"}, reg: []string{"ab,cd "}, modified: true}},
		{name: "dge backward", input: "ab,cd ef\n", keys: "Wdge", want: replayState{lines: []string{"ab,cf"}, col: 4, reg: []string{"d e"}, modified: true}},
		{name: "dgE backward", input: "ab,cd ef\n", keys: "WdgE", want: replayState{lines: []string{"ab,cf"}, col: 4, reg: []string{"d e"}, modified: true}},
		{name: "operator counts multiply", input: "a! b? c. d, e; f: g\n", keys: "2d3W", want: replayState{lines: []string{"g"}, reg: []string{"a! b? c. d, e; f: "}, modified: true}},
		{name: "yE characterwise", input: "ab,cd ef\n", keys: "yE", want: replayState{lines: []string{"ab,cd ef"}, reg: []string{"ab,cd"}}},
		{name: "named register and put", input: "ab,cd ef\n", keys: "\"adE\"ap", want: replayState{lines: []string{" ab,cdef"}, col: 5, reg: []string{"ab,cd"}, modified: true}},
		{name: "change WORD preserves separator", input: "ab,cd ef\n", keys: "cWX<Esc>", want: replayState{lines: []string{"X ef"}, reg: []string{"ab,cd"}, modified: true}},
		{name: "delete undo", input: "ab,cd ef\n", keys: "dEu", want: replayState{lines: []string{"ab,cd ef"}, reg: []string{"ab,cd"}}},
		{name: "delete redo", input: "ab,cd ef\n", keys: "dEu<C-r>", want: replayState{lines: []string{" ef"}, reg: []string{"ab,cd"}, modified: true}},
		{name: "dot repeats WORD delete", input: "ab,cd ef!gh zz\n", keys: "dW.", want: replayState{lines: []string{"zz"}, reg: []string{"ef!gh "}, modified: true}},
		{name: "visual E delete", input: "ab,cd ef\n", keys: "vEd", want: replayState{lines: []string{" ef"}, reg: []string{"ab,cd"}, modified: true}},
		{name: "visual backward gE yank", input: "ab,cd ef\n", keys: "WvgEy", want: replayState{lines: []string{"ab,cd ef"}, col: 4, reg: []string{"d e"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
