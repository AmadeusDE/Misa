package imageframe

import (
	"image"
	"math"
)

// Mapping holds the clipped local pixels and the absolute host cells that cover
// them. Pixels may occupy only part of the outermost cells.
type Mapping struct {
	Pixels image.Rectangle
	Cells  image.Rectangle
}

// MapPixels clips a viewport-local pixel rectangle to both pixel and host-cell
// bounds. Empty or invalid viewports and rectangles have no mapping.
func MapPixels(v Viewport, r image.Rectangle) (Mapping, bool) {
	if v.Cells.Empty() || r.Empty() || v.PixelSize.X <= 0 || v.PixelSize.Y <= 0 || v.CellPixels.X <= 0 || v.CellPixels.Y <= 0 {
		return Mapping{}, false
	}
	width, okX := span(v.Cells.Min.X, v.Cells.Max.X)
	height, okY := span(v.Cells.Min.Y, v.Cells.Max.Y)
	if !okX || !okY {
		return Mapping{}, false
	}
	limit := image.Pt(min(v.PixelSize.X, saturatingProduct(width, v.CellPixels.X)), min(v.PixelSize.Y, saturatingProduct(height, v.CellPixels.Y)))
	r = r.Intersect(image.Rectangle{Max: limit})
	if r.Empty() {
		return Mapping{}, false
	}
	// Ceiling without adding cell size, which could overflow.
	ceil := func(n, d int) int { return 1 + (n-1)/d }
	cells := image.Rect(v.Cells.Min.X+r.Min.X/v.CellPixels.X, v.Cells.Min.Y+r.Min.Y/v.CellPixels.Y, v.Cells.Min.X+ceil(r.Max.X, v.CellPixels.X), v.Cells.Min.Y+ceil(r.Max.Y, v.CellPixels.Y))
	return Mapping{Pixels: r, Cells: cells}, true
}

func span(a, b int) (int, bool) {
	n := uint(b) - uint(a)
	if n > uint(math.MaxInt) {
		return 0, false
	}
	return int(n), true
}

func saturatingProduct(a, b int) int {
	if a > math.MaxInt/b {
		return math.MaxInt
	}
	return a * b
}
