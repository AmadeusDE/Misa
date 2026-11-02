package term

import (
	"bufio"
	"os"
	"testing"
	"time"
)

// newSilentPipe installs a pipe as the PTY side and returns the read end
// alongside a reader for the replies that do arrive.
func newSilentPipe(t *testing.T, tm *Terminal) (*os.File, *bufio.Reader) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	tm.ptm = w
	return r, bufio.NewReader(r)
}

// expectSilence fails when the terminal answers within a short window. It
// leaves the read deadline cleared, so a later readReply still works.
func expectSilence(t *testing.T, r *os.File) {
	t.Helper()
	if err := r.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := r.Read(buf)
	if n > 0 {
		t.Fatalf("reply %q, want none", buf[:n])
	}
	if err == nil {
		t.Fatalf("read returned no data and no error")
	}
	if err := r.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}

// CSI 14t reports the text area in pixels and CSI 16t the cell size, both as
// height;width, which is the order xterm uses.
func TestPixelSizeReports(t *testing.T) {
	tm := New(24, 80)
	tm.SetCellPixels(9, 20)
	br := pipeReplies(t, tm)

	tm.ProcessOutput([]byte("\x1b[16t"))
	readReply(t, br, "\x1b[6;20;9t")
	tm.ProcessOutput([]byte("\x1b[14t"))
	readReply(t, br, "\x1b[4;480;720t")
	tm.ProcessOutput([]byte("\x1b[18t"))
	readReply(t, br, "\x1b[8;24;80t")

	tm.Resize(30, 100)
	if w, h := tm.CellPixels(); w != 9 || h != 20 {
		t.Fatalf("cell pixels after resize = %dx%d, want 9x20", w, h)
	}
	tm.ProcessOutput([]byte("\x1b[16t"))
	readReply(t, br, "\x1b[6;20;9t")
	tm.ProcessOutput([]byte("\x1b[14t"))
	readReply(t, br, "\x1b[4;600;900t")
	tm.ProcessOutput([]byte("\x1b[18t"))
	readReply(t, br, "\x1b[8;30;100t")
}

// A terminal whose host never said how big a cell is must stay silent on the
// pixel queries rather than invent a made-up geometry.
func TestNoPixelReportsWithoutCellSize(t *testing.T) {
	tm := New(24, 80)
	r, br := newSilentPipe(t, tm)

	tm.ProcessOutput([]byte("\x1b[14t\x1b[16t"))
	expectSilence(t, r)

	tm.ProcessOutput([]byte("\x1b[18t"))
	readReply(t, br, "\x1b[8;24;80t")
}

func TestSetCellPixelsUnknownAndClamped(t *testing.T) {
	tm := New(24, 80)
	tm.SetCellPixels(9, 20)
	if w, h := tm.CellPixels(); w != 9 || h != 20 {
		t.Fatalf("cell pixels = %dx%d, want 9x20", w, h)
	}
	// Zero or negative on either axis means the whole size is unknown.
	for _, in := range [][2]int{{0, 20}, {9, 0}, {-3, 20}, {9, -1}, {0, 0}} {
		tm.SetCellPixels(in[0], in[1])
		if w, h := tm.CellPixels(); w != 0 || h != 0 {
			t.Fatalf("SetCellPixels(%d, %d) = %dx%d, want 0x0", in[0], in[1], w, h)
		}
	}
	tm.SetCellPixels(99999, 1)
	if w, h := tm.CellPixels(); w != 4096 || h != 1 {
		t.Fatalf("clamped cell pixels = %dx%d, want 4096x1", w, h)
	}
}

func TestPtySize(t *testing.T) {
	ws := ptySize(24, 80, 9, 20)
	if ws.Rows != 24 || ws.Cols != 80 || ws.X != 720 || ws.Y != 480 {
		t.Fatalf("ptySize = %+v, want 24/80/720/480", ws)
	}
	// Unknown cell size: rows and columns only.
	ws = ptySize(24, 80, 0, 20)
	if ws.X != 0 || ws.Y != 0 {
		t.Fatalf("unknown cell size gave pixels %+v", ws)
	}
	// Saturate instead of wrapping, on either axis.
	ws = ptySize(1000, 1000, 4096, 4096)
	if ws.X != 65535 || ws.Y != 65535 {
		t.Fatalf("saturated ptySize X/Y = %d/%d, want 65535", ws.X, ws.Y)
	}
}
