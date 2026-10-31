package term

import (
	"bufio"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Regression tests for real-world programs (agy, cmatrix, ncurses apps).

func TestKittyKeyboardPopIsNotCursorRestore(t *testing.T) {
	// agy exits with CSI < 1 u (kitty keyboard pop). Treating it as CSI u
	// (restore cursor) jumped to the top and the following ED wiped the banner.
	tm := New(6, 20)
	tm.ProcessOutput([]byte("\x1b7line0\r\nline1\r\nline2\r\nline3\x1b[<1u\r\n\x1b[J"))
	if got := cellStr(tm, 2); got != "line2" {
		t.Errorf("row 2 = %q, want line2 (CSI < u must not move the cursor)", got)
	}
	if tm.curRow != 4 {
		t.Errorf("cursor row = %d, want 4", tm.curRow)
	}
}

func TestEraseUsesCurrentBackground(t *testing.T) {
	// TERM=xterm-256color advertises bce; ncurses (cmatrix -b) relies on it.
	tm := New(3, 4)
	tm.ProcessOutput([]byte("\x1b[40m\x1b[H\x1b[2J"))
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			cell := tm.cells[r][c]
			if !cell.HasBG || cell.BG != IndexedColor(0) {
				t.Fatalf("cell (%d,%d) bg = %+v, want black after ED with bg set", r, c, cell.BG)
			}
		}
	}
}

