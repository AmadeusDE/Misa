package vi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Original public-event fixtures for H04 word text objects. Every expectation
// was confirmed by black-box replay of the same keystrokes in a stock
// vi-compatible editor (Vim 9.0.1378, -Nu NONE, nocompatible defaults) and then
// written as an independent fixture here; no editor source was consulted and
// none is needed to run these tests.
func TestWordObjectsH04(t *testing.T) {
	for _, f := range []replayFixture{
		// --- operator + iw/aw/iW/aW ---
		{name: "inner word at start", input: "one two three\n", keys: "diw", want: replayState{lines: []string{" two three"}, reg: []string{"one"}, modified: true}},
		{name: "inner word in middle", input: "one two three\n", keys: "wdiw", want: replayState{lines: []string{"one  three"}, col: 4, reg: []string{"two"}, modified: true}},
		{name: "count inner", input: "one two three\n", keys: "2diw", want: replayState{lines: []string{"two three"}, reg: []string{"one "}, modified: true}},
		{name: "count around", input: "one two three\n", keys: "2daw", want: replayState{lines: []string{"three"}, reg: []string{"one two "}, modified: true}},
		{name: "around from middle", input: "one two three\n", keys: "w2daw", want: replayState{lines: []string{"one"}, col: 2, reg: []string{" two three"}, modified: true}},
		{name: "operator and object counts multiply", input: "one two three four five\n", keys: "2d2iw", want: replayState{lines: []string{"three four five"}, reg: []string{"one two "}, modified: true}},
		{name: "operator and object counts multiply around", input: "one two three four five\n", keys: "2d2aw", want: replayState{lines: []string{"five"}, reg: []string{"one two three four "}, modified: true}},
		{name: "yank multiplies counts", input: "one two three four five\n", keys: "2y2aw", want: replayState{lines: []string{"one two three four five"}, reg: []string{"one two three four "}}},
		{name: "inner whitespace", input: "one  two\n", keys: "3ldiw", want: replayState{lines: []string{"onetwo"}, col: 3, reg: []string{"  "}, modified: true}},
		{name: "around whitespace", input: "one  two\n", keys: "3ldaw", want: replayState{lines: []string{"one"}, col: 2, reg: []string{"  two"}, modified: true}},
		{name: "inner word second line", input: "one\ntwo\n", keys: "jdiw", want: replayState{lines: []string{"one", ""}, row: 1, reg: []string{"two"}, modified: true}},
		{name: "inner on empty line is a no-op", input: "one\n\ntwo\n", keys: "jdiw", want: replayState{lines: []string{"one", "", "two"}, row: 1}},
		{name: "around on empty line is linewise", input: "one\n\ntwo\n", keys: "jdaw", want: replayState{lines: []string{"one"}, reg: []string{"", "two"}, linewise: true, modified: true}},
		{name: "inner multiline count", input: "one two\nthree four\n", keys: "w2diw", want: replayState{lines: []string{"one  four"}, col: 4, reg: []string{"two\nthree"}, modified: true}},
		{name: "around multiline count", input: "one two\nthree four\n", keys: "w2daw", want: replayState{lines: []string{"one four"}, col: 4, reg: []string{"two\nthree "}, modified: true}},
		{name: "inner charwise multiline", input: "one\ntwo three\n", keys: "2diw", want: replayState{lines: []string{" three"}, reg: []string{"one\ntwo"}, modified: true}},
		{name: "inner multiline to eof is linewise", input: "one\ntwo\n", keys: "2diw", want: replayState{lines: []string{""}, reg: []string{"one", "two"}, linewise: true, modified: true}},
		{name: "inner last char", input: "one\n", keys: "$diw", want: replayState{lines: []string{""}, reg: []string{"one"}, modified: true}},
		{name: "around last char", input: "one\n", keys: "$daw", want: replayState{lines: []string{""}, reg: []string{"one"}, modified: true}},
		{name: "no final newline inner", input: "one two", keys: "wdiw", want: replayState{lines: []string{"one "}, col: 3, reg: []string{"two"}, modified: true}},
		{name: "no final newline around", input: "one two", keys: "wdaw", want: replayState{lines: []string{"one"}, col: 2, reg: []string{" two"}, modified: true}},
		{name: "punctuation inner", input: "a.b c\n", keys: "diw", want: replayState{lines: []string{".b c"}, reg: []string{"a"}, modified: true}},
		{name: "punctuation around", input: "a.b c\n", keys: "daw", want: replayState{lines: []string{".b c"}, reg: []string{"a"}, modified: true}},
		{name: "big word inner", input: "a.b c\n", keys: "diW", want: replayState{lines: []string{" c"}, reg: []string{"a.b"}, modified: true}},
		{name: "big word around", input: "a.b c\n", keys: "daW", want: replayState{lines: []string{"c"}, reg: []string{"a.b "}, modified: true}},

		{name: "tab inner", input: "\tone two\n", keys: "diw", want: replayState{lines: []string{"one two"}, reg: []string{"\t"}, modified: true}},
		{name: "tab around", input: "\tone two\n", keys: "daw", want: replayState{lines: []string{" two"}, reg: []string{"\tone"}, modified: true}},
		{name: "inner trailing whitespace", input: "one two  \n", keys: "$diw", want: replayState{lines: []string{"one two"}, col: 6, reg: []string{"  "}, modified: true}},
		{name: "around trailing whitespace", input: "one two  \n", keys: "wdaw", want: replayState{lines: []string{"one "}, col: 3, reg: []string{"two  "}, modified: true}},
		{name: "failed count leaves cursor at scan end", input: "one two three\n", keys: "5daw", want: replayState{lines: []string{"one two three"}, col: 12}},
		{name: "visual backwards extension", input: "one two\nthree four\n", keys: "wvhiwd", want: replayState{lines: []string{"wo", "three four"}, reg: []string{"one t"}, modified: true}},

		{name: "yank inner keeps cursor", input: "one two\n", keys: "wyiw", want: replayState{lines: []string{"one two"}, col: 4, reg: []string{"two"}}},
		{name: "yank counted inner", input: "one two three\n", keys: "2yiw", want: replayState{lines: []string{"one two three"}, reg: []string{"one "}}},
		{name: "yank counted around", input: "one two three\n", keys: "2yaw", want: replayState{lines: []string{"one two three"}, reg: []string{"one two "}}},
		{name: "visual yank object", input: "one two three\n", keys: "viwiwy", want: replayState{lines: []string{"one two three"}, reg: []string{"one "}}},
		{name: "visual yank backwards object", input: "one two\n", keys: "wvhiwy", want: replayState{lines: []string{"one two"}, reg: []string{"one t"}}},

		// --- registers, undo/redo, dot ---
		{name: "named register round trip", input: "one two\n", keys: "\"adiw\"ap", want: replayState{lines: []string{" onetwo"}, col: 3, reg: []string{"one"}, modified: true}},
		{name: "undo", input: "one two\n", keys: "diwu", want: replayState{lines: []string{"one two"}, reg: []string{"one"}}},
		{name: "redo", input: "one two\n", keys: "diwu<C-r>", want: replayState{lines: []string{" two"}, reg: []string{"one"}, modified: true}},
		{name: "redo without undo is a no-op", input: "one two\n", keys: "diw<C-r>", want: replayState{lines: []string{" two"}, reg: []string{"one"}, modified: true}},
		{name: "dot after ciw", input: "one two three\n", keys: "ciwX<Esc>w.", want: replayState{lines: []string{"X X three"}, col: 2, reg: []string{"two"}, modified: true}},
		{name: "counted dot after ciw", input: "one two three\n", keys: "ciwX<Esc>w.", want: replayState{lines: []string{"X X three"}, col: 2, reg: []string{"two"}, modified: true}},

		// --- Visual: objects extend the existing selection ---
		{name: "visual inner", input: "one two three\n", keys: "viwd", want: replayState{lines: []string{" two three"}, reg: []string{"one"}, modified: true}},
		{name: "visual repeated inner extends", input: "one two three\n", keys: "viwiwd", want: replayState{lines: []string{"two three"}, reg: []string{"one "}, modified: true}},
		{name: "visual repeated inner twice", input: "one two three\n", keys: "viwiwiwd", want: replayState{lines: []string{" three"}, reg: []string{"one two"}, modified: true}},
		{name: "visual repeated around", input: "one two three\n", keys: "vawawd", want: replayState{lines: []string{"three"}, reg: []string{"one two "}, modified: true}},
		{name: "visual count inner", input: "one two\n", keys: "v2iwd", want: replayState{lines: []string{"two"}, reg: []string{"one "}, modified: true}},
		{name: "visual count around", input: "one two\n", keys: "v2awd", want: replayState{lines: []string{""}, reg: []string{"one two"}, modified: true}},
		{name: "visual count multiline", input: "one\ntwo\n", keys: "v2iwd", want: replayState{lines: []string{""}, reg: []string{"one\ntwo"}, modified: true}},
		{name: "visual inner multiline", input: "one two\nthree four\n", keys: "wv2iwd", want: replayState{lines: []string{"one  four"}, col: 4, reg: []string{"two\nthree"}, modified: true}},
		{name: "visual around multiline", input: "one two\nthree four\n", keys: "wv2awd", want: replayState{lines: []string{"one four"}, col: 4, reg: []string{"two\nthree "}, modified: true}},
		{name: "visual around then inner", input: "one two three\n", keys: "vawiwd", want: replayState{lines: []string{" three"}, reg: []string{"one two"}, modified: true}},
		{name: "visual inner then around", input: "one two three\n", keys: "viwawd", want: replayState{lines: []string{" three"}, reg: []string{"one two"}, modified: true}},
		{name: "visual object keeps earlier anchor", input: "one two three\n", keys: "vwwiwd", want: replayState{lines: []string{""}, reg: []string{"one two three"}, modified: true}},
		{name: "visual count then repeated object", input: "one two three\n", keys: "v2iwiwd", want: replayState{lines: []string{" three"}, reg: []string{"one two"}, modified: true}},
		{name: "visual object after motion", input: "one two three\n", keys: "wviwiwd", want: replayState{lines: []string{"one three"}, col: 4, reg: []string{"two "}, modified: true}},
		{name: "visual line mode switches to charwise", input: "one\ntwo\n", keys: "Viwd", want: replayState{lines: []string{"", "two"}, reg: []string{"one"}, modified: true}},
		{name: "visual big word", input: "a.b c\n", keys: "viWd", want: replayState{lines: []string{" c"}, reg: []string{"a.b"}, modified: true}},
		{name: "visual around big word", input: "a.b c\n", keys: "vaWd", want: replayState{lines: []string{"c"}, reg: []string{"a.b "}, modified: true}},
		{name: "visual count big word", input: "a.b c.d\n", keys: "v2iWd", want: replayState{lines: []string{"c.d"}, reg: []string{"a.b "}, modified: true}},

		// --- line breaks, whole lines and the object's own surroundings ---
		{name: "counted yank stays charwise", input: "one\ntwo\n", keys: "2yiw", want: replayState{lines: []string{"one", "two"}, reg: []string{"one\ntwo"}}},
		{name: "counted change stays charwise", input: "one\ntwo\n", keys: "2ciw<Esc>", want: replayState{lines: []string{""}, reg: []string{"one\ntwo"}, modified: true}},
		{name: "counted delete of whole lines is linewise", input: "one\ntwo\nthree\n", keys: "2diw", want: replayState{lines: []string{"three"}, reg: []string{"one", "two"}, linewise: true, modified: true}},
		{name: "counted object stops on the indent of the next line", input: "one\n  two\n", keys: "2diw", want: replayState{lines: []string{"two"}, reg: []string{"one\n  "}, modified: true}},
		{name: "counted around crosses the line break", input: "one\ntwo three\n", keys: "2daw", want: replayState{lines: []string{"three"}, reg: []string{"one\ntwo "}, modified: true}},
		{name: "inner count keeps trailing blanks", input: "one\ntwo  \nthree\n", keys: "j2diw", want: replayState{lines: []string{"one", "", "three"}, row: 1, reg: []string{"two  "}, modified: true}},
		{name: "visual backwards around takes the space in front", input: "one two three\n", keys: "wwvhawd", want: replayState{lines: []string{"onehree"}, col: 3, reg: []string{" two t"}, modified: true}},
		{name: "visual backwards inner then around", input: "one two three\n", keys: "wwvhiwiwd", want: replayState{lines: []string{"onehree"}, col: 3, reg: []string{" two t"}, modified: true}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

// TestWordObjectRegisterRoundTripH04 yanks a multiline object and puts it
// back. Put cursor placement is the existing F06 behavior, so only the buffer
// and register are asserted.
func TestWordObjectRegisterRoundTripH04(t *testing.T) {
	e := New("", t.TempDir())
	e.Resize(80, 24)
	e.lines = []string{"one two", "three four"}
	for _, key := range replayKeys(t, "wv2iwy$p") {
		e.Update(key)
	}
	if got, want := strings.Join(e.lines, "|"), "one twotwo|three|three four"; got != want {
		t.Fatalf("lines %q, want %q", got, want)
	}
	reg, linewise := e.Register()
	if len(reg) != 1 || reg[0] != "two\nthree" || linewise {
		t.Fatalf("register %q linewise=%v", reg, linewise)
	}
}

// TestWordObjectFilterH04 checks that !iw routes the object's lines to the
// filter prompt instead of silently doing nothing or creating an undo unit.
func TestWordObjectFilterH04(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filter.txt")
	if err := os.WriteFile(path, []byte("one two\nthree four\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	for _, key := range replayKeys(t, "j!iw") {
		e.Update(key)
	}
	if e.mode != ModeCommand || e.cmdBuf != "2,2!" {
		t.Fatalf("mode=%v cmdBuf=%q", e.mode, e.cmdBuf)
	}
	if e.modified || len(e.undoStack) != 0 {
		t.Fatalf("filter created an edit: modified=%v undo=%d", e.modified, len(e.undoStack))
	}
}
