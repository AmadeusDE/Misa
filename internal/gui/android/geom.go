package android

import "math"

// Insets are the system bars and the IME, in surface pixels.
type Insets struct {
	Left, Top, Right, Bottom int
}

// Geometry is one surface configuration. Width and Height are the full
// surface in pixels. Density is Android's density (dpi/160); the core uses
// it as the buffer scale, so a configured point size grows with the display.
// Insets shrink the grid. They do not change the window buffer size.
type Geometry struct {
	Width, Height int
	Density       float64
	Insets        Insets
}

func (g Geometry) clamped() Geometry {
	if g.Width < 0 {
		g.Width = 0
	}
	if g.Height < 0 {
		g.Height = 0
	}
	if g.Insets.Left < 0 {
		g.Insets.Left = 0
	}
	if g.Insets.Top < 0 {
		g.Insets.Top = 0
	}
	if g.Insets.Right < 0 {
		g.Insets.Right = 0
	}
	if g.Insets.Bottom < 0 {
		g.Insets.Bottom = 0
	}
	return g
}

// Scale is the core buffer scale for this display. A non-positive density
// is mdpi.
func (g Geometry) Scale() float64 {
	if g.Density <= 0 {
		return 1
	}
	return g.Density
}

// Content is the pixel box inside the insets.
func (g Geometry) Content() (w, h int) {
	g = g.clamped()
	w = g.Width - g.Insets.Left - g.Insets.Right
	h = g.Height - g.Insets.Top - g.Insets.Bottom
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}

// Logical is the size and scale passed to the core. Integer densities use
// integer division, so the core's pixel size (logical × scale) never
// exceeds the content box. Other densities round.
func (g Geometry) Logical() (w, h int, scale float64) {
	cw, ch := g.Content()
	scale = g.Scale()
	if scale == float64(int(scale)) {
		s := int(scale)
		return cw / s, ch / s, scale
	}
	return int(math.Round(float64(cw) / scale)), int(math.Round(float64(ch) / scale)), scale
}

// LogicalPoint maps a surface pixel to the core's logical coordinate,
// with the inset as the origin.
func (g Geometry) LogicalPoint(x, y float64) (float64, float64) {
	g = g.clamped()
	s := g.Scale()
	return (x - float64(g.Insets.Left)) / s, (y - float64(g.Insets.Top)) / s
}

// FontPx is the glyph scale the core applies: the configured point size
// times density. The point size itself stays put; density is the buffer scale.
func FontPx(points, density float64) float64 {
	if density <= 0 {
		density = 1
	}
	if points <= 0 {
		points = 11
	}
	return points * density
}

// Grid is the terminal size for a content box in pixels. It matches the
// core: padding on every side, then whole cells, and at least 1×1.
func Grid(contentW, contentH, cellW, cellH, pad int) (rows, cols int) {
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}
	if pad < 0 {
		pad = 0
	}
	return max(1, (contentH-2*pad)/cellH), max(1, (contentW-2*pad)/cellW)
}
