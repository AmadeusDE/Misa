package term

import (
	"strings"
	"testing"
)

func cellStr(t *Terminal, row int) string {
	var sb strings.Builder
	for _, c := range t.cells[row] {
		ch := c.Ch
		if ch == 0 {
			ch = ' '
		}
		sb.WriteRune(ch)
	}
	return strings.TrimRight(sb.String(), " ")
}

func TestBasicText(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("hello"))
	if got := cellStr(tm, 0); got != "hello" {
		t.Errorf("row 0 = %q, want %q", got, "hello")
	}
	if tm.curRow != 0 || tm.curCol != 5 {
		t.Errorf("cursor = (%d,%d), want (0,5)", tm.curRow, tm.curCol)
	}
}

func TestNewline(t *testing.T) {
	tm := New(5, 10)
	tm.ProcessOutput([]byte("abc\r\ndef"))
	if got := cellStr(tm, 0); got != "abc" {
		t.Errorf("row 0 = %q, want %q", got, "abc")
	}
	if got := cellStr(tm, 1); got != "def" {
		t.Errorf("row 1 = %q, want %q", got, "def")
	}
}

func TestCarriageReturn(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("abc\rXY"))
	if got := cellStr(tm, 0); got != "XYc" {
		t.Errorf("row 0 = %q, want %q", got, "XYc")
	}
}

func TestCursorPosition(t *testing.T) {
	tm := New(5, 20)
	tm.ProcessOutput([]byte("\x1b[3;5Hx"))
	if tm.curRow != 2 || tm.curCol != 5 {
		t.Errorf("cursor after CUP = (%d,%d), want (2,5)", tm.curRow, tm.curCol)
	}
	if got := tm.cells[2][4].Ch; got != 'x' {
		t.Errorf("cell = %q, want 'x'", got)
	}
}

func TestEraseDisplay(t *testing.T) {
	tm := New(3, 5)
	tm.ProcessOutput([]byte("abcde12345ABCDE"))
	tm.ProcessOutput([]byte("\x1b[2J"))
	for r := 0; r < 3; r++ {
		if got := cellStr(tm, r); got != "" {
			t.Errorf("row %d = %q, want empty", r, got)
		}
	}
}

func TestEraseLine(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("0123456789"))
	tm.ProcessOutput([]byte("\x1b[5G\x1b[K"))
	if got := cellStr(tm, 0); got != "0123" {
		t.Errorf("row 0 = %q, want %q", got, "0123")
	}
}

func TestScrollUp(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("line1\r\nline2\r\nline3\r\nline4"))
	if got := cellStr(tm, 0); got != "line2" {
		t.Errorf("row 0 = %q, want %q", got, "line2")
	}
	if got := cellStr(tm, 2); got != "line4" {
		t.Errorf("row 2 = %q, want %q", got, "line4")
	}
}

func TestScrollback(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("line1\r\nline2\r\nline3\r\nline4\r\nline5"))
	if len(tm.scrollback) != 2 {
		t.Fatalf("scrollback len = %d, want 2", len(tm.scrollback))
	}
	lines := tm.Lines()
	if len(lines) < 5 {
		t.Fatalf("Lines() len = %d, want >= 5", len(lines))
	}
	if !strings.Contains(lines[0], "line1") {
		t.Errorf("Lines()[0] = %q, want to contain %q", lines[0], "line1")
	}
	if !strings.Contains(lines[4], "line5") {
		t.Errorf("Lines()[4] = %q, want to contain %q", lines[4], "line5")
	}
}

func TestSGRColors(t *testing.T) {
	tm := New(3, 20)
	tm.ProcessOutput([]byte("\x1b[31;1mred"))
	c := tm.cells[0][0]
	if !c.HasFG || c.FG != IndexedColor(1) {
		t.Errorf("FG = %v, want indexed 1 (red)", c.FG)
	}
	if !c.Bold {
		t.Error("expected bold")
	}
}

func TestSGRReset(t *testing.T) {
	tm := New(3, 20)
	tm.ProcessOutput([]byte("\x1b[31mA\x1b[0mB"))
	if !tm.cells[0][0].HasFG {
		t.Error("cell A should have FG")
	}
	if tm.cells[0][1].HasFG {
		t.Error("cell B should not have FG after reset")
	}
}

func TestSGR256Color(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("\x1b[38;5;196mX"))
	c := tm.cells[0][0]
	if !c.HasFG {
		t.Error("expected FG set")
	}
	if c.FG != IndexedColor(196) {
		t.Errorf("FG = %v, want indexed 196", c.FG)
	}
}

func TestSGRRGB(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("\x1b[38;2;100;150;200mY"))
	c := tm.cells[0][0]
	if !c.HasFG || c.FG != RGBColor(100, 150, 200) {
		t.Errorf("FG = %v, want RGB(100,150,200)", c.FG)
	}
}

