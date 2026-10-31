package vi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// F09 fixtures target Vim 9.1 with 'nowrap', 'tabstop' 8 and 'sidescroll' 0.
func newDisplayEditor(t *testing.T, input string, w, h int) *Editor {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "f09.txt")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(w, h)
	return e
}

func (e *Editor) typeKeys(t *testing.T, keys string) {
	t.Helper()
	for _, k := range replayKeys(t, keys) {
		e.Update(k)
	}
}

var trickyLines = []string{
	"",
	"plain ascii text",
	"\t\tindented\ttabs\t",
	"日本語のテキストは幅が広い",
	"ééé combining ́́",
	"emoji 👨‍👩‍👧 🇯🇵 😀 end",
	"ctl \x01\x02\x1b\x7f \u0085 end",
	"bad \xff\xfe\xc3 bytes \xe6\x97",
	strings.Repeat("0123456789", 30),
	strings.Repeat("世", 100),
	strings.Repeat("\t", 40) + "x",
}

func checkBounds(t *testing.T, e *Editor, w, h int, ctx string) {
	t.Helper()
	view := e.View()
	lines := strings.Split(view, "\n")
	if len(lines) > h {
		t.Fatalf("%s: %d lines in a %d-row pane:\n%s", ctx, len(lines), h, ansi.Strip(view))
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got > w {
			t.Fatalf("%s: line %d is %d cells in a %d-wide pane: %q", ctx, i, got, w, ansi.Strip(l))
		}
	}
	if w >= 1 && h >= 3 {
		start, n := e.rowLayout(e.row).cursorCells(e.col, e.insertLike())
		if start < e.leftCol || start+n > e.leftCol+max(w, n) {
			t.Fatalf("%s: cursor cells %d+%d outside window left=%d width=%d", ctx, start, n, e.leftCol, w)
		}
	}
}

func TestViewStaysInsidePane(t *testing.T) {
	sequences := []string{"", "$", "j$", "jjj$", "ll", "vl", "Vj", "A", "R", "jjjjjjjjjj$", ":s/e/x/gc<CR>", "/e<CR>", "G$"}
	for _, w := range []int{1, 2, 3, 4, 5, 8, 17, 40} {
		for _, h := range []int{1, 2, 3, 4, 6} {
			for _, keys := range sequences {
				e := newDisplayEditor(t, strings.Join(trickyLines, "\n")+"\n", w, h)
				ctx := fmt.Sprintf("w=%d h=%d keys=%q", w, h, keys)
				e.typeKeys(t, keys)
				checkBounds(t, e, w, h, ctx)
				for range 12 {
					e.typeKeys(t, "l")
					checkBounds(t, e, w, h, ctx+" +l")
				}
				e.typeKeys(t, "<Esc>$")
				checkBounds(t, e, w, h, ctx+" $")
			}
		}
	}
}

func TestViewLongStatusAndPromptFit(t *testing.T) {
	for _, w := range []int{1, 3, 10, 30} {
		e := newDisplayEditor(t, "x\n", w, 6)
		e.filename = strings.Repeat("長いファイル名", 10)
		e.modified = true
		e.statusMsg = "E486: Pattern not found: " + strings.Repeat("世", 40) + "\nsecond"
		checkBounds(t, e, w, 6, "status")
		e.statusMsg = ""
		e.typeKeys(t, ":"+strings.Repeat("世x", 30))
		checkBounds(t, e, w, 6, "command")
		if w >= 3 {
			last := ansi.Strip(e.View())
			last = last[strings.LastIndex(last, "\n")+1:]
			if !strings.HasSuffix(last, "x") {
				t.Errorf("w=%d: command line should keep the typed tail, got %q", w, last)
			}
		}
	}
}

func TestResizeKeepsCursorVisible(t *testing.T) {
	e := newDisplayEditor(t, strings.Repeat("0123456789", 20)+"\n"+strings.Repeat("line\n", 50), 80, 24)
	e.typeKeys(t, "$")
	e.typeKeys(t, "20j")
	for _, size := range []tea.WindowSizeMsg{{Width: 20, Height: 5}, {Width: 3, Height: 3}, {Width: 1, Height: 1}, {Width: 100, Height: 40}, {Width: 7, Height: 2}} {
		e.Update(size)
		checkBounds(t, e, size.Width, size.Height, fmt.Sprintf("%+v", size))
		if size.Height >= 3 && (e.row < e.topLine || e.row >= e.topLine+e.height) {
			t.Errorf("%+v: row %d outside rows %d..%d", size, e.row, e.topLine, e.topLine+e.height-1)
		}
	}
}

