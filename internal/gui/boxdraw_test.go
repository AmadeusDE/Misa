package gui

import (
	"image"
	"testing"
)

func lit(m *image.Alpha, x, y int) bool { return m.AlphaAt(x, y).A > 127 }

func TestBoxWeightsTable(t *testing.T) {
	tests := map[rune][4]byte{
		'─': {0, 1, 0, 1}, '┃': {2, 0, 2, 0}, '┌': {0, 1, 1, 0}, '┛': {2, 0, 0, 2},
		'┼': {1, 1, 1, 1}, '╋': {2, 2, 2, 2}, '═': {0, 3, 0, 3}, '╬': {3, 3, 3, 3},
		'╴': {0, 0, 0, 1}, '╿': {2, 0, 1, 0}, '╭': {}, '╳': {},
	}
	for r, want := range tests {
		if got := boxWeights(r); got != want {
			t.Errorf("%c = %v, want %v", r, got, want)
		}
	}
}

func TestBoxLinesReachCellEdges(t *testing.T) {
	const w, h = 9, 18
	horiz := boxGlyph('─', w, h)
	if !lit(horiz, 0, h/2) || !lit(horiz, w-1, h/2) || lit(horiz, w/2, 0) {
		t.Error("─ does not span the cell width at mid height only")
	}
	corner := boxGlyph('┌', w, h)
	if !lit(corner, w-1, h/2) || !lit(corner, w/2, h-1) || lit(corner, 0, h/2) || lit(corner, w/2, 0) {
		t.Error("┌ arms wrong")
	}
	if !lit(corner, w/2, h/2) {
		t.Error("┌ corner not closed")
	}
}

func TestArcsMeetNeighbours(t *testing.T) {
	const w, h = 9, 18
	arc := boxGlyph('╭', w, h)
	vert := boxGlyph('│', w, h)
	horiz := boxGlyph('─', w, h)
	if !lit(arc, w/2, h-1) || lit(arc, w/2, 0) {
		t.Error("╭ does not end at the bottom middle")
	}
	if !lit(arc, w-1, h/2) || lit(arc, 0, h/2) {
		t.Error("╭ does not end at the right middle")
	}
	for x := 0; x < w; x++ {
		if lit(arc, x, h-1) != lit(vert, x, h-1) {
			t.Fatalf("╭ bottom edge differs from │ at x=%d", x)
		}
	}
	for y := 0; y < h; y++ {
		if lit(arc, w-1, y) != lit(horiz, w-1, y) {
			t.Fatalf("╭ right edge differs from ─ at y=%d", y)
		}
	}
}

func TestBlocksAndBraille(t *testing.T) {
	const w, h = 8, 16
	full := boxGlyph('█', w, h)
	for _, a := range full.Pix {
		if a != 255 {
			t.Fatal("█ not solid")
		}
	}
	lower := boxGlyph('▄', w, h)
	if lit(lower, 0, 0) || !lit(lower, 0, h-1) {
		t.Error("▄ wrong half")
	}
	quad := boxGlyph('▚', w, h)
	if !lit(quad, 0, 0) || lit(quad, w-1, 0) || !lit(quad, w-1, h-1) || lit(quad, 0, h-1) {
		t.Error("▚ quadrants wrong")
	}
	dots := boxGlyph('⠁', w, h)
	if !lit(dots, w/4, h/8) || lit(dots, 3*w/4, h/8) {
		t.Error("⠁ dot 1 misplaced")
	}
	if boxGlyph('a', w, h) != nil {
		t.Error("boxGlyph drew an ordinary letter")
	}
}
