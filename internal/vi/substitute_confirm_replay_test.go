package vi

import (
	"strings"
	"testing"
)

func TestSubstituteConfirmFullReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "confirmed delimiter", input: "cat cat\n", keys: ":s/cat/dog/gc<CR>ny", want: replayState{lines: []string{"cat dog"}, modified: true}},
		{name: "replacement without trailing delimiter", input: "cat\n", keys: ":s/cat/dog<CR>", want: replayState{lines: []string{"dog"}, modified: true}},
		{name: "replacement with trailing delimiter", input: "cat\n", keys: ":s/cat/dog/<CR>", want: replayState{lines: []string{"dog"}, modified: true}},
		{name: "explicit visual marks", input: "cat\ncat\ncat\ncat\n", keys: "jVj<Esc>:'<lt>,'>s/cat/dog/c<CR>yn", want: replayState{lines: []string{"cat", "dog", "cat", "cat"}, row: 1, modified: true}},
		{name: "counted movement selects current line", input: "cat\ncat\ncat\n", keys: "2j:s/cat/dog/c<CR>y", want: replayState{lines: []string{"cat", "cat", "dog"}, row: 2, modified: true}},
		{name: "counted undo spans substitution and prior edit", input: "abc\ncat\ncat\n", keys: "x2j:s/cat/dog/c<CR>y2u", want: replayState{lines: []string{"abc", "cat", "cat"}, reg: []string{"a"}}},
		{name: "unicode case insensitive", input: "Été été\n", keys: ":s/été/X/gic<CR>ny", want: replayState{lines: []string{"Été X"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestSubstituteConfirmPromptAndMatchCursor(t *testing.T) {
	e := newUndoF05Editor(t, "é 猫猫 猫猫\n")
	sendUndoF05(t, e, ":s/猫猫/犬/gc<CR>")
	if e.row != 0 || e.col != 2 {
		t.Errorf("first match cursor = (%d,%d), want (0,2) in runes", e.row, e.col)
	}
	view := e.View()
	// Both runes of the match, not only the cursor rune, are marked in the
	// rendered line; the later match remains unselected.
	selected := cursorStyle.Render("猫") + visualStyle.Render("猫")
	if line := e.renderTextLine(0); !strings.Contains(line, selected+" 猫猫") {
		t.Errorf("current match not highlighted in rendered line: %q", line)
	}
	if !strings.Contains(view, "replace with 犬 (y/n/a/q/l/^E/^Y)?") {
		t.Errorf("confirmation view must advertise choices: %q", view)
	}
	sendUndoF05(t, e, "n")
	if e.row != 0 || e.col != 5 {
		t.Errorf("second match cursor = (%d,%d), want (0,5) in runes", e.row, e.col)
	}
	sendUndoF05(t, e, "q")
}

func TestSubstituteConfirmNoMatchStatus(t *testing.T) {
	e := newUndoF05Editor(t, "cat\n")
	sendUndoF05(t, e, ":s/missing/dog/c<CR>")
	assertUndoF05(t, e, []string{"cat"}, false)
	if e.mode != ModeNormal || !strings.Contains(strings.ToLower(e.statusMsg), "not found") {
		t.Errorf("no-match mode=%v status=%q; want normal and not-found message", e.mode, e.statusMsg)
	}
}

func TestSubstituteConfirmIdenticalPreservesRedo(t *testing.T) {
	e := newUndoF05Editor(t, "cat cat\n")
	sendUndoF05(t, e, "iX<Esc>u:s/cat/cat/gc<CR>a")
	assertUndoF05(t, e, []string{"cat cat"}, false)
	sendUndoF05(t, e, "<C-r>")
	assertUndoF05(t, e, []string{"Xcat cat"}, true)
}

func TestSubstituteConfirmEmptyPatternReusesLastSearch(t *testing.T) {
	e := newUndoF05Editor(t, "abc\n")
	sendUndoF05(t, e, "iZ<Esc>u:s//X/gc<CR>")
	assertUndoF05(t, e, []string{"abc"}, false)
	if e.mode != ModeNormal || !strings.Contains(e.statusMsg, "E35") {
		t.Errorf("no prior pattern: mode=%v status=%q; want normal and E35", e.mode, e.statusMsg)
	}
	sendUndoF05(t, e, "<C-r>")
	assertUndoF05(t, e, []string{"Zabc"}, true)

	e = newUndoF05Editor(t, "abc abc\n")
	sendUndoF05(t, e, "/b<CR>:s//X/gc<CR>yy")
	assertUndoF05(t, e, []string{"aXc aXc"}, true)
}

func TestSubstituteConfirmZeroWidthTerminates(t *testing.T) {
	e := newUndoF05Editor(t, "abc\n")
	sendUndoF05(t, e, ":s/x*/-/gc<CR>a")
	assertUndoF05(t, e, []string{"-a-b-c"}, true)
	if e.mode != ModeNormal {
		t.Errorf("mode = %v, want normal", e.mode)
	}
}
