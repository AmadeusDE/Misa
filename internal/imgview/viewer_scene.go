package imgview

import (
	"github.com/AmadeusDE/Misa/internal/imageframe"
	"image"
	"math"
)

func (m *Viewer) columns() int { return max(1, m.width/max(1, m.thumb)) }
func (m *Viewer) visibleRange() (int, int) {
	cols := m.columns()
	rows := max(1, (m.height-1)/max(1, m.thumb/2+2))
	page := cols * rows
	start := (m.index / page) * page
	return start, min(len(m.items), start+page)
}
func (m *Viewer) imageSize() image.Point {
	if len(m.items) == 0 {
		return image.Point{}
	}
	it := m.items[m.index]
	pix := m.pixels(m.index)
	if pix == nil {
		return image.Point{}
	}
	s := pix.Bounds().Size()
	if it.rotation%2 == 1 {
		s.X, s.Y = s.Y, s.X
	}
	return s
}
func (m *Viewer) scale() float64 {
	s := m.imageSize()
	if s.X == 0 || s.Y == 0 {
		return 1
	}
	w, h := float64(m.width), float64(max(0, m.height-1)*2)
	x, y := w/float64(s.X), h/float64(s.Y)
	switch m.fit {
	case fitNoUpscale:
		return min(1, min(x, y))
	case fitWidth:
		return x
	case fitHeight:
		return y
	case fitActual:
		return 1
	case fitManual:
		return m.zoom
	default:
		return min(x, y)
	}
}
func (m *Viewer) clampPan() {
	s := m.imageSize()
	z := m.scale()
	x := max(0, (int(float64(s.X)*z)-m.width)/2)
	y := max(0, (int(float64(s.Y)*z)-max(0, m.height-1)*2)/2)
	m.pan.X = max(-x, min(x, m.pan.X))
	m.pan.Y = max(-y, min(y, m.pan.Y))
}
func transformed(it *viewerItem, src *image.NRGBA) *image.NRGBA {
	if src == nil {
		return nil
	}
	if it.rotation == 0 && !it.flipX && !it.flipY {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	ow, oh := w, h
	if it.rotation%2 == 1 {
		ow, oh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a, b := x, y
			if it.flipX {
				a = w - 1 - a
			}
			if it.flipY {
				b = h - 1 - b
			}
			switch it.rotation {
			case 1:
				a, b = h-1-b, a
			case 2:
				a, b = w-1-a, h-1-b
			case 3:
				a, b = b, w-1-a
			}
			dst.SetNRGBA(a, b, src.NRGBAAt(x, y))
		}
	}
	return dst
}

// Scene returns viewport-local placements; the bottom row is reserved for status.
func (m *Viewer) Scene(owner, generation uint64) imageframe.Scene {
	w, h := m.width, max(0, m.height-1)
	s := imageframe.Scene{Owner: owner, Generation: generation, Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, w, h), PixelSize: image.Pt(w, h*2), CellPixels: image.Pt(1, 2)}}
	if w == 0 || h == 0 || len(m.items) == 0 {
		return s
	}
	add := func(i int, box image.Rectangle, z float64, pan image.Point) {
		it := &m.items[i]
		pix := transformed(it, m.pixels(i))
		if pix == nil {
			return
		}
		size := pix.Bounds().Size()
		if z <= 0 {
			z = math.Min(float64(box.Dx())/float64(size.X), float64(box.Dy())/float64(size.Y))
		}
		dw, dh := max(1, int(float64(size.X)*z)), max(1, int(float64(size.Y)*z))
		x := box.Min.X + (box.Dx()-dw)/2 - pan.X
		y := box.Min.Y + (box.Dy()-dh)/2 - pan.Y
		r := &imageframe.Raster{ID: uint64(i + 1), Generation: it.version + it.appearance, Pixels: pix}
		s.Placements = append(s.Placements, imageframe.Placement{ID: uint64(i + 1), Raster: r, Source: pix.Bounds(), Dest: image.Rect(x, y, x+dw, y+dh), Clip: box})
	}
	if !m.grid {
		add(m.index, image.Rect(0, 0, w, h*2), m.scale(), m.pan)
	} else {
		start, end := m.visibleRange()
		cols := m.columns()
		rowh := m.thumb/2 + 2
		for i := start; i < end; i++ {
			x := (i - start) % cols * m.thumb
			y := (i - start) / cols * rowh * 2
			add(i, image.Rect(x+1, y+2, min(w, x+m.thumb-1), min(h*2, y+rowh*2-2)), 0, image.Point{})
		}
	}
	return s
}
