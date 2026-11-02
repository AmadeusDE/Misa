package term

import (
	"image"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

// Pixel-to-cell math needs a cell size. The host reports one through
// SetCellPixels; when it does not, placements without explicit c/r assume the
// documented fallback below, which is the same 10x20 the image viewer uses.
const (
	fallbackCellWidth  = 10
	fallbackCellHeight = 20
	maxGraphicsCells   = 4096
	maxGraphicsSpan    = 1 << 20 // pixels of source or offset, far past any image
)

func (t *Terminal) cellPixels() (int, int) {
	if t.cellW > 0 && t.cellH > 0 {
		return t.cellW, t.cellH
	}
	return fallbackCellWidth, fallbackCellHeight
}

// gfxPlacement turns a place or transmit-and-place command into the placement
// it describes: the cursor cell is the anchor, size is the image in pixels.
func (t *Terminal) gfxPlacement(cmd kittygfx.Command, size image.Point) (gfxPlacement, bool) {
	// Every value below is clamped before it is added, so a client cannot make
	// the geometry arithmetic overflow with huge source or offset fields.
	bounds := image.Rectangle{Max: size}
	src := bounds
	if cmd.Has('x') {
		src.Min.X = min(cmd.SourceX, size.X)
	}
	if cmd.Has('y') {
		src.Min.Y = min(cmd.SourceY, size.Y)
	}
	if cmd.Has('w') && cmd.SourceWidth > 0 {
		src.Max.X = src.Min.X + min(cmd.SourceWidth, maxGraphicsSpan)
	}
	if cmd.Has('h') && cmd.SourceHeight > 0 {
		src.Max.Y = src.Min.Y + min(cmd.SourceHeight, maxGraphicsSpan)
	}
	src = src.Intersect(bounds)
	if src.Empty() {
		return gfxPlacement{}, false
	}
	offX, offY := min(cmd.OffsetX, maxGraphicsSpan), min(cmd.OffsetY, maxGraphicsSpan)
	cellW, cellH := t.cellPixels()
	cols, rows := cmd.Columns, cmd.Rows
	if cols <= 0 {
		cols = (offX + src.Dx() + cellW - 1) / cellW
	}
	if rows <= 0 {
		rows = (offY + src.Dy() + cellH - 1) / cellH
	}
	cols, rows = clampInt(cols, 1, maxGraphicsCells), clampInt(rows, 1, maxGraphicsCells)
	return gfxPlacement{
		imageID: cmd.ID,
		pid:     cmd.PlacementID,
		screen:  t.shownScreen(),
		cell:    image.Rect(t.curCol, t.curRow, t.curCol+cols, t.curRow+rows),
		source:  src,
		offset:  image.Pt(offX, offY),
	}, true
}

// gfxAdvance moves the cursor past a placement unless the command asked for
// C=1: Kitty leaves it on the image's last row, one column past its right edge.
func (t *Terminal) gfxAdvance(cmd kittygfx.Command, p gfxPlacement) {
	if cmd.CursorMovement == 1 {
		return
	}
	t.curRow = min(p.cell.Max.Y-1, t.Rows-1)
	t.curCol = min(p.cell.Max.X, t.Cols-1)
	t.wrapNext = false
}

// graphicsPlace queues a=p. The image may still be waiting in the job queue,
// in which case its size comes from the transfer that will create it.
func (t *Terminal) graphicsPlace(cmd kittygfx.Command) {
	if !cmd.Has('i') && !cmd.Has('I') {
		t.gfxReply(cmd, errGraphicsNeedID)
		return
	}
	s := t.shownScreen()
	size := image.Point{}
	if img := t.gfx.resolve(s, cmd.ID, cmd.Number); img != nil {
		size = img.raster.Bounds().Size()
	} else {
		size = t.pendingSize(s, cmd.ID, cmd.Number)
	}
	if size.X <= 0 || size.Y <= 0 {
		t.gfxReply(cmd, errGraphicsNoImage)
		return
	}
	p, ok := t.gfxPlacement(cmd, size)
	if !ok {
		t.gfxReply(cmd, errGraphicsBadSource)
		return
	}
	// The cursor moves when the placement is committed, not when it is
	// queued: a placement that fails must leave the cursor alone.
	t.gfxJobs = append(t.gfxJobs, gfxJob{cmd: cmd, screen: s, number: cmd.Number, place: &p})
}

// queueTransmit queues the decode of a complete transfer, plus the placement
// an a=T command asked for.
func (t *Terminal) queueTransmit(cmd kittygfx.Command, data []byte) {
	if cmd.Width <= 0 || cmd.Height <= 0 {
		t.gfxReply(cmd, errGraphicsBadSource)
		return
	}
	job := gfxJob{cmd: cmd, screen: t.shownScreen(), data: data}
	if cmd.Action == kittygfx.ActionTransmitPlace {
		p, ok := t.gfxPlacement(cmd, image.Pt(cmd.Width, cmd.Height))
		if !ok {
			t.gfxReply(cmd, errGraphicsBadSource)
			return
		}
		job.place = &p
	}
	t.gfxJobs = append(t.gfxJobs, job)
}

// pendingSize reports the pixel size of an image that a transfer in flight or
// a queued transmit is about to create.
func (t *Terminal) pendingSize(s screen, id, number uint32) image.Point {
	match := func(cmd kittygfx.Command) bool {
		if cmd.Width <= 0 || cmd.Height <= 0 {
			return false
		}
		if id != 0 {
			return cmd.ID == id
		}
		return number != 0 && cmd.Number == number
	}
	if tr := t.gfx.transfer; tr != nil && tr.screen == s && match(tr.cmd) {
		return image.Pt(tr.cmd.Width, tr.cmd.Height)
	}
	for _, job := range t.gfxJobs {
		if job.screen != s || job.cmd.Action != kittygfx.ActionTransmit && job.cmd.Action != kittygfx.ActionTransmitPlace {
			continue
		}
		if match(job.cmd) {
			return image.Pt(job.cmd.Width, job.cmd.Height)
		}
	}
	return image.Point{}
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
