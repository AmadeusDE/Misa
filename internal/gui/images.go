package gui

import (
	"image"
	"image/draw"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
	"github.com/AmadeusDE/Misa/internal/term"
)

// pixmap is one raster's pixels laid out for row reads: 3 bytes per pixel (RGB,
// always opaque) or 4 (non-premultiplied RGBA), with min the raster coordinate
// of pix[0].
type pixmap struct {
	pix    []byte
	stride int
	bpp    int
	min    image.Point
}

// compositor draws the terminal's images as one fixed layer and keeps the state
// that drawing reuses between frames; the renderer draws on one goroutine, so
// the buffers need no lock.
//
// The layer sits over the cell backgrounds and under the glyphs and the cursor
// of the cells an image spans. Misa's terminal store keeps a single layer and
// ignores z, so placements are drawn in the order the program made them: that
// matches Kitty's z<0 (behind the text) closely enough for the subset this
// terminal accepts, and the alternative — an image hiding the text — would hide
// the status line the viewer draws next to its picture.
//
// A placement's source window is scaled into the pixels of its cell rectangle
// with nearest-neighbour sampling. The terminal sizes a placement in cells, so
// the correction is at most one cell and a smooth filter would cost more than
// it shows; when the image is a whole number of cells (the common case for a
// frame-sized picture) one source pixel lands on one destination pixel.
//
// The destination is premultiplied, like the rest of the canvas, so a
// translucent image is composited over the cell background with the same
// formula canvas.blend uses for glyphs.
type compositor struct {
	// scratch holds the pixels of the last kittygfx raster drawn. The raster
	// exposes no pixel slice of its own, so drawing copies it; keeping the copy
	// means a frame that places one image several times copies it once.
	scratch   []byte
	scratchOf *kittygfx.Raster
	scratchPm pixmap

	// generic converts a raster type without a fast path.
	generic *image.NRGBA

	// cols maps destination columns to source byte offsets inside a row.
	cols []int32
}

// gfxDrawn records one placement as a buffer drew it: which image generation
// painted what geometry, and the buffer rectangle to repaint when it changes.
// The struct is comparable, so a frame can be compared with the next snapshot.
type gfxDrawn struct {
	imageID     uint32
	placementID uint32
	imageGen    uint64
	cell        image.Rectangle
	source      image.Rectangle
	offset      image.Point
	rect        image.Rectangle
}

// drawn lists where snap's placements paint in this renderer's pixels.
func (r *renderer) drawn(snap *term.Snapshot) []gfxDrawn {
	if len(snap.Graphics) == 0 {
		return nil
	}
	out := make([]gfxDrawn, 0, len(snap.Graphics))
	for i := range snap.Graphics {
		p := &snap.Graphics[i]
		if p.Raster == nil {
			continue
		}
		rect := r.placementRect(p).Intersect(r.placementClip(p))
		if rect.Empty() {
			continue
		}
		out = append(out, gfxDrawn{
			imageID:     p.ImageID,
			placementID: p.PlacementID,
			imageGen:    p.ImageGeneration,
			cell:        p.CellRect,
			source:      p.Source,
			offset:      p.PixelOffset,
			rect:        rect,
		})
	}
	return out
}

// gfxDamage lists the buffer rectangles a draw has to repaint because a
// placement appeared, vanished, moved or was replaced. Placements are compared
// in order and a difference damages both rectangles, so a reordering repaints
// more than it must but never leaves pixels of an image that is gone.
func gfxDamage(prev *frame, drawn []gfxDrawn) []image.Rectangle {
	var rects []image.Rectangle
	add := func(rect image.Rectangle) {
		for _, have := range rects {
			if have == rect {
				return
			}
		}
		rects = append(rects, rect)
	}
	for i := 0; i < len(prev.gfx) || i < len(drawn); i++ {
		if i < len(prev.gfx) && i < len(drawn) && prev.gfx[i] == drawn[i] {
			continue
		}
		if i < len(prev.gfx) {
			add(prev.gfx[i].rect)
		}
		if i < len(drawn) {
			add(drawn[i].rect)
		}
	}
	return rects
}

// placementRect is where a placement paints in buffer pixels: its cell
// rectangle in the renderer's real cell metrics and padding, shifted by the
// pixel offset inside the first cell. Those metrics already include the buffer
// scale, so it is counted once.
func (r *renderer) placementRect(p *term.GraphicsPlacement) image.Rectangle {
	cw, ch := r.fonts.cellW, r.fonts.cellH
	return image.Rect(
		r.pad+p.CellRect.Min.X*cw+p.PixelOffset.X,
		r.pad+p.CellRect.Min.Y*ch+p.PixelOffset.Y,
		r.pad+p.CellRect.Max.X*cw,
		r.pad+p.CellRect.Max.Y*ch,
	)
}

