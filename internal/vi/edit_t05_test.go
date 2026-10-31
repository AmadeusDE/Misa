package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func t05File(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestEditT05CommandsAndIdentity(t *testing.T) {
	for _, cmd := range []string{"e", "ed", "edi", "edit", "edit!"} {
		t.Run(cmd, func(t *testing.T) {
			e, first := f07Editor(t, "first\nline\n")
			other := filepath.Join(filepath.Dir(first), "other file")
			t05File(t, other, "second\r\nlast")
			f07Keys(t, e, "jll:"+cmd+" "+other+"<CR>")
			if e.filename != other || e.alternateFilename != first || !reflect.DeepEqual(e.lines, []string{"second", "last"}) || e.row != 0 || e.col != 0 || e.modified {
				t.Errorf("switch: name=%q alt=%q lines=%q cursor=%d,%d dirty=%v status=%q", e.filename, e.alternateFilename, e.lines, e.row, e.col, e.modified, e.statusMsg)
			}
			f07Keys(t, e, ":w<CR>")
			if got := f07Read(t, other); got != "second\r\nlast" {
				t.Errorf("bytes = %q", got)
			}
		})
	}
}

func TestEditT05GuardReloadAndSwap(t *testing.T) {
	e, first := f07Editor(t, "abc\nsecond\n")
	other := filepath.Join(filepath.Dir(first), "other")
	t05File(t, other, "other\n")
	f07Keys(t, e, "x:e "+other+"<CR>")
	if e.filename != first || !e.modified || !strings.Contains(e.statusMsg, "E37") {
		t.Errorf("guard: %q dirty=%v status=%q", e.filename, e.modified, e.statusMsg)
	}
	f07Keys(t, e, ":e %<CR>")
	if e.filename != first || !e.modified || !strings.Contains(e.statusMsg, "E37") {
		t.Errorf("same-file guard: %q dirty=%v status=%q", e.filename, e.modified, e.statusMsg)
	}
	f07Keys(t, e, ":e!<CR>u<C-r>")
	if !reflect.DeepEqual(e.lines, []string{"abc", "second"}) || e.modified || e.row != 0 || e.col != 0 {
		t.Errorf("reload/undo: %q dirty=%v cursor=%d,%d", e.lines, e.modified, e.row, e.col)
	}
	f07Keys(t, e, ":e "+other+"<CR>:e #<CR>")
	if e.filename != first || e.alternateFilename != other || !reflect.DeepEqual(e.lines, []string{"abc", "second"}) {
		t.Errorf("swap: %q alt=%q lines=%q", e.filename, e.alternateFilename, e.lines)
	}
}

func TestEditT05NewFile(t *testing.T) {
	e, first := f07Editor(t, "old\n")
	newPath := filepath.Join(filepath.Dir(first), "new file")
	f07Keys(t, e, ":e "+newPath+"<CR>")
	if e.filename != newPath || !reflect.DeepEqual(e.lines, []string{""}) || !strings.Contains(e.statusMsg, "[New]") {
		t.Errorf("new: %q %q status=%q", e.filename, e.lines, e.statusMsg)
	}
	f07Keys(t, e, "ihello<Esc>:w<CR>")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new file: %v", err)
		return
	}
	if got := f07Read(t, newPath); got != "hello\n" {
		t.Errorf("new bytes = %q", got)
	}
}
