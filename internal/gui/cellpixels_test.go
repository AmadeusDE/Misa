package gui

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/term"
)

// A font change has to reach the child as a new cell size, so programs that
// query pixel dimensions (the image viewer) stop guessing at one.
func TestFontChangeUpdatesCellPixels(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 240, 80
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	if w, h := a.term.CellPixels(); w != 0 || h != 0 {
		t.Fatalf("fresh terminal has cell pixels %dx%d, want unknown", w, h)
	}
	oldW, oldH := a.rend.fonts.cellW, a.rend.fonts.cellH

	a.onFont(Event{FontSize: a.fontSize + 4})

	w, h := a.term.CellPixels()
	if w != a.rend.fonts.cellW || h != a.rend.fonts.cellH {
		t.Fatalf("cell pixels = %dx%d, want %dx%d", w, h, a.rend.fonts.cellW, a.rend.fonts.cellH)
	}
	if w == oldW && h == oldH {
		t.Fatalf("font change left the cell size at %dx%d", oldW, oldH)
	}
}
