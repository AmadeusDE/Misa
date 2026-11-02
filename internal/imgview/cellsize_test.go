package imgview

import (
	"image"
	"testing"

	"github.com/AmadeusDE/Misa/internal/config"
)

func TestCellPixelsFor(t *testing.T) {
	measured := image.Pt(9, 20)
	aspect := func(a float64) config.ImageViewerConfig {
		c := config.DefaultImageViewer()
		c.CellAspect = a
		return c
	}
	exact := func(w, h int) config.ImageViewerConfig {
		c := config.DefaultImageViewer()
		c.CellPixels = []int{w, h}
		return c
	}
	cases := []struct {
		name     string
		cfg      config.ImageViewerConfig
		measured image.Point
		want     image.Point
		exact    bool
	}{
		{"default", config.DefaultImageViewer(), image.Point{}, image.Pt(10, 20), false},
		{"measured", config.DefaultImageViewer(), measured, measured, true},
		{"aspect beats measurement", aspect(0.45), measured, image.Pt(9, 20), false},
		{"aspect rounds", aspect(0.55), image.Point{}, image.Pt(11, 20), false},
		{"aspect clamps", aspect(100), image.Point{}, image.Pt(maxCellPixels, 20), false},
		{"pixels beat aspect", exact(12, 24), measured, image.Pt(12, 24), true},
		{"pixels clamp", exact(9000, 3), image.Point{}, image.Pt(maxCellPixels, 3), true},
		{"invalid pixels fall back", exact(0, 20), measured, measured, true},
		{"invalid measurement", config.DefaultImageViewer(), image.Pt(-4, 9000), image.Pt(10, 20), false},
		{"one axis only", config.DefaultImageViewer(), image.Pt(0, 20), image.Pt(10, 20), false},
	}
	for _, c := range cases {
		got, isExact := CellPixelsFor(c.cfg, c.measured)
		if got != c.want || isExact != c.exact {
			t.Errorf("%s: CellPixelsFor = %v exact %v, want %v exact %v", c.name, got, isExact, c.want, c.exact)
		}
	}
}

func TestSetCellSizeBoundsAndDefault(t *testing.T) {
	v, err := New([]string{"one"}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.CellPixels(); got != defaultCellPixels {
		t.Fatalf("default cell %v", got)
	}
	v.SetCellSize(image.Pt(9, 20), true)
	if got := v.CellPixels(); got != image.Pt(9, 20) {
		t.Fatalf("cell %v", got)
	}
	v.SetCellSize(image.Pt(0, 20), true)
	v.SetCellSize(image.Pt(9, -1), false)
	if got := v.CellPixels(); got != image.Pt(9, 20) {
		t.Fatalf("invalid size replaced cell: %v", got)
	}
	v.SetCellSize(image.Pt(9000, 9000), true)
	if got := v.CellPixels(); got != image.Pt(maxCellPixels, maxCellPixels) {
		t.Fatalf("cell not clamped: %v", got)
	}
}

func TestMeasureCellPixelsNeverInventsSize(t *testing.T) {
	// Test output is not a terminal with pixel dimensions, so the zero point
	// means unknown; a real terminal may report a plausible size instead.
	p := MeasureCellPixels()
	if p.X < 0 || p.Y < 0 {
		t.Fatalf("negative measurement %v", p)
	}
	if (p.X == 0) != (p.Y == 0) {
		t.Fatalf("half a measurement: %v", p)
	}
}
