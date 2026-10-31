package vi

import (
	"strings"
	"testing"
)

func TestCountSafetyReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "put growth limit", input: "ab\n", keys: "x999999999999999p", want: replayState{lines: []string{"b" + strings.Repeat("a", maxContentRepeats)}, col: maxContentRepeats, reg: []string{"a"}, modified: true}},
		// TODO(F06): replayInsert restores the pre-repeat cursor, not the final inserted character.
		{name: "insert growth limit", skip: "TODO(F06): counted insert leaves the cursor at its first repetition", input: "b\n", keys: "999999999999999ia<Esc>", want: replayState{lines: []string{strings.Repeat("a", maxContentRepeats) + "b"}, col: maxContentRepeats - 1, modified: true}},
		{name: "replace past end refuses", input: "ab\n", keys: "3rx", want: replayState{lines: []string{"ab"}}},
		{name: "replace cancellation resets count", input: "abcd\n", keys: "3r<Esc>x", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "linewise put count", input: "a\nb\n", keys: "yy3p", want: replayState{lines: []string{"a", "a", "a", "a", "b"}, row: 1, reg: []string{"a"}, linewise: true, modified: true}},
		{name: "linewise put before count", input: "a\nb\n", keys: "yyj3P", want: replayState{lines: []string{"a", "a", "a", "a", "b"}, row: 1, reg: []string{"a"}, linewise: true, modified: true}},
		{name: "count delete undo redo", input: "abcd\n", keys: "2xu<C-r>", want: replayState{lines: []string{"cd"}, reg: []string{"ab"}, modified: true}},
		// TODO(F06): the repeat closure for p/P still puts only once.
		{name: "dot repeats put count", skip: "TODO(F06): dot does not retain put count", input: "ab\n", keys: "x3p.", want: replayState{lines: []string{"baaaaaa"}, col: 6, reg: []string{"a"}, modified: true}},
		// F04: the operator dispatcher awaits a find target.
		{name: "find operator composition", input: "abxc\n", keys: "dfx", want: replayState{lines: []string{"c"}, reg: []string{"abx"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestCommandStateDoesNotLeak(t *testing.T) {
	for _, keys := range []string{"d<Esc>", "2\"<Esc>", "\"a<Esc>", "\"ad!", "2g!", "2\"!", "3r<Esc>"} {
		t.Run(keys, func(t *testing.T) {
			e := New("", t.TempDir())
			for _, key := range replayKeys(t, keys) {
				e.Update(key)
			}
			if e.count != 0 || e.countGiven || e.pendingReg != 0 || e.hasReg || e.pendingOp != 0 || e.pendingKey != "" || e.opCount != 0 || e.opCountGiven || e.replaceCount != 0 {
				t.Fatalf("pending state after %q: count=%d/%v reg=%q/%v op=%q key=%q opCount=%d/%v replaceCount=%d", keys, e.count, e.countGiven, e.pendingReg, e.hasReg, e.pendingOp, e.pendingKey, e.opCount, e.opCountGiven, e.replaceCount)
			}
		})
	}
}

// Full cursor expectations remain TODO(F06), but content growth is bounded now.
func TestInsertRepeatGrowthBound(t *testing.T) {
	e := New("", t.TempDir())
	for _, key := range replayKeys(t, "999999999999999ia<Esc>") {
		e.Update(key)
	}
	if len(e.lines) != 1 || e.lines[0] != strings.Repeat("a", maxContentRepeats) {
		t.Fatalf("insert repeat produced %d lines, first length %d", len(e.lines), len(e.lines[0]))
	}
}
