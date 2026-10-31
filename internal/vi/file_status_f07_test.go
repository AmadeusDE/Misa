package vi

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileF07WriteStatus(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		dos, noeol  bool
	}{
		{"dos", "a\r\nb\r\n", true, false},
		{"noeol", "café", false, true},
		{"dos noeol", "a\r\nb", true, true},
		{"empty", "", false, true},
		{"mixed", "a\r\nb\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, p := f07Editor(t, tc.input)
			f07Keys(t, e, ":w<CR>")
			if got := f07Read(t, p); got != tc.input {
				t.Errorf("saved %q want %q", got, tc.input)
			}
			for _, flag := range []struct {
				s    string
				want bool
			}{{"[dos]", tc.dos}, {"[noeol]", tc.noeol}} {
				if strings.Contains(e.statusMsg, flag.s) != flag.want {
					t.Errorf("status %q: %s presence should be %v", e.statusMsg, flag.s, flag.want)
				}
			}
			bytes := fmt.Sprintf("%d bytes", len(tc.input))
			if !strings.Contains(e.statusMsg, bytes) {
				t.Errorf("status %q missing byte count %q", e.statusMsg, bytes)
			}
		})
	}
}

func TestFileF07NewEmptyWrite(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "new-empty")
	e := New(p, d)
	e.Resize(80, 24)
	f07Keys(t, e, ":w<CR>")
	if got := f07Read(t, p); got != "" {
		t.Errorf("empty new file = %q, want zero bytes", got)
	}
	if e.modified {
		t.Error("new file remains dirty after save")
	}
	if !strings.Contains(e.statusMsg, "0 bytes") {
		t.Errorf("write status = %q, want 0 bytes", e.statusMsg)
	}
}
