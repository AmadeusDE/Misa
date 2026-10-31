package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestT02CleanCurrentDivergence(t *testing.T) {
	for _, command := range []string{"1w!", "1w >>"} {
		t.Run(command, func(t *testing.T) {
			e, p := f07Editor(t, "one\ntwo\n")
			saved := e.savedRevision
			f07Keys(t, e, ":"+command+"<CR>")
			if e.modified || e.savedRevision != saved {
				t.Fatal("partial/append write changed original clean flag or saved revision")
			}
			if e.file.info == nil || e.file.info.Size() != int64(len(f07Read(t, p))) {
				t.Fatal("write did not refresh the external modification snapshot")
			}
			f07Keys(t, e, ":w<CR>")
			if got := f07Read(t, p); got != "one\ntwo\n" {
				t.Errorf("full write after partial/append = %q: %s", got, e.statusMsg)
			}
		})
	}
}

func TestT02Status(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\n")
	f07Keys(t, e, ":2w excerpt<CR>")
	if e.statusMsg != `"excerpt" [New] 1 lines, 4 bytes written` {
		t.Errorf("status = %q", e.statusMsg)
	}
	f07Keys(t, e, ":2w >> excerpt<CR>")
	if e.statusMsg != `"excerpt" 1 lines, 4 bytes appended` {
		t.Errorf("append status = %q", e.statusMsg)
	}
	if got := f07Read(t, filepath.Join(filepath.Dir(p), "excerpt")); got != "two\ntwo\n" {
		t.Errorf("append bytes = %q", got)
	}
}

func TestT02RenamedExistingNeedsForce(t *testing.T) {
	e, p := f07Editor(t, "one\n")
	other := filepath.Join(filepath.Dir(p), "other")
	if err := os.WriteFile(other, []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f07Keys(t, e, ":file other<CR>")
	before := e.file
	f07Keys(t, e, ":w<CR>")
	if !strings.Contains(e.statusMsg, "E13") || e.file != before || f07Read(t, other) != "old\n" {
		t.Fatalf("rename write protection: %s", e.statusMsg)
	}
	f07Keys(t, e, ":w!<CR>")
	if f07Read(t, other) != "one\n" || e.modified || e.file.notEdited {
		t.Fatal("forced renamed write did not save")
	}
}

func TestT02WriteUsesEditorDirectory(t *testing.T) {
	e, p := f07Editor(t, "one\n")
	dir := t.TempDir()
	e = New(p, dir)
	f07Keys(t, e, ":w out<CR>")
	if got := f07Read(t, filepath.Join(dir, "out")); got != "one\n" {
		t.Errorf("copy = %q", got)
	}
	if e.filename != p {
		t.Errorf("copy renamed to %q", e.filename)
	}
}

func TestT02WritesPreserveRegisterAndDot(t *testing.T) {
	e, _ := f07Editor(t, "abc\ndef\n")
	f07Keys(t, e, "x:1w excerpt<CR>:saveas saved<CR>.")
	reg, linewise := e.Register()
	if !reflect.DeepEqual(e.lines, []string{"c", "def"}) || !reflect.DeepEqual(reg, []string{"b"}) || linewise {
		t.Errorf("dot after writes: lines=%q register=%q linewise=%v", e.lines, reg, linewise)
	}
}