func TestAltScreen(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("main"))
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	if !tm.altScreen {
		t.Error("expected alt screen on")
	}
	if got := cellStr(tm, 0); got != "" {
		t.Errorf("alt screen should be clear, got %q", got)
	}
	tm.ProcessOutput([]byte("alt"))
	tm.ProcessOutput([]byte("\x1b[?1049l"))
	if tm.altScreen {
		t.Error("expected alt screen off")
	}
	if got := cellStr(tm, 0); got != "main" {
		t.Errorf("main screen should be restored, got %q", got)
	}
}

func TestScrollRegion(t *testing.T) {
	tm := New(5, 5)
	tm.ProcessOutput([]byte("\x1b[2;4r"))
	if tm.scrollTop != 1 || tm.scrollBot != 3 {
		t.Errorf("scroll region = (%d,%d), want (1,3)", tm.scrollTop, tm.scrollBot)
	}
}

func TestWrapAround(t *testing.T) {
	tm := New(3, 5)
	tm.ProcessOutput([]byte("12345X"))
	if got := cellStr(tm, 0); got != "12345" {
		t.Errorf("row 0 = %q, want %q", got, "12345")
	}
	if got := cellStr(tm, 1); got != "X" {
		t.Errorf("row 1 = %q, want %q", got, "X")
	}
}

func TestCursorMovement(t *testing.T) {
	tm := New(5, 10)
	tm.ProcessOutput([]byte("\x1b[3;3H"))
	tm.ProcessOutput([]byte("\x1b[2A"))
	if tm.curRow != 0 {
		t.Errorf("after CUU 2: row = %d, want 0", tm.curRow)
	}
	tm.ProcessOutput([]byte("\x1b[3B"))
	if tm.curRow != 3 {
		t.Errorf("after CUD 3: row = %d, want 3", tm.curRow)
	}
	tm.ProcessOutput([]byte("\x1b[5C"))
	if tm.curCol != 7 {
		t.Errorf("after CUF 5: col = %d, want 7", tm.curCol)
	}
	tm.ProcessOutput([]byte("\x1b[3D"))
	if tm.curCol != 4 {
		t.Errorf("after CUB 3: col = %d, want 4", tm.curCol)
	}
}

func TestDeleteChars(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("0123456789"))
	tm.ProcessOutput([]byte("\x1b[4G\x1b[2P"))
	if got := cellStr(tm, 0); got != "01256789" {
		t.Errorf("after DCH: %q, want %q", got, "01256789")
	}
}

func TestInsertChars(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("0123456789"))
	tm.ProcessOutput([]byte("\x1b[4G\x1b[2@"))
	if got := cellStr(tm, 0); got != "012  34567" {
		t.Errorf("after ICH: %q, want %q", got, "012  34567")
	}
}

func TestRender(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("hi\r\nthere"))
	out := tm.Render()
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("render lines = %d, want 2", len(lines))
	}
	if strings.TrimRight(lines[0], " ") != "hi" {
		t.Errorf("render line 0 = %q, want %q", lines[0], "hi")
	}
}

func TestInsertLines(t *testing.T) {
	tm := New(4, 10)
	tm.ProcessOutput([]byte("AAAA\r\nBBBB\r\nCCCC\r\nDDDD"))
	tm.ProcessOutput([]byte("\x1b[2;1H\x1b[1L"))
	if got := cellStr(tm, 0); got != "AAAA" {
		t.Errorf("row 0 = %q, want %q", got, "AAAA")
	}
	if got := cellStr(tm, 1); got != "" {
		t.Errorf("row 1 (inserted) = %q, want empty", got)
	}
	if got := cellStr(tm, 2); got != "BBBB" {
		t.Errorf("row 2 = %q, want %q", got, "BBBB")
	}
}

func TestDeleteLines(t *testing.T) {
	tm := New(4, 10)
	tm.ProcessOutput([]byte("AAAA\r\nBBBB\r\nCCCC\r\nDDDD"))
	tm.ProcessOutput([]byte("\x1b[2;1H\x1b[1M"))
	if got := cellStr(tm, 0); got != "AAAA" {
		t.Errorf("row 0 = %q, want %q", got, "AAAA")
	}
	if got := cellStr(tm, 1); got != "CCCC" {
		t.Errorf("row 1 = %q, want %q", got, "CCCC")
	}
	if got := cellStr(tm, 2); got != "DDDD" {
		t.Errorf("row 2 = %q, want %q", got, "DDDD")
	}
}

func TestBackspace(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("abc\bX"))
	if got := cellStr(tm, 0); got != "abX" {
		t.Errorf("row 0 = %q, want %q", got, "abX")
	}
}

func TestTab(t *testing.T) {
	tm := New(3, 20)
	tm.ProcessOutput([]byte("a\tb"))
	if tm.curCol != 9 {
		t.Errorf("cursor col after tab+b = %d, want 9", tm.curCol)
	}
}

func TestResize(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("hello"))
	tm.Resize(5, 15)
	if tm.Rows != 5 || tm.Cols != 15 {
		t.Errorf("size = %dx%d, want 5x15", tm.Rows, tm.Cols)
	}
	if got := cellStr(tm, 0); got != "hello" {
		t.Errorf("row 0 after resize = %q, want %q", got, "hello")
	}
}