func TestHorizontalScrollFollowsCursor(t *testing.T) {
	e := newDisplayEditor(t, strings.Repeat("0123456789", 5)+"\nshort\n", 20, 6)
	e.typeKeys(t, "$")
	if got := ansi.Strip(strings.Split(e.View(), "\n")[0]); !strings.HasSuffix(got, "89") || ansi.StringWidth(got) > 20 {
		t.Errorf("end of line not visible: %q", got)
	}
	left := e.leftCol
	if left == 0 {
		t.Fatal("window did not scroll")
	}
	e.typeKeys(t, "h")
	if e.leftCol != left {
		t.Errorf("moving inside the window scrolled: %d -> %d", left, e.leftCol)
	}
	e.typeKeys(t, "j")
	e.View()
	if e.leftCol != 0 {
		t.Errorf("short line should scroll back, leftCol=%d", e.leftCol)
	}
	e.typeKeys(t, "k$0")
	if e.col != 0 || e.leftCol != 0 {
		t.Errorf("0 leaves col=%d leftCol=%d", e.col, e.leftCol)
	}
}

func TestPreferredColumnIsDisplayColumn(t *testing.T) {
	e := newDisplayEditor(t, "abcdefghijk\n\tz\n世世世x\nabcdefg\n", 80, 24)
	e.typeKeys(t, "7l")
	e.typeKeys(t, "j")
	if e.row != 1 || e.col != 0 {
		t.Fatalf("on tab: row=%d col=%d", e.row, e.col)
	}
	if got := ansi.Strip(strings.Split(e.View(), "\n")[22]); !strings.Contains(got, "2,1-8") {
		t.Errorf("ruler should show the tab's last cell: %q", got)
	}
	e.typeKeys(t, "k")
	if e.row != 0 || e.col != 7 {
		t.Fatalf("back up: row=%d col=%d", e.row, e.col)
	}
	e.typeKeys(t, "l")
	e.typeKeys(t, "j")
	if e.row != 1 || e.col != 1 {
		t.Fatalf("past the tab: row=%d col=%d", e.row, e.col)
	}
	e.typeKeys(t, "j")
	if e.row != 2 || e.col != 3 { // column 8 is past the line: last rune
		t.Fatalf("clamped to the last rune: row=%d col=%d", e.row, e.col)
	}
}

func TestPreferredColumnThroughWideChars(t *testing.T) {
	e := newDisplayEditor(t, "abcdefg\n世世世x\nabcdefg\n", 80, 24)
	e.typeKeys(t, "5l") // display column 5
	e.typeKeys(t, "j")
	if e.row != 1 || e.col != 2 {
		t.Fatalf("column 5 is the third wide char: row=%d col=%d", e.row, e.col)
	}
	e.typeKeys(t, "j")
	if e.row != 2 || e.col != 5 {
		t.Fatalf("preferred column survived the wide line: row=%d col=%d", e.row, e.col)
	}
}

func TestCursorCellsInModes(t *testing.T) {
	e := newDisplayEditor(t, "a\tb\n", 80, 24)
	e.typeKeys(t, "l")
	if got := e.cursorVCol(); got != 8 {
		t.Errorf("normal mode on tab: vcol %d, want 8", got)
	}
	e.typeKeys(t, "i")
	if got := e.cursorVCol(); got != 2 {
		t.Errorf("insert before tab: vcol %d, want 2", got)
	}
	e.typeKeys(t, "<Esc>")
}

func TestVisualSelectionAlignsWithCells(t *testing.T) {
	e := newDisplayEditor(t, "a世\tb\n", 80, 24)
	e.typeKeys(t, "vll")
	line := strings.Split(e.View(), "\n")[0]
	if got := ansi.Strip(line); got != "a世     b" {
		t.Errorf("selected line %q", got)
	}
	layout := e.rowLayout(0)
	if start, n := layout.cursorCells(e.col, false); start != 7 || n != 1 {
		t.Errorf("visual cursor on the tab's last cell, got %d+%d", start, n)
	}
}

func TestSubstituteConfirmHighlightOnWideLine(t *testing.T) {
	e := newDisplayEditor(t, "世界 foo 世界 foo\n", 12, 6)
	e.typeKeys(t, ":s/foo/bar/gc<CR>")
	e.typeKeys(t, "y")
	checkBounds(t, e, 12, 6, "confirm")
	if e.mode != ModeConfirm {
		t.Fatalf("mode %v", e.mode)
	}
	start, _ := e.rowLayout(e.row).cursorCells(e.col, false)
	if start < e.leftCol || start >= e.leftCol+12 {
		t.Errorf("second match not scrolled into view: start=%d left=%d", start, e.leftCol)
	}
}

func TestInsertTypingScrollsAndKeepsWidth(t *testing.T) {
	e := newDisplayEditor(t, "\n", 10, 5)
	e.typeKeys(t, "i")
	for _, r := range "世界abc\tdef日本語" {
		e.Update(keyFromName(string(r)))
		checkBounds(t, e, 10, 5, fmt.Sprintf("after %q", r))
	}
}
