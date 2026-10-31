package vi

import (
	"os"
	"path/filepath"
	"testing"
)

// Original public-key fixtures: columns are rune indices. No user vimrc or
// external editor is needed. Word searches must become the last search, not
// merely move the cursor, so subsequent n/N and operators are exercised too.
func TestH02WordSearchReplay(t *testing.T) {
	for _, f := range []replayFixture{
		{name: "forward whole word", input: "cat scatter cat\n", keys: "*", want: replayState{lines: []string{"cat scatter cat"}, col: 12}},
		{name: "backward wraps whole word", input: "cat scatter cat\n", keys: "#", want: replayState{lines: []string{"cat scatter cat"}, col: 12}},
		{name: "inside word searches entire word", input: "cat cat cat\n", keys: "l*", want: replayState{lines: []string{"cat cat cat"}, col: 4}},
		{name: "whitespace finds next word on line", input: "  cat cat\n", keys: "*", want: replayState{lines: []string{"  cat cat"}, col: 6}},
		{name: "forward count", input: "cat cat cat cat\n", keys: "2*", want: replayState{lines: []string{"cat cat cat cat"}, col: 8}},
		{name: "backward count", input: "cat cat cat cat\n", keys: "2#", want: replayState{lines: []string{"cat cat cat cat"}, col: 8}},
		{name: "forward repeat wraps", input: "cat cat cat\n", keys: "*2n", want: replayState{lines: []string{"cat cat cat"}}},
		{name: "N temporarily reverses forward", input: "cat cat cat\n", keys: "*Nn", want: replayState{lines: []string{"cat cat cat"}, col: 4}},
		{name: "N temporarily reverses backward", input: "cat cat cat\n", keys: "#Nn", want: replayState{lines: []string{"cat cat cat"}, col: 8}},
		{name: "punctuation searched literally", input: ". + .\n", keys: "*", want: replayState{lines: []string{". + ."}, col: 4}},
		{name: "punctuation prefers following keyword", input: ". x . x\n", keys: "*", want: replayState{lines: []string{". x . x"}, col: 6}},
		{name: "regex star searched literally", input: "* + *\n", keys: "*", want: replayState{lines: []string{"* + *"}, col: 4}},
		{name: "unicode word and boundaries", input: "猫 猫咪 猫\n", keys: "*", want: replayState{lines: []string{"猫 猫咪 猫"}, col: 5}},
		{name: "across lines", input: "cat\ndog\ncat\n", keys: "*", want: replayState{lines: []string{"cat", "dog", "cat"}, row: 2}},
		{name: "g star matches inside words", input: "cat scatter cat\n", keys: "g*", want: replayState{lines: []string{"cat scatter cat"}, col: 5}},
		{name: "g hash matches inside words backward", input: "cat scatter cat\n", keys: "$g#", want: replayState{lines: []string{"cat scatter cat"}, col: 5}},
		{name: "g star count", input: "cat scatter cat\n", keys: "2g*", want: replayState{lines: []string{"cat scatter cat"}, col: 12}},
		{name: "operator g star", input: "cat scatter cat\n", keys: "dg*", want: replayState{lines: []string{"catter cat"}, reg: []string{"cat s"}, modified: true}},
		{name: "visual g star", input: "cat scatter cat\n", keys: "vg*d", want: replayState{lines: []string{"atter cat"}, reg: []string{"cat sc"}, modified: true}},
		{name: "operator g hash backward", input: "cat scatter cat\n", keys: "$dg#", want: replayState{lines: []string{"cat st"}, col: 5, reg: []string{"catter ca"}, modified: true}},
		{name: "visual g hash backward", input: "cat scatter cat\n", keys: "$vg#d", want: replayState{lines: []string{"cat s"}, col: 4, reg: []string{"catter cat"}, modified: true}},
		{name: "underscore and digits are keyword", input: "a_1 xa_1 a_1\n", keys: "*", want: replayState{lines: []string{"a_1 xa_1 a_1"}, col: 9}},
		{name: "operator direct star", input: "cat x cat y cat\n", keys: "d*", want: replayState{lines: []string{"cat y cat"}, reg: []string{"cat x "}, modified: true}},
		{name: "operator multiplied count", input: "cat cat cat cat cat\n", keys: "2d2*", want: replayState{lines: []string{"cat"}, reg: []string{"cat cat cat cat "}, modified: true}},
		{name: "operator repeat after star", input: "cat x cat y cat\n", keys: "*dn", want: replayState{lines: []string{"cat x cat"}, col: 6, reg: []string{"cat y "}, modified: true}},
		{name: "dot repeats change backward word search", input: "cat x cat y cat z cat\n", keys: "6wc#X<Esc>0ww.", want: replayState{lines: []string{"Xcat y Xcat"}, reg: []string{"cat x "}, modified: true}},
		{name: "change backward word search and undo", input: "cat x cat\n", keys: "wwc#X<Esc>u", want: replayState{lines: []string{"cat x cat"}, reg: []string{"cat x "}}},
		{name: "visual search selection", input: "cat x cat\n", keys: "v*d", want: replayState{lines: []string{"at"}, reg: []string{"cat x c"}, modified: true}},
		{name: "ignorecase", input: "cat CAT cat\n", keys: ":set ignorecase<CR>*", want: replayState{lines: []string{"cat CAT cat"}, col: 4}},
		{name: "empty line leaves cursor", input: "\ncat\n", keys: "*", want: replayState{lines: []string{"", "cat"}}},
	} {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestH02WordSearchRestoresHighlights(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("cat scatter cat\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	send := func(s string) {
		for _, key := range replayKeys(t, s) {
			e.Update(key)
		}
	}
	send(":set hlsearch<CR>/scatter<CR>:nohlsearch<CR>0*")
	marks, _ := e.highlightLineMarks(0, e.rowLayout(0))
	if marks == nil || !marks(0) || marks(5) || !marks(12) {
		t.Fatal("word search should restore highlights for whole-word matches, not the prior pattern or a substring")
	}
}

func TestH02WordSearchPatternAndDirection(t *testing.T) {
	for _, tc := range []struct {
		keys, pattern string
		dir           int
	}{
		{"*", `\<cat\>`, 1},
		{"#", `\<cat\>`, -1},
		{"g*", "cat", 1},
		{"g#", "cat", -1},
		{"*N", `\<cat\>`, 1},
		{"#N", `\<cat\>`, -1},
	} {
		t.Run(tc.keys, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sample")
			if err := os.WriteFile(path, []byte("cat scatter cat\n"), 0600); err != nil {
				t.Fatal(err)
			}
			e := New(path, dir)
			e.Resize(80, 24)
			for _, key := range replayKeys(t, tc.keys) {
				e.Update(key)
			}
			if e.searchPat != tc.pattern || e.searchDir != tc.dir {
				t.Errorf("last search = %q direction %d; want %q direction %d", e.searchPat, e.searchDir, tc.pattern, tc.dir)
			}
		})
	}
}

func TestH02NoStringPreservesPreviousSearch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("cat cat\n   \n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	for _, key := range replayKeys(t, "/cat<CR>j*") {
		e.Update(key)
	}
	if e.row != 1 || e.searchPat != "cat" || e.searchDir != 1 || e.statusMsg == "" {
		t.Errorf("failed word search: cursor (%d,%d), last search %q direction %d, status %q", e.row, e.col, e.searchPat, e.searchDir, e.statusMsg)
	}
}
