package gui

import (
	"image"
	"testing"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
)

func testRenderer(t testing.TB) *renderer {
	solid := 1.0
	return &renderer{fonts: embeddedFonts(t), pal: newPalette(config.GUIConfig{Opacity: &solid}), pad: 2}
}

func screen(rows, cols int, out string) *term.Snapshot {
	tm := term.New(rows, cols)
	tm.ProcessOutput([]byte(out))
	var s term.Snapshot
	tm.Snapshot(&s)
	return &s
}

func canvasFor(r *renderer, rows, cols int) *canvas {
	return newCanvas(2*r.pad+cols*r.fonts.cellW, 2*r.pad+rows*r.fonts.cellH)
}

func TestChangedRowsAndDamage(t *testing.T) {
	r := testRenderer(t)
	c := canvasFor(r, 3, 4)
	var prev frame
	cur := cursorState{}
	if _, full := changedRows(screen(3, 4, "ab"), cur, &prev); !full {
		t.Fatal("empty frame not full")
	}
	r.draw(c, screen(3, 4, "ab"), cur, &prev)
	rows, full := changedRows(screen(3, 4, "ab\r\n\r\nX"), cur, &prev)
	if full || len(rows) != 1 || rows[0] != 2 {
		t.Fatalf("rows = %v full %v", rows, full)
	}
	y := r.pad + 2*r.fonts.cellH
	want := image.Rect(0, y, c.w, y+r.fonts.cellH)
	if d := r.damage(rows, false, c.bounds()); len(d) != 1 || d[0] != want {
		t.Fatalf("damage = %v, want %v", d, want)
	}
	if d := r.damage([]int{0, 1}, false, c.bounds()); len(d) != 1 || d[0].Dy() != 2*r.fonts.cellH {
		t.Fatalf("adjacent rows not merged: %v", d)
	}
	if rows, _ := changedRows(screen(3, 4, "ab"), cur, &prev); len(rows) != 0 {
		t.Fatalf("unchanged screen reports %v", rows)
	}
	if _, full := changedRows(screen(2, 4, "ab"), cur, &prev); !full {
		t.Fatal("resize not full")
	}
}

func TestDrawPaintsBackgroundAndCursor(t *testing.T) {
	r := testRenderer(t)
	c := canvasFor(r, 1, 3)
	var prev frame
	r.draw(c, screen(1, 3, "\x1b[41m \x1b[m"), cursorState{row: 0, col: 2, visible: true, focused: true}, &prev)
	mid := r.pad + r.fonts.cellH/2
	if got := c.at(r.pad+r.fonts.cellW/2, mid); got != r.pal.colors[1] {
		t.Errorf("red cell = %v", got)
	}
	if got := c.at(r.pad+2*r.fonts.cellW+r.fonts.cellW/2, mid); got != r.pal.cursor {
		t.Errorf("cursor cell = %v", got)
	}
	if got := c.at(0, 0); got != r.pal.bg {
		t.Errorf("padding = %v", got)
	}
}

func TestCursorMoveDamagesBothRows(t *testing.T) {
	r := testRenderer(t)
	c := canvasFor(r, 3, 2)
	var prev frame
	s := screen(3, 2, "")
	r.draw(c, s, cursorState{row: 0, visible: true, focused: true}, &prev)
	rows, _ := changedRows(s, cursorState{row: 2, visible: true, focused: true}, &prev)
	if len(rows) != 2 || rows[0] != 0 || rows[1] != 2 {
		t.Fatalf("rows = %v", rows)
	}
}

func TestTranslucentOnlyBehindDefaultBackground(t *testing.T) {
	r := testRenderer(t)
	r.pal.alpha = 100
	c := canvasFor(r, 1, 3)
	var prev frame
	r.draw(c, screen(1, 3, " \x1b[44m \x1b[m"), cursorState{}, &prev)
	mid := r.pad + r.fonts.cellH/2
	if a := c.alphaAt(r.pad+r.fonts.cellW/2, mid); a != 100 {
		t.Errorf("default background alpha = %d", a)
	}
	if a := c.alphaAt(r.pad+r.fonts.cellW+r.fonts.cellW/2, mid); a != 255 {
		t.Errorf("blue background alpha = %d", a)
	}
	if a := c.alphaAt(0, 0); a != 100 {
		t.Errorf("padding alpha = %d", a)
	}
}
