package term

// Snapshot is a copy of the visible screen for a frontend to draw.
type Snapshot struct {
	Rows, Cols    int
	Cells         []Cell // row-major, Rows*Cols
	CursorRow     int
	CursorCol     int
	CursorVisible bool
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
}

// SetOSCHook receives the OSC strings the emulator does not answer itself
// (titles, clipboard), without the leading ESC ] and terminator. It runs
// while the terminal is locked, so it must not call back into t.
func (t *Terminal) SetOSCHook(fn func(osc string)) {
	t.mu.Lock()
	t.oscHook = fn
	t.mu.Unlock()
}
