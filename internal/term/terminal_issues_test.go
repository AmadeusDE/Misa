package term

import (
	"strings"
	"testing"
)

// --- Tests for confirmed issues (Phase 0) ---
// These tests encode known bugs. They should FAIL until the corresponding
// fix is implemented. Each test documents what the correct behavior should be.

func TestApplicationCursorModeNotTracked(t *testing.T) {
	// Confirmed issue: private mode 1 (DECCKM) is acknowledged in a comment
	// but not actually tracked as state. keyToBytes always sends normal-mode
	// cursor sequences regardless of the application's cursor mode setting.
	tm := New(5, 10)

	// Send DECCKM set (application cursor keys)
	tm.ProcessOutput([]byte("\x1b[?1h"))

	// The terminal should expose this state so input forwarding can use it.
	// Currently this is a no-op in privateMode().
	if !tm.appCursorKeys {
		t.Error("application cursor mode (DECCKM) is not tracked; private mode 1 is a no-op")
	}

	tm.ProcessOutput([]byte("\x1b[?1l"))
	if tm.appCursorKeys {
		t.Error("application cursor mode should be cleared after DECCKM reset")
	}
}

func TestBracketedPasteNotTracked(t *testing.T) {
	// Confirmed issue: private mode 2004 is explicitly ignored with a comment
	// "bracketed paste — ignore". Applications requesting bracketed paste
	// never receive paste brackets, breaking paste handling.
	tm := New(5, 10)

	tm.ProcessOutput([]byte("\x1b[?2004h"))
	if !tm.bracketedPaste {
		t.Error("bracketed paste mode is not tracked; mode 2004 is ignored")
	}

	tm.ProcessOutput([]byte("\x1b[?2004l"))
	if tm.bracketedPaste {
		t.Error("bracketed paste mode should be cleared after mode 2004 reset")
	}
}

func TestCursorVisibilityNotTracked(t *testing.T) {
	// Confirmed issue: private mode 25 (DECTCEM) is acknowledged in a comment
	// "cursor visibility — we don't render a cursor" but not tracked.
	tm := New(5, 10)

	tm.ProcessOutput([]byte("\x1b[?25l"))
	if !tm.cursorHidden {
		t.Error("cursor visibility is not tracked; mode 25 is a no-op")
	}

	tm.ProcessOutput([]byte("\x1b[?25h"))
	if tm.cursorHidden {
		t.Error("cursor should be visible after DECTCEM set")
	}
}

func TestANSIColorPreservesIdentity(t *testing.T) {
	// Verify that ANSI colors preserve their palette index identity
	// instead of being converted to hardcoded RGB values.
	tm := New(3, 10)
	tm.ProcessOutput([]byte("\x1b[31mR"))
	c := tm.cells[0][0]

	if !c.HasFG {
		t.Fatal("expected FG set")
	}
	if c.FG.Kind != ColorIndexed {
		t.Errorf("ANSI red: FG kind = %v, want ColorIndexed", c.FG.Kind)
	}
	if c.FG.Index != 1 {
		t.Errorf("ANSI red: FG index = %d, want 1", c.FG.Index)
	}

	// Bright colors (90-97) should map to indices 8-15
	tm2 := New(3, 10)
	tm2.ProcessOutput([]byte("\x1b[91mB"))
	c2 := tm2.cells[0][0]
	if !c2.HasFG || c2.FG.Kind != ColorIndexed || c2.FG.Index != 9 {
		t.Errorf("bright red: FG = %v, want indexed 9", c2.FG)
	}

	// 256-color should preserve index
	tm3 := New(3, 10)
	tm3.ProcessOutput([]byte("\x1b[38;5;196mX"))
	c3 := tm3.cells[0][0]
	if !c3.HasFG || c3.FG.Kind != ColorIndexed || c3.FG.Index != 196 {
		t.Errorf("256-color: FG = %v, want indexed 196", c3.FG)
	}

	// RGB should remain RGB
	tm4 := New(3, 10)
	tm4.ProcessOutput([]byte("\x1b[38;2;100;150;200mY"))
	c4 := tm4.cells[0][0]
	if !c4.HasFG || c4.FG.Kind != ColorRGB {
		t.Errorf("RGB: FG kind = %v, want ColorRGB", c4.FG.Kind)
	}
	if c4.FG.R != 100 || c4.FG.G != 150 || c4.FG.B != 200 {
		t.Errorf("RGB values = (%d,%d,%d), want (100,150,200)", c4.FG.R, c4.FG.G, c4.FG.B)
	}
}