func TestEraseOpsUseCurrentBackground(t *testing.T) {
	seqs := map[string]string{
		"EL":  "\x1b[K",
		"ECH": "\x1b[4X",
		"DCH": "\x1b[2P",
		"ICH": "\x1b[2@",
		"IL":  "\x1b[L",
		"DL":  "\x1b[M",
		"SU":  "\x1b[S",
		"SD":  "\x1b[T",
	}
	for name, seq := range seqs {
		tm := New(2, 4)
		tm.ProcessOutput([]byte("\x1b[44m\x1b[H" + seq))
		found := false
		for _, row := range tm.cells {
			for _, c := range row {
				if c.HasBG && c.BG == IndexedColor(4) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: erased cells did not take the current background", name)
		}
	}
}

func TestScrollFillsWithCurrentBackground(t *testing.T) {
	tm := New(2, 3)
	tm.ProcessOutput([]byte("\x1b[41m\r\n\r\n"))
	if c := tm.cells[1][0]; !c.HasBG || c.BG != IndexedColor(1) {
		t.Errorf("scrolled-in row bg = %+v, want red", c.BG)
	}
}

func TestRepeatPrecedingCharacter(t *testing.T) {
	tm := New(1, 10)
	tm.ProcessOutput([]byte("ab\x1b[3b"))
	if got := cellStr(tm, 0); got != "abbbb" {
		t.Errorf("row = %q, want abbbb", got)
	}
}

func TestDECSpecialGraphics(t *testing.T) {
	tm := New(1, 10)
	tm.ProcessOutput([]byte("\x1b(0lqk\x1b(Bq"))
	if got := cellStr(tm, 0); got != "┌─┐q" {
		t.Errorf("row = %q, want ┌─┐q", got)
	}
	tm = New(1, 10)
	tm.ProcessOutput([]byte("\x1b)0\x0ex\x0fx"))
	if got := cellStr(tm, 0); got != "│x" {
		t.Errorf("G1 via SO/SI row = %q, want │x", got)
	}
}

func TestWideCharsOccupyTwoCells(t *testing.T) {
	tm := New(1, 10)
	tm.ProcessOutput([]byte("界a"))
	if tm.curCol != 3 {
		t.Errorf("cursor col = %d, want 3", tm.curCol)
	}
	if got := tm.PlainScreen()[0]; got != "界a" {
		t.Errorf("plain = %q, want 界a", got)
	}
	if w := ansi.StringWidth(tm.Render()); w != 3 {
		t.Errorf("rendered width = %d, want 3", w)
	}
}

func TestWideCharAtLastColumnWraps(t *testing.T) {
	tm := New(2, 3)
	tm.ProcessOutput([]byte("ab界"))
	if got := tm.PlainScreen()[1]; got != "界" {
		t.Errorf("row 1 = %q, want 界 wrapped", got)
	}
}

func TestOverwritingHalfOfWideCharClearsIt(t *testing.T) {
	tm := New(1, 4)
	tm.ProcessOutput([]byte("界\x1b[2Gx"))
	if got := tm.PlainScreen()[0]; got != " x" {
		t.Errorf("row = %q, want ' x'", got)
	}
	if w := ansi.StringWidth(tm.Render()); w != 2 {
		t.Errorf("rendered width = %d, want 2", w)
	}
}

func TestCombiningMarkJoinsPreviousCell(t *testing.T) {
	tm := New(1, 5)
	tm.ProcessOutput([]byte("éx"))
	if tm.curCol != 2 {
		t.Errorf("cursor col = %d, want 2", tm.curCol)
	}
	if got := tm.PlainScreen()[0]; got != "éx" {
		t.Errorf("plain = %q", got)
	}
}

func TestUTF8SplitAcrossReads(t *testing.T) {
	tm := New(1, 5)
	b := []byte("a─b")
	tm.ProcessOutput(b[:2])
	tm.ProcessOutput(b[2:])
	if got := tm.PlainScreen()[0]; got != "a─b" {
		t.Errorf("row = %q, want a─b", got)
	}
}

func TestResizeResetsScrollRegion(t *testing.T) {
	tm := New(5, 10)
	tm.Resize(10, 10)
	if tm.scrollTop != 0 || tm.scrollBot != 9 {
		t.Fatalf("scroll region = %d..%d, want 0..9", tm.scrollTop, tm.scrollBot)
	}
	tm.ProcessOutput([]byte("\x1b[10;1Hbottom\n"))
	if got := cellStr(tm, 8); got != "bottom" {
		t.Errorf("row 8 = %q, want bottom scrolled up by one", got)
	}
}

func TestShrinkKeepsCursorLineVisible(t *testing.T) {
	tm := New(6, 10)
	tm.ProcessOutput([]byte("a\r\nb\r\nc\r\nd\r\n$ "))
	tm.Resize(3, 10)
	if got := cellStr(tm, tm.curRow); got != "$" {
		t.Errorf("cursor row %d = %q, want prompt", tm.curRow, got)
	}
	if got := cellStr(tm, 0); got != "c" {
		t.Errorf("row 0 = %q, want c", got)
	}
	if n := len(tm.scrollback); n != 2 {
		t.Errorf("scrollback = %d, want 2 pushed lines", n)
	}
}

func TestSavedCursorClampedAfterShrink(t *testing.T) {
	tm := New(10, 10)
	tm.ProcessOutput([]byte("\x1b[9;9H\x1b7"))
	tm.Resize(3, 3)
	tm.ProcessOutput([]byte("\x1b8\x1b[K\x1b[2Pz"))
	if tm.curRow >= 3 || tm.curCol >= 3 {
		t.Errorf("cursor (%d,%d) outside 3x3", tm.curRow, tm.curCol)
	}
}

func TestSaveRestoreCursorKeepsAttributes(t *testing.T) {
	tm := New(1, 5)
	tm.ProcessOutput([]byte("\x1b[31m\x1b7\x1b[0m\x1b8x"))
	if c := tm.cells[0][0]; !c.HasFG || c.FG != IndexedColor(1) {
		t.Errorf("restored attr fg = %+v, want red", c.FG)
	}
}

func TestAutowrapOff(t *testing.T) {
	tm := New(2, 3)
	tm.ProcessOutput([]byte("\x1b[?7labcdef"))
	if got := cellStr(tm, 0); got != "abf" {
		t.Errorf("row 0 = %q, want abf", got)
	}
	if got := cellStr(tm, 1); got != "" {
		t.Errorf("row 1 = %q, want empty", got)
	}
}

func TestCursorUpOutsideRegionClampsToScreen(t *testing.T) {
	tm := New(10, 5)
	tm.ProcessOutput([]byte("\x1b[5;8r\x1b[2;1H\x1b[A"))
	if tm.curRow != 0 {
		t.Errorf("cursor row = %d, want 0 (above region clamps to screen top)", tm.curRow)
	}
}

func TestDeleteCharsBeyondLineKeepsLeftSide(t *testing.T) {
	tm := New(1, 6)
	tm.ProcessOutput([]byte("abcdef\x1b[3G\x1b[99P"))
	if got := cellStr(tm, 0); got != "ab" {
		t.Errorf("row = %q, want ab", got)
	}
}

func TestLongOSCPayloadIsNotPrinted(t *testing.T) {
	tm := New(1, 20)
	payload := strings.Repeat("A", 10000)
	tm.ProcessOutput([]byte("\x1b]52;c;" + payload + "\x07ok"))
	if got := cellStr(tm, 0); got != "ok" {
		t.Errorf("row = %q, want ok", got)
	}
}

func TestEscapeIntermediatesConsumed(t *testing.T) {
	tm := New(1, 10)
	tm.ProcessOutput([]byte("\x1b%G\x1b#8\x1b Fok"))
	if got := cellStr(tm, 0); got != "ok" {
		t.Errorf("row = %q, want ok", got)
	}
}

func TestControlCharInsideCSIExecutes(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("abcd\x1b[\r1Cx"))
	if got := cellStr(tm, 0); got != "axcd" {
		t.Errorf("row = %q, want axcd (CR executed mid-CSI)", got)
	}
}

