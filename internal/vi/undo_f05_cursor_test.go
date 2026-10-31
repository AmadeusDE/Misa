package vi

import "testing"

func TestUndoF05RepeatedTextCursor(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "undo repeated deletion", input: "aaa\n", keys: "xu", want: replayState{lines: []string{"aaa"}, reg: []string{"a"}}},
		{name: "redo repeated deletion", input: "aaa\n", keys: "xu$<C-r>", want: replayState{lines: []string{"aa"}, reg: []string{"a"}, modified: true}},
		{name: "undo repeated insertion", input: "aaa\n", keys: "ia<Esc>u", want: replayState{lines: []string{"aaa"}}},
		{name: "reverted intermediate mutation", input: "abc\n", keys: "liX<BS><BS>az<Esc>u", want: replayState{lines: []string{"abc"}}},
		{name: "undo backward operator", input: "aaaa\n", keys: "$d0u", want: replayState{lines: []string{"aaaa"}, reg: []string{"aaa"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}
