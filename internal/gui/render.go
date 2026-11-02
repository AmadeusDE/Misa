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

// cellMetrics is the cell geometry a buffer was drawn with. A change moves
// every glyph and every image, so the buffer has to be redrawn whole.
type cellMetrics struct {
	w, h, pad int
}

// frame is what a buffer last showed, so the next draw into it repaints only
// what changed. Each rotating backend buffer has its own frame.
type frame struct {
	cells   []term.Cell
	rows    int
	cols    int
	cursor  cursorState
	metrics cellMetrics
	gfx     []gfxDrawn
	valid   bool
}

type renderer struct {
	fonts *fontSet
	pal   palette
	pad   int
	gfx   compositor
}

func (r *renderer) metrics() cellMetrics {
	return cellMetrics{w: r.fonts.cellW, h: r.fonts.cellH, pad: r.pad}
}

// changedRows lists the rows of snap that differ from what prev holds; full
// means prev is unusable and everything must be drawn.
func changedRows(snap *term.Snapshot, cur cursorState, prev *frame, m cellMetrics) (rows []int, full bool) {
	if !prev.valid || prev.metrics != m || prev.rows != snap.Rows || prev.cols != snap.Cols {
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

// record remembers what f showed, so the next draw into the same buffer only
// repaints what changed since. drawn is owned by the frame from here on.
func (f *frame) record(snap *term.Snapshot, cur cursorState, drawn []gfxDrawn, m cellMetrics) {
	f.cells = append(f.cells[:0], snap.Cells...)
	f.rows, f.cols, f.cursor, f.metrics, f.gfx, f.valid = snap.Rows, snap.Cols, cur, m, drawn, true
}

// draw brings c, which last showed prev, up to snap.
func (r *renderer) draw(c *canvas, snap *term.Snapshot, cur cursorState, prev *frame) {
	rows, full := changedRows(snap, cur, prev, r.metrics())
	drawn := r.drawn(snap)
	if full {
		r.repaint(c, snap, cur, c.bounds())
	} else {
		for _, row := range rows {
			r.repaint(c, snap, cur, r.rowBand(row, c.bounds()))
		}
		for _, rect := range gfxDamage(prev, drawn) {
			r.repaint(c, snap, cur, rect)
		}
	}
	prev.record(snap, cur, drawn, r.metrics())
}

// repaint redraws every layer inside region: the cell backgrounds, the images
// over them, then the glyphs and the cursor of the rows it touches.
func (r *renderer) repaint(c *canvas, snap *term.Snapshot, cur cursorState, region image.Rectangle) {
	region = region.Intersect(c.bounds())
	if region.Empty() || snap.Rows <= 0 || snap.Cols <= 0 {
		return
	}
	c.fillAlpha(region, r.pal.bg, r.pal.alpha)
	first, last := r.rowRange(snap, region)
	for row := first; row <= last; row++ {
		r.drawBackgrounds(c, snap, row, region)
	}
	r.drawImages(c, snap, region)
	for row := first; row <= last; row++ {
		r.drawText(c, snap, row, cur, region)
	}
}

// rowBand is the buffer band one cell row occupies, padding included.
func (r *renderer) rowBand(row int, bounds image.Rectangle) image.Rectangle {
	y := r.pad + row*r.fonts.cellH
	return image.Rect(bounds.Min.X, y, bounds.Max.X, y+r.fonts.cellH)
}

// rowRange is the first and last cell row region touches.
func (r *renderer) rowRange(snap *term.Snapshot, region image.Rectangle) (first, last int) {
	first = max(0, (region.Min.Y-r.pad)/r.fonts.cellH)
	last = min(snap.Rows-1, (region.Max.Y-r.pad-1)/r.fonts.cellH)
	return first, last
}

// damage is what the backend is told to repaint: the whole buffer, or the
// changed row bands and the placement rectangles.
func (r *renderer) damage(rows []int, rects []image.Rectangle, full bool, bounds image.Rectangle) []image.Rectangle {
	if full {
		return []image.Rectangle{bounds}
	}
	var out []image.Rectangle
	for _, row := range rows {
		band := r.rowBand(row, bounds)
		if n := len(out); n > 0 && out[n-1].Max.Y == band.Min.Y && out[n-1].Min.X == band.Min.X && out[n-1].Max.X == band.Max.X {
			out[n-1].Max.Y = band.Max.Y
			continue
		}
		out = append(out, band)
	}
	for _, rect := range rects {
		if rect = rect.Intersect(bounds); !rect.Empty() {
			out = append(out, rect)
		}
	}
	return out
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

// drawBackgrounds paints the non-default cell backgrounds of one row. The
// default background is already there.
func (r *renderer) drawBackgrounds(c *canvas, snap *term.Snapshot, row int, clip image.Rectangle) {
	line := snap.Cells[row*snap.Cols : (row+1)*snap.Cols]
	y := r.pad + row*r.fonts.cellH
	for col := 0; col < len(line); col++ {
		if line[col].Continuation() && col > 0 {
			continue
		}
		if _, bg, defaultBG := r.pal.cellColors(&line[col]); !defaultBG {
			c.fill(r.cellRect(col, y, span(line, col)).Intersect(clip), bg)
		}
	}
}

// drawText paints the glyphs, combining marks, underlines and cursor of one
// row, clipped to clip.
func (r *renderer) drawText(c *canvas, snap *term.Snapshot, row int, cur cursorState, clip image.Rectangle) {
	line := snap.Cells[row*snap.Cols : (row+1)*snap.Cols]
	y := r.pad + row*r.fonts.cellH
	cursor := cursorCol(cur, row)
	lw := r.fonts.lineWidth
	for col := 0; col < len(line); col++ {
		cell := &line[col]
		if cell.Continuation() || cell.Hidden {
			continue
		}
		fg, bg, _ := r.pal.cellColors(cell)
		n := span(line, col)
		rect := r.cellRect(col, y, n)
		box := rect
		if col+n < len(line) && blank(&line[col+n]) {
			box.Max.X += r.fonts.cellW
		}
		if box = box.Intersect(clip); box.Empty() {
			continue
		}
		// The cursor block is part of the text layer, so it covers an image
		// under the cell instead of being hidden by it, and the glyph inverts
		// to the cell's own background.
		if col == cursor || (cursor == col+1 && n == 2) {
			c.fill(rect.Intersect(clip), r.pal.cursor)
			fg = bg
		}
		s := cellStyle(cell.Bold, cell.Italic)
		if g := r.fonts.glyph(cell.Ch, s, n); g != nil {
			c.blend(g.mask, rect.Min, box, fg)
		}
		for _, mark := range cell.Marks() {
			if g := r.fonts.glyph(mark, s, n); g != nil {
				c.blend(g.mask, rect.Min, rect.Intersect(clip), fg)
			}
		}
		if cell.Under {
			c.fill(image.Rect(rect.Min.X, y+r.fonts.underline, rect.Max.X, y+r.fonts.underline+lw).Intersect(clip), fg)
		}
		if cell.Strike {
			mid := y + r.fonts.cellH/2
			c.fill(image.Rect(rect.Min.X, mid, rect.Max.X, mid+lw).Intersect(clip), fg)
		}
	}
	if cur.visible && !cur.focused && cur.row == row {
		r.drawHollowCursor(c, cur.col, y, clip)
	}
}

func blank(c *term.Cell) bool {
	return (c.Ch == ' ' || c.Ch == 0) && !c.HasBG && !c.Reverse && c.Marks() == ""
}

func (r *renderer) drawHollowCursor(c *canvas, col, y int, clip image.Rectangle) {
	rect := r.cellRect(col, y, 1)
	lw := r.fonts.lineWidth
	for _, edge := range []image.Rectangle{
		{rect.Min, image.Pt(rect.Max.X, rect.Min.Y+lw)},
		{image.Pt(rect.Min.X, rect.Max.Y-lw), rect.Max},
		{rect.Min, image.Pt(rect.Min.X+lw, rect.Max.Y)},
		{image.Pt(rect.Max.X-lw, rect.Min.Y), rect.Max},
	} {
		c.fill(edge.Intersect(clip), r.pal.cursor)
	}
}