func TestSGRColonColors(t *testing.T) {
	tm := New(1, 5)
	tm.ProcessOutput([]byte("\x1b[38:2::10:20:30;48:5:200mx"))
	c := tm.cells[0][0]
	if !c.HasFG || c.FG != RGBColor(10, 20, 30) {
		t.Errorf("fg = %+v, want rgb(10,20,30)", c.FG)
	}
	if !c.HasBG || c.BG != IndexedColor(200) {
		t.Errorf("bg = %+v, want 200", c.BG)
	}
	tm.ProcessOutput([]byte("\x1b[38:2:1:2:3my"))
	if c := tm.cells[0][1]; c.FG != RGBColor(1, 2, 3) {
		t.Errorf("fg without colorspace = %+v, want rgb(1,2,3)", c.FG)
	}
}

func TestSGRStrikeAndHidden(t *testing.T) {
	tm := New(1, 5)
	tm.ProcessOutput([]byte("\x1b[9;8mx\x1b[29;28my"))
	if c := tm.cells[0][0]; !c.Strike || !c.Hidden {
		t.Errorf("cell 0 strike=%v hidden=%v, want both", c.Strike, c.Hidden)
	}
	if c := tm.cells[0][1]; c.Strike || c.Hidden {
		t.Errorf("cell 1 strike=%v hidden=%v, want neither", c.Strike, c.Hidden)
	}
}

func TestEraseScrollbackKeepsScreen(t *testing.T) {
	tm := New(2, 5)
	tm.ProcessOutput([]byte("a\r\nb\r\nc\x1b[3J"))
	if len(tm.scrollback) != 0 {
		t.Errorf("scrollback = %d, want 0", len(tm.scrollback))
	}
	if got := cellStr(tm, 1); got != "c" {
		t.Errorf("row 1 = %q, want c (ED 3 must not clear the screen)", got)
	}
}

func TestScrollRegionDoesNotFeedScrollback(t *testing.T) {
	tm := New(5, 5)
	tm.ProcessOutput([]byte("\x1b[2;4r\x1b[4;1H\n\n\n"))
	if len(tm.scrollback) != 0 {
		t.Errorf("scrollback = %d, want 0 for a region not at the top", len(tm.scrollback))
	}
}

func TestLinesAfterExitInAltScreenShowsMainScreen(t *testing.T) {
	tm := New(3, 10)
	tm.ProcessOutput([]byte("shell\x1b[?1049hfullscreen-ui"))
	lines := tm.Lines()
	if len(lines) == 0 || !strings.Contains(lines[0], "shell") {
		t.Errorf("Lines() = %q, want main screen content", lines)
	}
	for _, l := range lines {
		if strings.Contains(l, "fullscreen-ui") {
			t.Errorf("Lines() leaked alt screen content: %q", lines)
		}
	}
}

func pipeReplies(t *testing.T, tm *Terminal) *bufio.Reader {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	tm.conn = writeConn{w}
	return bufio.NewReader(r)
}

func readReply(t *testing.T, br *bufio.Reader, want string) {
	t.Helper()
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, len(want))
		n := 0
		for n < len(buf) {
			k, err := br.Read(buf[n:])
			if err != nil {
				break
			}
			n += k
		}
		got <- string(buf[:n])
	}()
	select {
	case g := <-got:
		if g != want {
			t.Errorf("reply = %q, want %q", g, want)
		}
	case <-time.After(time.Second):
		t.Errorf("no reply, want %q", want)
	}
}

