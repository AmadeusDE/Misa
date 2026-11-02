package term

import (
	"math"

	"github.com/creack/pty"
)

// maxCellPixels bounds a recorded cell size. No font needs more, and it keeps
// the pixel arithmetic for a huge grid from looking like a deliberate request.
const maxCellPixels = 4096

// ptySize describes the PTY to its program, with pixel dimensions when the
// host's cell size is known.
func ptySize(rows, cols, cellW, cellH int) *pty.Winsize {
	ws := &pty.Winsize{Rows: satUint16(rows), Cols: satUint16(cols)}
	if cellW > 0 && cellH > 0 {
		ws.X = satUint16(cols * cellW)
		ws.Y = satUint16(rows * cellH)
	}
	return ws
}

func satUint16(n int) uint16 {
	if n <= 0 {
		return 0
	}
	return uint16(min(n, math.MaxUint16))
}

// SetCellPixels records the host's cell size in physical pixels so a
// program can query pixel dimensions (TIOCGWINSZ, CSI 14t, CSI 16t).
// Zero or negative values mean unknown.
func (t *Terminal) SetCellPixels(w, h int) {
	if w <= 0 || h <= 0 {
		w, h = 0, 0
	} else {
		w, h = min(w, maxCellPixels), min(h, maxCellPixels)
	}
	t.mu.Lock()
	changed := t.cellW != w || t.cellH != h
	t.cellW, t.cellH = w, h
	if changed {
		// Placements were measured in cells with the old metrics, and a decode
		// running outside the lock used them too: drop that work and redraw.
		t.gfx.epoch++
		t.gfx.gen++
	}
	rows, cols := t.Rows, t.Cols
	t.mu.Unlock()
	// A font change leaves the grid alone, so the child needs a fresh ioctl to
	// notice the new pixel size.
	if changed && t.ptm != nil {
		setWinsize(t.ptm, ptySize(rows, cols, w, h))
	}
}

// CellPixels returns the recorded cell size; zero means unknown.
func (t *Terminal) CellPixels() (w, h int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cellW, t.cellH
}
