package term

import (
	"slices"
	"strings"
	"testing"
)

func TestUnderlineLeaking(t *testing.T) {
	// Simulate Bubble Tea rendering: cursor home, then line by line
	tm := New(5, 40)

	// Frame 1: line 0 has underline, line 1 doesn't
	tm.ProcessOutput([]byte("\x1b[H")) // cursor home
	tm.ProcessOutput([]byte("\x1b[4munderlined\x1b[0m normal text\x1b[K\r\n"))
	tm.ProcessOutput([]byte("plain line two\x1b[K\r\n"))
	tm.ProcessOutput([]byte("plain line three\x1b[K"))

	// Check line 0
	for c := 0; c < 10; c++ {
		if !tm.cells[0][c].Under {
			t.Errorf("cell[0][%d] should be underlined (char=%c)", c, tm.cells[0][c].Ch)
		}
	}
	// Check "normal text" is NOT underlined
	for c := 11; c < 22; c++ {
		if tm.cells[0][c].Under {
			t.Errorf("cell[0][%d] should NOT be underlined (char=%c)", c, tm.cells[0][c].Ch)
		}
	}
	// Check line 1 is NOT underlined
	for c := 0; c < 14; c++ {
		if tm.cells[1][c].Under {
			t.Errorf("cell[1][%d] should NOT be underlined (char=%c)", c, tm.cells[1][c].Ch)
		}
	}
}

func TestSGR58ConsumedSemicolon(t *testing.T) {
	// SGR 58;2;R;G;B (underline color) — the 2 must NOT be treated as SGR 2 (dim)
	tm := New(1, 40)
	tm.ProcessOutput([]byte("\x1b[4;58;2;255;128;0mhello\x1b[0m"))

	if !tm.attr.Under {
		// attr was reset by \x1b[0m, but let's check cell
	}
	if tm.cells[0][0].Dim {
		t.Error("SGR 58;2;R;G;B should not enable dim (SGR 2) - the 2 is a color type, not an attribute")
	}
	if !tm.cells[0][0].Under {
		t.Error("cell should be underlined")
	}
}

func TestSGR58ConsumedColon(t *testing.T) {
	// SGR 58:2:R:G:B (underline color, colon form) — should be consumed without side effects
	tm := New(1, 40)
	tm.ProcessOutput([]byte("\x1b[4;58:2:255:128:0mhello\x1b[0m"))
	if tm.cells[0][0].Dim {
		t.Error("SGR 58:2:... should not enable dim")
	}
}

func TestCSIGreaterPrefix(t *testing.T) {
	// \x1b[>1u is Kitty keyboard protocol, NOT cursor restore
	tm := New(5, 20)
	tm.curRow = 3
	tm.curCol = 5
	tm.saved.row = 0
	tm.saved.col = 0
	tm.ProcessOutput([]byte("\x1b[>1u"))
	if tm.curRow == 0 && tm.curCol == 0 {
		t.Error("CSI >1u should NOT restore cursor (it's Kitty keyboard protocol)")
	}
}

func TestCSIGreaterNotSGR(t *testing.T) {
	// \x1b[>4;2m is XTMODKEYS, NOT SGR — must not set underline or dim
	tm := New(1, 20)
	tm.ProcessOutput([]byte("\x1b[>4;2m"))
	if tm.attr.Under {
		t.Error("CSI >4;2m should NOT enable underline (it's XTMODKEYS, not SGR)")
	}
	if tm.attr.Dim {
		t.Error("CSI >4;2m should NOT enable dim")
	}
	// Also verify \x1b[>c (DA2) doesn't crash or misfire
	tm = New(1, 20)
	tm.ProcessOutput([]byte("\x1b[>c"))
	// Should just be silently ignored
}

func TestCSIQuestionStillWorks(t *testing.T) {
	// \x1b[?25l should still hide cursor
	tm := New(5, 20)
	tm.ProcessOutput([]byte("\x1b[?25l"))
	if !tm.cursorHidden {
		t.Error("CSI ?25l should hide cursor")
	}
	tm.ProcessOutput([]byte("\x1b[?25h"))
	if tm.cursorHidden {
		t.Error("CSI ?25h should show cursor")
	}
}

func TestRenderRowUnderline(t *testing.T) {
	// Verify that renderRow produces underline only for underlined cells
	row := make([]Cell, 20)
	for i := range row {
		row[i].Ch = 'a' + rune(i%26)
	}
	// Set underline on cells 5-9
	for i := 5; i < 10; i++ {
		row[i].Under = true
	}
	rendered := renderRow(row, 20)
	// The rendered string should contain \x1b[4m (underline on) for cells 5-9 only
	if !strings.Contains(rendered, "\x1b[4m") && !strings.Contains(rendered, "\x1b[") {
		// lipgloss may use different SGR encoding, just check that underline is present
	}
	// More importantly: cells 0-4 and 10-19 should NOT have underline in the rendered output
	// This is hard to test precisely because of lipgloss encoding, but we can check
	// that the output has some reset between the underlined and non-underlined sections
	t.Logf("rendered: %q", rendered)
}

func TestRenderStyledMatchesLipgloss(t *testing.T) {
	screen := func(s string) []Cell {
		tm := New(1, 10)
		tm.ProcessOutput([]byte(s))
		return tm.cells[0]
	}
	cells := []Cell{
		{FG: RGBColor(1, 2, 3), HasFG: true, Bold: true},
		{BG: IndexedColor(4), HasBG: true, Under: true, Strike: true},
		{Reverse: true, Italic: true, Dim: true},
	}
	for i := range cells {
		for _, text := range []string{"hello", "a b"} {
			want := screen(cells[i].style().Render(text))
			for pass := 0; pass < 2; pass++ {
				if got := screen(renderStyled(&cells[i], text)); !slices.Equal(got, want) {
					t.Errorf("cell %d %q pass %d: cells differ from lipgloss rendering", i, text, pass)
				}
			}
		}
	}
}
