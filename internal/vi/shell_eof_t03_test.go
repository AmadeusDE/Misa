package vi

import (
	"os"
	"reflect"
	"testing"
)

func TestT03ReadAtEOFUndoRedoBytes(t *testing.T) {
	t03Shell(t)
	for _, tc := range []struct{ name, input, command, want string }{
		{"noeol result", "first\n", "printf 'last'", "first\nlast"},
		{"blank lines", "first\n", "printf '\\n\\nlast\\n'", "first\n\n\nlast\n"},
		{"dos", "first\r\n", "printf 'last\\r\\n'", "first\r\nlast\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, tc.input)
			t03Keys(t, e, ":r !"+tc.command+"<CR>:w<CR>")
			if b, err := os.ReadFile(p); err != nil || string(b) != tc.want {
				t.Errorf("read bytes %q err %v want %q", b, err, tc.want)
			}
			t03Keys(t, e, "u:w<CR>")
			if b, err := os.ReadFile(p); err != nil || string(b) != tc.input {
				t.Errorf("undo bytes %q err %v want %q", b, err, tc.input)
			}
			t03Keys(t, e, "<C-r>:w<CR>")
			if b, err := os.ReadFile(p); err != nil || string(b) != tc.want {
				t.Errorf("redo bytes %q err %v want %q", b, err, tc.want)
			}
		})
	}
}

func TestT03EmptyReadDoesNotEdit(t *testing.T) {
	t03Shell(t)
	e, _ := f07Editor(t, "one\ntwo\n")
	t03Keys(t, e, ":r !:<CR>")
	if !reflect.DeepEqual(e.lines, []string{"one", "two"}) || e.modified {
		t.Errorf("empty read lines=%q modified=%v", e.lines, e.modified)
	}
}
