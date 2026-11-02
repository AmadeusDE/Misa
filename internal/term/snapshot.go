package term

import "image"

// Snapshot is a copy of the visible screen for a frontend to draw.
type Snapshot struct {
	Rows, Cols    int
	Cells         []Cell // row-major, Rows*Cols
	CursorRow     int
	CursorCol     int
	CursorVisible bool

	// GraphicsGeneration changes whenever the visible graphics change, even
	// when no cell does. Graphics lists the placements of the shown screen.
	GraphicsGeneration uint64
	Graphics           []GraphicsPlacement
}

// GraphicsPlacement is one image placement on the visible screen. Raster is
// immutable once published, so a snapshot shares it rather than copying
// pixels: a replacement publishes a new image and leaves old snapshots alone.
//
// CellRect is in terminal cells and may extend past the screen; Clip is its
// intersection with the screen and any scroll-region clipping. Source is in
// raster pixels and PixelOffset is
// the offset inside the first cell, both of which a frontend turns into pixels
// with its own cell metrics. The terminal keeps one fixed layer, so the order
// of Graphics is the order the placements were made and z is ignored.
type GraphicsPlacement struct {
	ImageID         uint32
	PlacementID     uint32
	ImageGeneration uint64
	Raster          image.Image
	CellRect        image.Rectangle
	Clip            image.Rectangle
	Source          image.Rectangle
	PixelOffset     image.Point
}

// Continuation reports the right half of a double-width character, which a
// frontend leaves to the left half to draw.
func (c *Cell) Continuation() bool { return c.cont }

// Snapshot copies the screen into s, reusing its cell storage.
func (t *Terminal) Snapshot(s *Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s.Rows, s.Cols = t.Rows, t.Cols
	if cap(s.Cells) < t.Rows*t.Cols {
		s.Cells = make([]Cell, t.Rows*t.Cols)
	}
	s.Cells = s.Cells[:t.Rows*t.Cols]
	for r, row := range t.cells {
		copy(s.Cells[r*t.Cols:(r+1)*t.Cols], row)
	}
	s.CursorRow, s.CursorCol = t.curRow, t.curCol
	s.CursorVisible = !t.cursorHidden
	s.GraphicsGeneration = t.gfx.gen
	s.Graphics = t.gfx.visible(t.shownScreen(), t.Rows, t.Cols, s.Graphics)
}

func (t *Terminal) shownScreen() screen {
	if t.altScreen {
		return screenAlt
	}
	return screenMain
}

// SetOSCHook receives the OSC strings the emulator does not answer itself
// (titles, clipboard), without the leading ESC ] and terminator. It runs
// while the terminal is locked, so it must not call back into t.
func (t *Terminal) SetOSCHook(fn func(osc string)) {
	t.mu.Lock()
	t.oscHook = fn
	t.mu.Unlock()
}
