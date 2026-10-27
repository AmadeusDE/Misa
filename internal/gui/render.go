package gui

import (
	"image"

	"github.com/AmadeusDE/Misa/internal/term"
)

type cursorState struct {
	row, col int
	visible  bool
	focused  bool
}

// frame is what a buffer last showed, so the next draw into it repaints
// only rows that changed.
type frame struct {
	cells      []term.Cell
	rows, cols int
	cursor     cursorState
	valid      bool
}

type renderer struct {
	fonts *fontSet
	pal   palette
	pad   int
}

// changedRows lists the rows of snap that differ from what prev holds; full
// means prev is unusable and everything must be drawn.
func changedRows(snap *term.Snapshot, cur cursorState, prev *frame) (rows []int, full bool) {
	if !prev.valid || prev.rows != snap.Rows || prev.cols != snap.Cols {
		return nil, true
	}
	for row := 0; row < snap.Rows; row++ {
		if rowChanged(snap.Cells[row*snap.Cols:(row+1)*snap.Cols], prev, row, cur) {
			rows = append(rows, row)
		}
	}
	return rows, false
}

func rowChanged(line []term.Cell, prev *frame, row int, cur cursorState) bool {
	if (prev.cursor.row == row || cur.row == row) && prev.cursor != cur {
		return true
	}
	old := prev.cells[row*prev.cols : (row+1)*prev.cols]
	for i := range line {
		if line[i] != old[i] {
			return true
		}
	}
	return false
}

func (f *frame) record(snap *term.Snapshot, cur cursorState) {
	f.cells = append(f.cells[:0], snap.Cells...)
	f.rows, f.cols, f.cursor, f.valid = snap.Rows, snap.Cols, cur, true
}

// draw brings c, which last showed prev, up to snap.
func (r *renderer) draw(c *canvas, snap *term.Snapshot, cur cursorState, prev *frame) {
	rows, full := changedRows(snap, cur, prev)
	if full {
		c.fillAlpha(c.bounds(), r.pal.bg, r.pal.alpha)
		for row := 0; row < snap.Rows; row++ {
			rows = append(rows, row)
		}
	}
	for _, row := range rows {
		line := snap.Cells[row*snap.Cols : (row+1)*snap.Cols]
		y := r.pad + row*r.fonts.cellH
		c.fillAlpha(image.Rect(r.pad, y, r.pad+snap.Cols*r.fonts.cellW, y+r.fonts.cellH), r.pal.bg, r.pal.alpha)
		r.drawRow(c, line, y, cursorCol(cur, row))
		if cur.visible && !cur.focused && cur.row == row {
			r.drawHollowCursor(c, cur.col, y)
		}
	}
	prev.record(snap, cur)
}

// damage turns changed rows into buffer rectangles, merging adjacent ones.
func (r *renderer) damage(rows []int, full bool, bounds image.Rectangle) []image.Rectangle {
	if full {
		return []image.Rectangle{bounds}
	}
	var rects []image.Rectangle
	for _, row := range rows {
		y := r.pad + row*r.fonts.cellH
		band := image.Rect(0, y, bounds.Dx(), y+r.fonts.cellH)
		if n := len(rects); n > 0 && rects[n-1].Max.Y == band.Min.Y {
			rects[n-1].Max.Y = band.Max.Y
			continue
		}
		rects = append(rects, band)
	}
	return rects
}

// cursorCol is the column of a solid cursor on row, or -1.
func cursorCol(cur cursorState, row int) int {
	if cur.visible && cur.focused && cur.row == row {
		return cur.col
	}
	return -1
}

func (r *renderer) cellRect(col, y, span int) image.Rectangle {
	x := r.pad + col*r.fonts.cellW
	return image.Rect(x, y, x+span*r.fonts.cellW, y+r.fonts.cellH)
}

func span(line []term.Cell, col int) int {
	if line[col].Wide && col+1 < len(line) && line[col+1].Continuation() {
		return 2
	}
	return 1
}

// drawRow paints backgrounds first, then glyphs, so a glyph spilling into a
// blank neighbour is not painted over.
func (r *renderer) drawRow(c *canvas, line []term.Cell, y, cursor int) {
	colors := func(col int) (fg, bg rgb, defaultBG bool) {
		fg, bg, defaultBG = r.pal.cellColors(&line[col])
		if col == cursor || (cursor == col+1 && span(line, col) == 2) {
			return bg, r.pal.cursor, false
		}
		return fg, bg, defaultBG
	}
	for col := 0; col < len(line); col++ {
		if line[col].Continuation() && col > 0 {
			continue
		}
		if _, bg, defaultBG := colors(col); !defaultBG {
			c.fill(r.cellRect(col, y, span(line, col)), bg)
		}
	}
	for col := 0; col < len(line); col++ {
		cell := &line[col]
		if cell.Continuation() || cell.Hidden {
			continue
		}
		fg, _, _ := colors(col)
		n := span(line, col)
		rect := r.cellRect(col, y, n)
		clip := rect
		if col+n < len(line) && blank(&line[col+n]) {
			clip.Max.X += r.fonts.cellW
		}
		s := cellStyle(cell.Bold, cell.Italic)
		if g := r.fonts.glyph(cell.Ch, s, n); g != nil {
			c.blend(g.mask, rect.Min, clip, fg)
		}
		for _, mark := range cell.Marks() {
			if g := r.fonts.glyph(mark, s, n); g != nil {
				c.blend(g.mask, rect.Min, rect, fg)
			}
		}
		lw := r.fonts.lineWidth
		if cell.Under {
			c.fill(image.Rect(rect.Min.X, y+r.fonts.underline, rect.Max.X, y+r.fonts.underline+lw), fg)
		}
		if cell.Strike {
			mid := y + r.fonts.cellH/2
			c.fill(image.Rect(rect.Min.X, mid, rect.Max.X, mid+lw), fg)
		}
	}
}

func blank(c *term.Cell) bool {
	return (c.Ch == ' ' || c.Ch == 0) && !c.HasBG && !c.Reverse && c.Marks() == ""
}

func (r *renderer) drawHollowCursor(c *canvas, col, y int) {
	rect := r.cellRect(col, y, 1)
	lw := r.fonts.lineWidth
	for _, edge := range []image.Rectangle{
		{rect.Min, image.Pt(rect.Max.X, rect.Min.Y+lw)},
		{image.Pt(rect.Min.X, rect.Max.Y-lw), rect.Max},
		{rect.Min, image.Pt(rect.Min.X+lw, rect.Max.Y)},
		{image.Pt(rect.Max.X-lw, rect.Min.Y), rect.Max},
	} {
		c.fill(edge, r.pal.cursor)
	}
}
