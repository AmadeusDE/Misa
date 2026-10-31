package vi

import (
	tea "charm.land/bubbletea/v2"
	"reflect"
	"testing"
)

func TestPasteNormalIsLiteralUndoable(t *testing.T) {
	e := New("", t.TempDir())
	e.Update(tea.PasteMsg{Content: "iα\n:wq"})
	if !reflect.DeepEqual(e.lines, []string{"iα", ":wq"}) || e.mode != ModeNormal || e.Quit {
		t.Fatalf("paste: lines=%q mode=%v quit=%v", e.lines, e.mode, e.Quit)
	}
	e.Update(keyFromName("u"))
	if !reflect.DeepEqual(e.lines, []string{""}) {
		t.Fatalf("undo: %q", e.lines)
	}
}

func TestPasteInsertAndCommand(t *testing.T) {
	e := New("", t.TempDir())
	e.Update(keyFromName("i"))
	e.Update(tea.PasteMsg{Content: "α\r\nb\t!"})
	if !reflect.DeepEqual(e.lines, []string{"α", "b\t!"}) || e.mode != ModeInsert {
		t.Fatalf("insert paste: %q mode %v", e.lines, e.mode)
	}
	e.Update(keyFromName("esc"))
	e.Update(keyFromName(":"))
	e.Update(tea.PasteMsg{Content: "echo\r\nquit\x1b\nnext"})
	if e.cmdBuf != "echo quit next" || e.mode != ModeCommand || e.Quit {
		t.Fatalf("command paste %q mode %v", e.cmdBuf, e.mode)
	}
}

func TestMultiRuneKeyText(t *testing.T) {
	e := New("", t.TempDir())
	e.Update(keyFromName("i"))
	e.Update(tea.KeyPressMsg{Code: 'a', Text: "a界"})
	if e.lines[0] != "a界" {
		t.Fatalf("got %q", e.lines[0])
	}
}

func TestPasteRepeatAndLiteralControls(t *testing.T) {
	e := New("", t.TempDir())
	e.Update(keyFromName("i"))
	e.Update(tea.PasteMsg{Content: "a\b\x1b\nb"})
	e.Update(keyFromName("esc"))
	e.Update(keyFromName("."))
	if !reflect.DeepEqual(e.lines, []string{"a\b\x1b", "a\b\x1b", "bb"}) {
		t.Fatalf("dot paste = %q", e.lines)
	}
}

func TestPasteSearchAndIgnoredModes(t *testing.T) {
	e := New("", t.TempDir())
	e.Update(keyFromName("/"))
	e.Update(tea.PasteMsg{Content: "a\r\nb\x03"})
	if e.cmdBuf != "a b" || e.mode != ModeSearch {
		t.Fatalf("search paste %q mode %v", e.cmdBuf, e.mode)
	}
	e.Update(keyFromName("esc"))
	e.Update(keyFromName("v"))
	e.Update(tea.PasteMsg{Content: "x"})
	if e.lines[0] != "" || e.visual == visualNone {
		t.Fatalf("visual paste %q visual %v", e.lines, e.visual)
	}
}

func TestPasteReplaceAndBackspace(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"abcd"}
	e.Update(keyFromName("R"))
	e.Update(tea.PasteMsg{Content: "xy"})
	e.Update(keyFromName("backspace"))
	if e.lines[0] != "xbcd" {
		t.Fatalf("replace paste backspace: %q", e.lines[0])
	}
}

func TestNormalPasteDoesNotReplaceDotOrRecordMacro(t *testing.T) {
	e := New("", t.TempDir())
	for _, k := range []string{"i", "x", "esc", "q", "a"} {
		e.Update(keyFromName(k))
	}
	e.Update(tea.PasteMsg{Content: "z"})
	e.Update(keyFromName("q"))
	if reg := e.regs['a']; reg == nil || len(reg.keys) != 0 {
		t.Fatalf("paste recorded in macro: %#v", reg)
	}
	e.Update(keyFromName("."))
	if e.lines[0] != "xzx" {
		t.Fatalf("previous dot lost: %q", e.lines[0])
	}
}