func TestModeReportsAnswered(t *testing.T) {
	tm := New(2, 10)
	br := pipeReplies(t, tm)
	tm.ProcessOutput([]byte("\x1b[?2026$p"))
	readReply(t, br, "\x1b[?2026;2$y")
	tm.ProcessOutput([]byte("\x1b[?2004h\x1b[?2004$p"))
	readReply(t, br, "\x1b[?2004;1$y")
	tm.ProcessOutput([]byte("\x1b[?9999$p"))
	readReply(t, br, "\x1b[?9999;0$y")
}

func TestSecondaryDeviceAttributesAnswered(t *testing.T) {
	tm := New(2, 10)
	br := pipeReplies(t, tm)
	tm.ProcessOutput([]byte("\x1b[>c"))
	readReply(t, br, "\x1b[>1;10;0c")
}

func TestMouseModesTracked(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("\x1b[?1002h\x1b[?1006h"))
	if m := tm.Mouse(); m.Tracking != 1002 || !m.SGR {
		t.Errorf("mouse = %+v, want 1002 SGR", m)
	}
	tm.ProcessOutput([]byte("\x1b[?1000l"))
	if m := tm.Mouse(); m.Tracking != 1002 {
		t.Errorf("resetting another mode cleared tracking: %+v", m)
	}
	tm.ProcessOutput([]byte("\x1b[?1002l\x1b[?1006l"))
	if m := tm.Mouse(); m.Tracking != 0 || m.SGR {
		t.Errorf("mouse = %+v, want off", m)
	}
	tm.ProcessOutput([]byte("\x1b[?1003h\x1bc"))
	if m := tm.Mouse(); m.Tracking != 0 {
		t.Errorf("RIS kept mouse tracking: %+v", m)
	}
}

func TestDefaultColorQueriesAnswered(t *testing.T) {
	defer SetHostForeground(nil)
	defer SetHostBackground(nil)
	SetHostForeground(color.RGBA{0xeb, 0xdb, 0xb2, 0xff})
	SetHostBackground(color.RGBA{0x1d, 0x20, 0x21, 0xff})
	tm := New(2, 10)
	br := pipeReplies(t, tm)
	tm.ProcessOutput([]byte("\x1b]11;?\x07"))
	readReply(t, br, "\x1b]11;rgb:1d1d/2020/2121\x07")
	tm.ProcessOutput([]byte("\x1b]10;?\x1b\\"))
	readReply(t, br, "\x1b]10;rgb:ebeb/dbdb/b2b2\x1b\\")
}

func TestSignalledProcessReportsShellStatus(t *testing.T) {
	tm := New(2, 10)
	if err := tm.StartArgv(t.TempDir(), "sh", "-c", "kill -INT $$"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	waitExited(t, tm)
	if code := tm.WaitExit(); code != 130 {
		t.Errorf("exit = %d, want 130", code)
	}
}

// A DA reply echoed back (or a query with a parameter) is not answered, so a
// program that echoes raw input cannot loop with the terminal.
func TestOnlyDeviceAttributeQueriesAreAnswered(t *testing.T) {
	tm := New(2, 10)
	br := pipeReplies(t, tm)
	tm.ProcessOutput([]byte("\x1b[?1;2c\x1b[>1;10;0c\x1b[5c\x1b[>5c\x1b[5n"))
	readReply(t, br, "\x1b[0n")
	tm.ProcessOutput([]byte("\x1b[0c"))
	readReply(t, br, "\x1b[?1;2c")
	tm.ProcessOutput([]byte("\x1b[>0c"))
	readReply(t, br, "\x1b[>1;10;0c")
}

// DCS, APC, PM and SOS are skipped up to ST; BEL does not end them and
// they never reach the OSC hook.
func TestControlStringsAreIgnored(t *testing.T) {
	tm := New(2, 20)
	var hooked []string
	tm.SetOSCHook(func(osc string) { hooked = append(hooked, osc) })
	tm.ProcessOutput([]byte("\x1bPq#0;2\a still dcs\x1b\\A\x1b_apc\x1b\\B\x1b^pm\x1b\\C\x1bXsos\x1b\\D"))
	if got := tm.PlainLines()[0]; got != "ABCD" {
		t.Errorf("screen = %q, want ABCD", got)
	}
	if len(hooked) != 0 {
		t.Errorf("OSC hook got %q", hooked)
	}
}
