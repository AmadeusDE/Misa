package vi

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTabCtrlIParity(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{keyFromName("tab"), keyFromName("ctrl+i")} {
		e := New("", t.TempDir())
		e.Update(keyFromName("i"))
		e.Update(key)
		e.Update(keyFromName("esc"))
		e.Update(keyFromName("."))
		if e.lines[0] != "\t\t" {
			t.Fatalf("%v inserted and repeated %q", key, e.lines[0])
		}
	}
	for _, key := range []tea.KeyPressMsg{keyFromName("tab"), keyFromName("ctrl+i")} {
		e := New("", t.TempDir())
		e.lines = []string{"abc"}
		e.jumpList = []pos{{0, 0}, {0, 2}}
		e.jumpIdx = 0
		e.Update(key)
		if e.col != 2 {
			t.Fatalf("%v jump col=%d", key, e.col)
		}
	}
}

func TestPasteSurroundingTypingUndo(t *testing.T) {
	e := New("", t.TempDir())
	e.Update(keyFromName("i"))
	e.Update(keyFromName("a"))
	e.Update(tea.PasteMsg{Content: "b\nc"})
	e.Update(keyFromName("d"))
	e.Update(keyFromName("esc"))
	e.Update(keyFromName("u"))
	if !reflect.DeepEqual(e.lines, []string{""}) {
		t.Fatalf("undo = %q", e.lines)
	}
	e.Update(keyFromName("ctrl+r"))
	if !reflect.DeepEqual(e.lines, []string{"ab", "cd"}) {
		t.Fatalf("redo = %q", e.lines)
	}
}
