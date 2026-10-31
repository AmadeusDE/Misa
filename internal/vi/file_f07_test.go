package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func f07Keys(t *testing.T, e *Editor, keys string) {
	t.Helper()
	for _, k := range replayKeys(t, keys) {
		e.Update(k)
	}
}
func f07Read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func f07Editor(t *testing.T, content string) (*Editor, string) {
	t.Helper()
	d := t.TempDir()
	p := filepath.Join(d, "file")
	if err := os.WriteFile(p, []byte(content), 0640); err != nil {
		t.Fatal(err)
	}
	e := New(p, d)
	e.Resize(80, 24)
	return e, p
}

func TestFileF07RoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		lines       []string
	}{
		{"crlf", "one\r\ntwo\r\n", []string{"one", "two"}},
		{"mixed", "one\r\ntwo\n", []string{"one\r", "two"}},
		{"no final newline", "one\ntwo", []string{"one", "two"}},
		{"empty", "", []string{""}},
		{"single empty line", "\n", []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, tc.input)
			if !reflect.DeepEqual(e.lines, tc.lines) {
				t.Errorf("lines = %#v, want %#v", e.lines, tc.lines)
			}
			f07Keys(t, e, ":w<CR>")
			if got := f07Read(t, p); got != tc.input {
				t.Errorf("saved bytes %q, want %q (status %q)", got, tc.input, e.statusMsg)
			}
		})
	}
}

func TestFileF07EditedEOL(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{{"crlf", "one\r\ntwo\r\n", "Xone\r\ntwo\r\n"}, {"mixed", "one\r\ntwo\n", "Xone\r\ntwo\n"}, {"no final newline", "one\ntwo", "Xone\ntwo"}, {"empty existing", "", "X"}} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, tc.input)
			f07Keys(t, e, "iX<Esc>:w<CR>")
			if got := f07Read(t, p); got != tc.want {
				t.Errorf("saved %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFileF07New(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "missing")
	e := New(p, d)
	e.Resize(80, 24)
	if !strings.Contains(e.statusMsg, "[New]") && !strings.Contains(e.View(), "[New]") {
		t.Errorf("missing [New] indication: status %q", e.statusMsg)
	}
	f07Keys(t, e, "ihello<Esc>:w<CR>")
	if got := f07Read(t, p); got != "hello\n" {
		t.Errorf("new file = %q", got)
	}
}

func TestFileF07StandaloneEntry(t *testing.T) {
	e, p := f07Editor(t, "one\r\ntwo\r\n")
	m := &standaloneModel{editor: e}
	for _, key := range replayKeys(t, "x:wq<CR>") {
		m.Update(key)
	}
	if !e.Quit {
		t.Error("standalone :wq did not quit")
	}
	if got := f07Read(t, p); got != "ne\r\ntwo\r\n" {
		t.Errorf("saved = %q", got)
	}
}
