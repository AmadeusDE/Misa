package imageframe

import (
	"image"
	"math"
	"testing"
)

func TestMapPixels(t *testing.T) {
	v := Viewport{Cells: image.Rect(10, 20, 14, 23), PixelSize: image.Pt(32, 48), CellPixels: image.Pt(8, 16)}
	tests := []struct {
		name          string
		v             Viewport
		rect          image.Rectangle
		pixels, cells image.Rectangle
		ok            bool
	}{
		{"offset", v, image.Rect(7, 15, 18, 33), image.Rect(7, 15, 18, 33), image.Rect(10, 20, 13, 23), true},
		{"clipped", v, image.Rect(-20, 16, 40, 80), image.Rect(0, 16, 32, 48), image.Rect(10, 21, 14, 23), true},
		{"outside", v, image.Rect(32, 0, 40, 10), image.Rectangle{}, image.Rectangle{}, false},
		{"empty", v, image.Rect(0, 0, 0, 1), image.Rectangle{}, image.Rectangle{}, false},
		{"zero metrics", Viewport{Cells: v.Cells, PixelSize: v.PixelSize}, image.Rect(0, 0, 1, 1), image.Rectangle{}, image.Rectangle{}, false},
		{"tiny host", Viewport{Cells: image.Rect(-3, 2, -2, 3), PixelSize: image.Pt(100, 100), CellPixels: image.Pt(8, 16)}, image.Rect(7, 15, 30, 30), image.Rect(7, 15, 8, 16), image.Rect(-3, 2, -2, 3), true},
		{"large metrics", Viewport{Cells: image.Rect(0, 0, 2, 2), PixelSize: image.Pt(math.MaxInt, math.MaxInt), CellPixels: image.Pt(math.MaxInt, math.MaxInt)}, image.Rect(0, 0, math.MaxInt, math.MaxInt), image.Rect(0, 0, math.MaxInt, math.MaxInt), image.Rect(0, 0, 1, 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := MapPixels(tt.v, tt.rect)
			if ok != tt.ok || (ok && (got.Pixels != tt.pixels || got.Cells != tt.cells)) {
				t.Fatalf("got %+v, %v", got, ok)
			}
		})
	}
}

func TestMapPixelsBounds(t *testing.T) {
	for w := 0; w < 5; w++ {
		for h := 0; h < 5; h++ {
			v := Viewport{Cells: image.Rect(-2, 3, -2+w, 3+h), PixelSize: image.Pt(15, 17), CellPixels: image.Pt(4, 5)}
			for x := -6; x < 20; x++ {
				for y := -6; y < 20; y++ {
					got, ok := MapPixels(v, image.Rect(x, y, x+3, y+3))
					if !ok {
						continue
					}
					if got.Cells.Empty() || !got.Cells.In(v.Cells) || got.Pixels.Empty() || !got.Pixels.In(image.Rect(0, 0, 15, 17)) {
						t.Fatalf("viewport %+v: %+v", v, got)
					}
				}
			}
		}
	}
}

func TestMapPixelsRejectsOverflowingCellSpan(t *testing.T) {
	v := Viewport{Cells: image.Rectangle{Min: image.Pt(math.MinInt, 0), Max: image.Pt(math.MaxInt, 1)}, PixelSize: image.Pt(1, 1), CellPixels: image.Pt(1, 1)}
	if _, ok := MapPixels(v, image.Rect(0, 0, 1, 1)); ok {
		t.Fatal("overflowing cell span accepted")
	}
}