// placementClip is the part of that rectangle the terminal says is on screen.
func (r *renderer) placementClip(p *term.GraphicsPlacement) image.Rectangle {
	cw, ch := r.fonts.cellW, r.fonts.cellH
	return image.Rect(
		r.pad+p.Clip.Min.X*cw,
		r.pad+p.Clip.Min.Y*ch,
		r.pad+p.Clip.Max.X*cw,
		r.pad+p.Clip.Max.Y*ch,
	)
}

// drawImages composites every placement of snap inside region, in the order the
// program made them.
func (r *renderer) drawImages(c *canvas, snap *term.Snapshot, region image.Rectangle) {
	for i := range snap.Graphics {
		r.drawPlacement(c, &snap.Graphics[i], region)
	}
}

// drawPlacement alpha-blends one placement's source window over region.
func (r *renderer) drawPlacement(c *canvas, p *term.GraphicsPlacement, region image.Rectangle) {
	if p.Raster == nil {
		return
	}
	dest := r.placementRect(p)
	area := dest.Intersect(r.placementClip(p)).Intersect(region).Intersect(c.bounds())
	if area.Empty() {
		return
	}
	src := p.Source.Intersect(p.Raster.Bounds())
	if src.Empty() {
		return
	}
	pm, ok := r.pixmapOf(p.Raster)
	if !ok {
		return
	}
	cols := r.columns(pm, src, dest, area)
	ri, gi, bi := 2, 1, 0
	if c.order == orderRGBA {
		ri, gi, bi = 0, 1, 2
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		sy := src.Min.Y + (y-dest.Min.Y)*src.Dy()/dest.Dy()
		spix := pm.pix[(sy-pm.min.Y)*pm.stride:]
		dst := c.pix[y*c.stride:]
		if pm.bpp == 3 {
			for i, off := range cols {
				sp := spix[off:]
				di := (area.Min.X + i) * 4
				dst[di+ri], dst[di+gi], dst[di+bi], dst[di+3] = sp[0], sp[1], sp[2], 0xff
			}
			continue
		}
		for i, off := range cols {
			sp := spix[off:]
			a := uint32(sp[3])
			if a == 0 {
				continue
			}
			di := (area.Min.X + i) * 4
			if a == 255 {
				dst[di+ri], dst[di+gi], dst[di+bi], dst[di+3] = sp[0], sp[1], sp[2], 0xff
				continue
			}
			inv := 255 - a
			dst[di+ri] = uint8((uint32(dst[di+ri])*inv + uint32(sp[0])*a) / 255)
			dst[di+gi] = uint8((uint32(dst[di+gi])*inv + uint32(sp[1])*a) / 255)
			dst[di+bi] = uint8((uint32(dst[di+bi])*inv + uint32(sp[2])*a) / 255)
			dst[di+3] = uint8((uint32(dst[di+3])*inv + 255*a) / 255)
		}
	}
}

// columns maps every destination column of area to the byte offset of its
// source pixel inside a pixmap row, so the inner loop divides once per column
// instead of once per pixel.
func (r *renderer) columns(pm pixmap, src, dest, area image.Rectangle) []int32 {
	if cap(r.gfx.cols) < area.Dx() {
		r.gfx.cols = make([]int32, area.Dx())
	}
	cols := r.gfx.cols[:area.Dx()]
	for i := range cols {
		sx := src.Min.X + (area.Min.X+i-dest.Min.X)*src.Dx()/dest.Dx()
		cols[i] = int32((sx - pm.min.X) * pm.bpp)
	}
	return cols
}

// pixmapOf returns img's pixels for fast row reads. A raster type it cannot
// read directly is converted once into the compositor's scratch image.
func (r *renderer) pixmapOf(img image.Image) (pixmap, bool) {
	switch src := img.(type) {
	case *kittygfx.Raster:
		if r.gfx.scratchOf == src {
			return r.gfx.scratchPm, true
		}
		b := src.Bounds()
		bpp := src.Format() / 8
		if b.Empty() || bpp != 3 && bpp != 4 {
			return pixmap{}, false
		}
		n := b.Dx() * b.Dy() * bpp
		if cap(r.gfx.scratch) < n {
			r.gfx.scratch = make([]byte, n)
		}
		pix := r.gfx.scratch[:n]
		if err := src.CopyPixels(pix); err != nil {
			return pixmap{}, false
		}
		r.gfx.scratchPm = pixmap{pix: pix, stride: b.Dx() * bpp, bpp: bpp}
		r.gfx.scratchOf = src
		return r.gfx.scratchPm, true
	case *image.NRGBA:
		b := src.Bounds()
		return pixmap{pix: src.Pix, stride: src.Stride, bpp: 4, min: b.Min}, !b.Empty()
	}
	b := img.Bounds()
	if b.Empty() {
		return pixmap{}, false
	}
	if r.gfx.generic == nil || r.gfx.generic.Bounds() != b {
		r.gfx.generic = image.NewNRGBA(b)
	}
	draw.Draw(r.gfx.generic, b, img, b.Min, draw.Src)
	return pixmap{pix: r.gfx.generic.Pix, stride: r.gfx.generic.Stride, bpp: 4, min: b.Min}, true
}
