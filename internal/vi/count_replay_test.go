package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCountReplay(t *testing.T) {
	fixtures := []replayFixture{
		{name: "operator count multiplication", input: "one two three four five six seven\n", keys: "2d3w", want: replayState{lines: []string{"seven"}, reg: []string{"one two three four five six "}, modified: true}},
		{name: "count gg", input: "a\nb\nc\nd\ne\nf\n", keys: "5gg", want: replayState{lines: []string{"a", "b", "c", "d", "e", "f"}, row: 4}},
		{name: "count find", input: "axaxax\n", keys: "2fx", want: replayState{lines: []string{"axaxax"}, col: 3}},
		{name: "count replace", input: "abcdef\n", keys: "3rx", want: replayState{lines: []string{"xxxdef"}, col: 2, modified: true}},
		{name: "count put after", input: "abcd\n", keys: "x3p", want: replayState{lines: []string{"baaacd"}, col: 3, reg: []string{"a"}, modified: true}},
		{name: "count put before", input: "abcd\n", keys: "x3P", want: replayState{lines: []string{"aaabcd"}, col: 2, reg: []string{"a"}, modified: true}},
		{name: "named counted yank put", input: "a\nb\nc\n", keys: "\"a2yy\"ap", want: replayState{lines: []string{"a", "a", "b", "b", "c"}, row: 1, reg: []string{"a", "b"}, linewise: true, modified: true}},
		{name: "count D", input: "abc\ndef\nghi\n", keys: "2D", want: replayState{lines: []string{"", "ghi"}, reg: []string{"abc\ndef"}, modified: true}},
		{name: "count C", input: "abc\ndef\nghi\n", keys: "2C", want: replayState{lines: []string{"", "ghi"}, mode: ModeInsert, reg: []string{"abc\ndef"}, modified: true}},
		{name: "count S", input: "abc\ndef\nghi\n", keys: "2S", want: replayState{lines: []string{"", "ghi"}, mode: ModeInsert, reg: []string{"abc", "def"}, linewise: true, modified: true}},
		{name: "count s", input: "abc\ndef\n", keys: "2s", want: replayState{lines: []string{"c", "def"}, mode: ModeInsert, reg: []string{"ab"}, modified: true}},
		{name: "count J", input: "abc\ndef\nghi\njkl\n", keys: "3J", want: replayState{lines: []string{"abc def ghi", "jkl"}, col: 7, modified: true}},
		{name: "count x", input: "abcdef\n", keys: "3x", want: replayState{lines: []string{"def"}, reg: []string{"abc"}, modified: true}},
		{name: "count X", input: "abcdef\n", keys: "$3X", want: replayState{lines: []string{"abf"}, col: 2, reg: []string{"cde"}, modified: true}},
		{name: "cancel operator then edit", input: "abcd\n", keys: "d<Esc>x", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "invalid counted prefix resets", input: "abcd\n", keys: "2g!x", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "huge count multiplication", input: "one two three\n", keys: "999999999999999999999d999999999999999999999w", want: replayState{lines: []string{""}, reg: []string{"one two three"}, modified: true}},
		{name: "huge count delete lines full state", input: "one\ntwo\nthree\n", keys: "99999999999dd", want: replayState{lines: []string{""}, reg: []string{"one", "two", "three"}, linewise: true, modified: true}},
		{name: "cancel count register", input: "abcd\n", keys: "2\"<Esc>x", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "cancel named register", input: "abcd\n", keys: "\"a<Esc>x", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "invalid operator resets", input: "abcd\n", keys: "d!x", want: replayState{lines: []string{"bcd"}, reg: []string{"a"}, modified: true}},
		{name: "huge count", input: "abcd\n", keys: "999999999999999999999999x", want: replayState{lines: []string{""}, reg: []string{"abcd"}, modified: true}},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

// Content-focused checks complement the full-state replay fixtures.
func TestHugeCountDeleteLinesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	for _, key := range replayKeys(t, "99999999999dd") {
		e.Update(key)
	}
	reg, linewise := e.Register()
	if !reflect.DeepEqual(e.lines, []string{""}) || !reflect.DeepEqual(reg, []string{"one", "two", "three"}) || !linewise {
		t.Errorf("lines=%q register=%q linewise=%v", e.lines, reg, linewise)
	}
}

func TestHugeCountMultiplicationDeleteLinesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	for _, key := range replayKeys(t, "999999999999999999d999999999999999999d") {
		e.Update(key)
	}
	reg, linewise := e.Register()
	if !reflect.DeepEqual(e.lines, []string{""}) || !reflect.DeepEqual(reg, []string{"one", "two", "three"}) || !linewise {
		t.Errorf("lines=%q register=%q linewise=%v", e.lines, reg, linewise)
	}
}
