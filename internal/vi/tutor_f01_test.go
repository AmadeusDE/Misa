package vi

import (
	tea "charm.land/bubbletea/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTutorLessonsOneTwo(t *testing.T) {
	fixtures := []replayFixture{
		{name: "hjkl", input: "abcd\nefgh\n", keys: "lljkh", want: replayState{lines: []string{"abcd", "efgh"}, col: 1}},
		{name: "delete character", input: "pear\n", keys: "x", want: replayState{lines: []string{"ear"}, reg: []string{"p"}, modified: true}},
		{name: "insert", input: "pear\n", keys: "iX<Esc>", want: replayState{lines: []string{"Xpear"}, modified: true}},
		{name: "append end", input: "pear\n", keys: "A!<Esc>", want: replayState{lines: []string{"pear!"}, col: 4, modified: true}},
		{name: "delete word", input: "red blue green\n", keys: "dw", want: replayState{lines: []string{"blue green"}, reg: []string{"red "}, modified: true}},
		{name: "delete to end", input: "red blue\n", keys: "wd$", want: replayState{lines: []string{"red "}, col: 3, reg: []string{"blue"}, modified: true}},
		{name: "delete word end", input: "red blue\n", keys: "de", want: replayState{lines: []string{" blue"}, reg: []string{"red"}, modified: true}},
		{name: "two words", input: "red blue green gold\n", keys: "2w", want: replayState{lines: []string{"red blue green gold"}, col: 9}},
		{name: "three ends", input: "red blue green gold\n", keys: "3e", want: replayState{lines: []string{"red blue green gold"}, col: 13}},
		{name: "zero", input: "red blue\n", keys: "w0", want: replayState{lines: []string{"red blue"}}},
		{name: "delete two words", input: "red blue green\n", keys: "d2w", want: replayState{lines: []string{"green"}, reg: []string{"red blue "}, modified: true}},
		{name: "delete line", input: "red\nblue\ngreen\n", keys: "dd", want: replayState{lines: []string{"blue", "green"}, reg: []string{"red"}, linewise: true, modified: true}},
		{name: "delete two lines", input: "red\nblue\ngreen\n", keys: "2dd", want: replayState{lines: []string{"green"}, reg: []string{"red", "blue"}, linewise: true, modified: true}},
		{name: "undo", input: "red\n", keys: "xu", want: replayState{lines: []string{"red"}, reg: []string{"r"}}},
		{name: "redo", input: "red\n", keys: "xu<C-r>", want: replayState{lines: []string{"ed"}, reg: []string{"r"}, modified: true}},
		{name: "undo line", input: "red\n", keys: "xU", want: replayState{lines: []string{"red"}, reg: []string{"r"}, modified: true}},
		{name: "empty file motion", input: "", keys: "hjkl0", want: replayState{lines: []string{""}}},
		{name: "empty line delete", input: "\nnext\n", keys: "dd", want: replayState{lines: []string{"next"}, reg: []string{""}, linewise: true, modified: true}},
		{name: "cancel insert", input: "red\n", keys: "i<Esc>", want: replayState{lines: []string{"red"}}},
		{name: "cancel command", input: "red\n", keys: ":q<Esc>", want: replayState{lines: []string{"red"}}},
		{name: "cancel operator", input: "red blue\n", keys: "d<Esc>w", want: replayState{lines: []string{"red blue"}, col: 4}},
		{name: "quit force", input: "red\n", keys: "x:q!<CR>", want: replayState{lines: []string{"ed"}, reg: []string{"r"}, modified: true, quit: true}},
		{name: "quit refuses modified", input: "red\n", keys: "x:q<CR>", want: replayState{lines: []string{"ed"}, reg: []string{"r"}, modified: true}},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) { runReplayFixture(t, f) })
	}
}

func TestTutorEmptyLineForwardSearch(t *testing.T) {
	runReplayFixture(t, replayFixture{name: "search", input: "\nneedle\n", keys: "/needle<CR>", want: replayState{lines: []string{"", "needle"}, row: 1}})
}

func TestTutorStandaloneQuitMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrapper.txt")
	if err := os.WriteFile(path, []byte("abcdef\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	m := &standaloneModel{editor: e}
	var cmd tea.Cmd
	for _, key := range replayKeys(t, "2x:q<CR>") {
		_, cmd = m.Update(key)
	}
	if e.Quit || cmd != nil || !e.modified || !reflect.DeepEqual(e.lines, []string{"cdef"}) {
		t.Fatalf("after 2x:q: Quit=%v cmd=%v modified=%v lines=%q", e.Quit, cmd != nil, e.modified, e.lines)
	}
	for _, key := range replayKeys(t, ":q!<CR>") {
		_, cmd = m.Update(key)
	}
	if !e.Quit || cmd == nil {
		t.Fatalf("Quit=%v cmd=%v, want quit and command", e.Quit, cmd != nil)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("command did not emit tea.QuitMsg")
	}
}

func TestTutorWriteQuit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "write.txt")
	if err := os.WriteFile(path, []byte("red\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	for _, key := range replayKeys(t, "A!<Esc>:wq<CR>") {
		e.Update(key)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "red!\n" || !e.Quit || e.modified {
		t.Errorf("file=%q Quit=%v modified=%v", data, e.Quit, e.modified)
	}
}

// This focused check also exercises linewise edits through public events.
func TestTutorDeleteLineContentAndRegister(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "line.txt")
	if err := os.WriteFile(path, []byte("red\nblue\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	for _, key := range replayKeys(t, "dd") {
		e.Update(key)
	}
	reg, linewise := e.Register()
	if !reflect.DeepEqual(e.lines, []string{"blue"}) || !reflect.DeepEqual(reg, []string{"red"}) || !linewise {
		t.Errorf("lines=%q register=%q linewise=%v", e.lines, reg, linewise)
	}
}
