package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestT05StandaloneEditEntry(t *testing.T) {
	e, first := f07Editor(t, "alpha\n")
	second := filepath.Join(filepath.Dir(first), "second")
	if err := os.WriteFile(second, []byte("beta\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := &standaloneModel{editor: e}
	send := func(keys string) tea.Cmd {
		t.Helper()
		var cmd tea.Cmd
		for _, key := range replayKeys(t, keys) {
			_, cmd = m.Update(key)
		}
		return cmd
	}
	send("iX<Esc>:e second<CR>")
	if !strings.Contains(e.statusMsg, "E37") || !reflect.DeepEqual(e.lines, []string{"Xalpha"}) {
		t.Errorf("modified guard: status=%q lines=%q", e.statusMsg, e.lines)
	}
	send(":edit! second<CR>")
	if e.filename != second || !reflect.DeepEqual(e.lines, []string{"beta"}) || e.modified {
		t.Fatalf("force switch: filename=%q lines=%q modified=%v status=%q", e.filename, e.lines, e.modified, e.statusMsg)
	}
	send("u")
	if !reflect.DeepEqual(e.lines, []string{"beta"}) {
		t.Errorf("undo crossed switch: %q", e.lines)
	}
	send("iY<Esc>:e!<CR>")
	if !reflect.DeepEqual(e.lines, []string{"beta"}) || e.modified {
		t.Fatalf("force reload: lines=%q modified=%v status=%q", e.lines, e.modified, e.statusMsg)
	}
	send("u")
	if !reflect.DeepEqual(e.lines, []string{"beta"}) {
		t.Errorf("undo crossed reload: %q", e.lines)
	}
	cmd := send(":q<CR>")
	if !e.Quit || cmd == nil {
		t.Fatal("clean :q did not quit standalone")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("standalone :q did not send tea.QuitMsg")
	}
	if got := f07Read(t, first); got != "alpha\n" {
		t.Errorf("discarded file changed: %q", got)
	}
}

func TestT05StandaloneEditCleanSwitch(t *testing.T) {
	e, first := f07Editor(t, "first\n")
	second := filepath.Join(filepath.Dir(first), "next")
	if err := os.WriteFile(second, []byte("next line\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := &standaloneModel{editor: e}
	for _, key := range replayKeys(t, ":e next<CR>") {
		m.Update(key)
	}
	if e.filename != second || !reflect.DeepEqual(e.lines, []string{"next line"}) {
		t.Fatalf("switch: filename=%q lines=%q status=%q", e.filename, e.lines, e.statusMsg)
	}
}
