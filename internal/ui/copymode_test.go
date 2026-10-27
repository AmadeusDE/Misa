package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestCopyStateMovement(t *testing.T) {
	cs := copyState{
		active: true,
		lines:  []string{"hello world", "second line", "third line"},
		curRow: 0,
		curCol: 0,
		rows:   10,
		cols:   80,
	}

	cs.moveRight(5)
	if cs.curCol != 5 {
		t.Errorf("moveRight(5): col = %d, want 5", cs.curCol)
	}

	cs.moveDown(2)
	if cs.curRow != 2 {
		t.Errorf("moveDown(2): row = %d, want 2", cs.curRow)
	}

	cs.moveDown(100)
	if cs.curRow != 2 {
		t.Errorf("moveDown past end: row = %d, want 2", cs.curRow)
	}

	cs.moveUp(1)
	if cs.curRow != 1 {
		t.Errorf("moveUp(1): row = %d, want 1", cs.curRow)
	}

	cs.moveHome()
	if cs.curCol != 0 {
		t.Errorf("moveHome: col = %d, want 0", cs.curCol)
	}

	cs.moveEnd()
	if cs.curCol != len(cs.lines[1]) {
		t.Errorf("moveEnd: col = %d, want %d", cs.curCol, len(cs.lines[1]))
	}
}

func TestCopyStateSelection(t *testing.T) {
	cs := copyState{
		active: true,
		lines:  []string{"hello world", "second line", "third line"},
		curRow: 0,
		curCol: 0,
		rows:   10,
		cols:   80,
	}

	cs.curCol = 6
	cs.toggleSelect()
	if !cs.selecting {
		t.Fatal("expected selecting after toggle")
	}

	cs.curCol = 11
	text := cs.selectedText()
	if text != "world" {
		t.Errorf("selectedText = %q, want %q", text, "world")
	}

	cs.toggleSelect()
	if cs.selecting {
		t.Fatal("expected not selecting after second toggle")
	}
}

func TestCopyStateMultilineSelection(t *testing.T) {
	cs := copyState{
		active:    true,
		lines:     []string{"hello world", "second line", "third line"},
		curRow:    0,
		curCol:    6,
		selecting: true,
		selRow:    0,
		selCol:    6,
		rows:      10,
		cols:      80,
	}

	cs.curRow = 1
	cs.curCol = 6
	text := cs.selectedText()
	want := "world\nsecond"
	if text != want {
		t.Errorf("multiline selectedText = %q, want %q", text, want)
	}
}

func TestCopyStateWordMovement(t *testing.T) {
	cs := copyState{
		active: true,
		lines:  []string{"hello world test"},
		curRow: 0,
		curCol: 0,
		rows:   10,
		cols:   80,
	}

	cs.moveWordForward()
	if cs.curCol != 6 {
		t.Errorf("after first word forward: col = %d, want 6", cs.curCol)
	}

	cs.moveWordForward()
	if cs.curCol != 12 {
		t.Errorf("after second word forward: col = %d, want 12", cs.curCol)
	}

	cs.moveWordBack()
	if cs.curCol != 6 {
		t.Errorf("after word back: col = %d, want 6", cs.curCol)
	}
}

func TestCopyStateScroll(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line"
	}
	cs := copyState{
		active: true,
		lines:  lines,
		curRow: 49,
		curCol: 0,
		rows:   10,
		cols:   80,
		scroll: 40,
	}

	cs.moveTop()
	if cs.curRow != 0 {
		t.Errorf("moveTop: row = %d, want 0", cs.curRow)
	}
	if cs.scroll != 0 {
		t.Errorf("moveTop: scroll = %d, want 0", cs.scroll)
	}

	cs.moveBottom()
	if cs.curRow != 49 {
		t.Errorf("moveBottom: row = %d, want 49", cs.curRow)
	}
	if cs.scroll != 40 {
		t.Errorf("moveBottom: scroll = %d, want 40", cs.scroll)
	}
}

func TestCopyWordBackAfterVerticalMoveToShorterLine(t *testing.T) {
	cs := copyState{active: true, lines: []string{"x", "abcdefghij"}, curRow: 1, rows: 10, cols: 80}
	cs.moveEnd()
	cs.moveUp(1)
	cs.moveWordBack()
	cs.moveWordForward()
}

func TestCopySelectionIsRuneBased(t *testing.T) {
	cs := copyState{active: true, lines: []string{"é界x"}, rows: 10, cols: 80}
	cs.toggleSelect()
	cs.moveRight(1)
	if got := cs.selectedText(); got != "é" {
		t.Errorf("selected %q, want é", got)
	}
	cs.moveRight(1)
	if got := cs.selectedText(); got != "é界" {
		t.Errorf("selected %q, want é界", got)
	}
	if r := cs.render(lipgloss.Color("#888888")); !strings.Contains(ansi.Strip(r), "é界x") {
		t.Errorf("render mangled text: %q", ansi.Strip(r))
	}
}
