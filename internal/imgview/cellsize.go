package imgview

import (
	"image"
	"math"

	"github.com/AmadeusDE/Misa/internal/config"
)

// defaultCellPixels is the assumed cell size when nothing measures the host: a
// cell twice as tall as it is wide at a typical physical size. Layout keeps its
// shape either way; only the 100% scale depends on the absolute size.
var defaultCellPixels = image.Pt(10, 20)

// maxCellPixels bounds a plausible cell size, so a hostile or broken host
// cannot inflate viewport arithmetic.
const maxCellPixels = 256

func clampCellPixels(n int) int { return max(1, min(n, maxCellPixels)) }

// CellPixels reports the physical size of one terminal cell used for layout.
func (m *Viewer) CellPixels() image.Point { return m.cellSize() }

// cellSize falls back to the default so a zero-value Viewer still lays out.
func (m *Viewer) cellSize() image.Point {
	if m.cellPixels.X <= 0 || m.cellPixels.Y <= 0 {
		return defaultCellPixels
	}
	return m.cellPixels
}

// SetCellSize records the physical size of one terminal cell. exact marks a
// measurement or an exact override rather than an aspect-only estimate.
// Non-positive sizes are ignored.
func (m *Viewer) SetCellSize(p image.Point, exact bool) {
	if p.X <= 0 || p.Y <= 0 {
		return
	}
	m.cellPixels = image.Pt(clampCellPixels(p.X), clampCellPixels(p.Y))
	m.exactCellPixels = exact
	m.clampPan()
}

// CellPixelsFor resolves the cell size for layout. An explicit imageviewer.yaml
// override wins over a measured size, and the default applies when neither is
// known. exact reports a real measurement or an exact override instead of an
// aspect-only estimate.
func CellPixelsFor(cfg config.ImageViewerConfig, measured image.Point) (image.Point, bool) {
	if len(cfg.CellPixels) == 2 && cfg.CellPixels[0] > 0 && cfg.CellPixels[1] > 0 {
		return image.Pt(clampCellPixels(cfg.CellPixels[0]), clampCellPixels(cfg.CellPixels[1])), true
	}
	if cfg.CellAspect > 0 {
		h := defaultCellPixels.Y
		return image.Pt(clampCellPixels(int(math.Round(float64(h)*cfg.CellAspect))), h), false
	}
	if measured.X > 0 && measured.Y > 0 {
		return image.Pt(clampCellPixels(measured.X), clampCellPixels(measured.Y)), true
	}
	return defaultCellPixels, false
}