func TestRendererUsesIndexedColors(t *testing.T) {
	tm := New(1, 5)
	tm.ProcessOutput([]byte("\x1b[31mhello"))
	out := tm.Render()
	if strings.Contains(out, "#aa0000") {
		t.Errorf("render output contains hardcoded hex #aa0000; should use palette-relative color")
	}
}

func TestRenderWithCursorHidden(t *testing.T) {
	tm := New(2, 5)
	tm.ProcessOutput([]byte("hello"))
	tm.ProcessOutput([]byte("\x1b[1;1H"))

	plain := tm.Render()

	tm.ProcessOutput([]byte("\x1b[?25l"))
	hidden := tm.RenderWithCursor(true)
	if hidden != plain {
		t.Error("RenderWithCursor with hidden cursor should match Render")
	}

	tm.ProcessOutput([]byte("\x1b[?25h"))
	noBlink := tm.RenderWithCursor(false)
	if noBlink != plain {
		t.Error("RenderWithCursor(false) should match Render")
	}
}

func TestCursorAccessors(t *testing.T) {
	tm := New(5, 10)
	tm.ProcessOutput([]byte("abc"))
	if tm.CursorRow() != 0 {
		t.Errorf("CursorRow = %d, want 0", tm.CursorRow())
	}
	if tm.CursorCol() != 3 {
		t.Errorf("CursorCol = %d, want 3", tm.CursorCol())
	}

	tm.ProcessOutput([]byte("\r\nxyz"))
	if tm.CursorRow() != 1 {
		t.Errorf("CursorRow = %d, want 1", tm.CursorRow())
	}
	if tm.CursorCol() != 3 {
		t.Errorf("CursorCol = %d, want 3", tm.CursorCol())
	}
}

func TestPlainLines(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("\x1b[31mhello\x1b[0m"))
	plain := tm.PlainScreen()
	if len(plain) != 2 {
		t.Fatalf("PlainScreen len = %d, want 2", len(plain))
	}
	if plain[0] != "hello" {
		t.Errorf("PlainScreen[0] = %q, want %q", plain[0], "hello")
	}
	if strings.Contains(plain[0], "\x1b") {
		t.Error("PlainScreen should not contain ANSI escapes")
	}
}

func TestScrollbackNotAppendedInAltScreen(t *testing.T) {
	// Verify that alternate-screen content does not pollute scrollback.
	tm := New(3, 10)
	tm.ProcessOutput([]byte("main1\r\nmain2\r\nmain3\r\n"))
	sbBefore := len(tm.scrollback)

	tm.ProcessOutput([]byte("\x1b[?1049h"))
	// Write several screenfuls in alt screen
	for i := 0; i < 10; i++ {
		tm.ProcessOutput([]byte("alt\r\n"))
	}
	sbAfterAlt := len(tm.scrollback)

	if sbAfterAlt != sbBefore {
		t.Errorf("scrollback grew by %d during alt screen; should not append alt-screen content",
			sbAfterAlt-sbBefore)
	}

	tm.ProcessOutput([]byte("\x1b[?1049l"))
	// Main screen should be restored
	if got := cellStr(tm, 0); got != "main2" {
		t.Logf("note: main screen row 0 = %q after alt-screen exit", got)
	}
}
