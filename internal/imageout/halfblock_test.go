package imageout

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

func TestHalfBlockCellsAndComposition(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	bg := color.NRGBA{G: 100, A: 255}
	pix := image.NewNRGBA(image.Rect(5, 7, 7, 9))
	pix.SetNRGBA(5, 7, red)
	pix.SetNRGBA(5, 8, blue)
	pix.SetNRGBA(6, 7, color.NRGBA{R: 200, A: 128})
	pix.SetNRGBA(6, 8, color.NRGBA{})
	s := imageframe.Scene{Owner: 3, Generation: 9, Presentation: imageframe.Presentation{Generation: 4}, Viewport: imageframe.Viewport{Cells: image.Rect(10, 20, 12, 21), PixelSize: image.Pt(16, 16), CellPixels: image.Pt(8, 16)}, Placements: []imageframe.Placement{{Raster: &imageframe.Raster{Pixels: pix}, Source: pix.Bounds(), Dest: image.Rect(0, 0, 16, 16), Clip: image.Rect(0, 0, 16, 16)}}}
	r := NewHalfBlock(bg)
	var _ imageframe.Renderer = r
	f, err := r.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if f.Owner != 3 || f.Generation != 9 || f.Presentation != s.Presentation || f.Cells.Bounds != s.Viewport.Cells || len(f.Cells.Cells) != 2 {
		t.Fatalf("frame: %+v", f)
	}
	a, b := f.Cells.Cells[0], f.Cells.Cells[1]
	if a.Text != "▀" || a.Foreground != red || a.Background != blue {
		t.Fatalf("first cell %+v", a)
	}
	if b.Foreground != (color.NRGBA{R: 100, G: 49, A: 255}) || b.Background != bg {
		t.Fatalf("alpha %+v", b)
	}
	if len(f.Effects) != 0 || r.Capabilities().Pixels || r.Release(3).Owner != 3 || len(r.Close().Effects) != 0 {
		t.Fatal("unexpected graphics resources")
	}
}

func TestHalfBlockClipLayeringAndBounds(t *testing.T) {
	bg := color.NRGBA{A: 255}
	red := color.NRGBA{R: 255, A: 255}
	green := color.NRGBA{G: 255, A: 255}
	solid := func(c color.NRGBA) *imageframe.Raster {
		p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		p.SetNRGBA(0, 0, c)
		return &imageframe.Raster{Pixels: p}
	}
	s := imageframe.Scene{Viewport: imageframe.Viewport{Cells: image.Rect(-1, 2, 1, 3), PixelSize: image.Pt(16, 16), CellPixels: image.Pt(8, 16)}, Placements: []imageframe.Placement{
		{Raster: solid(red), Source: image.Rect(0, 0, 1, 1), Dest: image.Rect(0, 0, 16, 16), Clip: image.Rect(0, 0, 8, 16)},
		{Raster: solid(green), Source: image.Rect(0, 0, 1, 1), Dest: image.Rect(0, 0, 16, 16), Clip: image.Rect(8, 8, 16, 16)},
	}}
	f, err := NewHalfBlock(bg).Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	a, b := f.Cells.Cells[0], f.Cells.Cells[1]
	if a.Foreground != red || a.Background != red || b.Foreground != bg || b.Background != green {
		t.Fatalf("cells %+v %+v", a, b)
	}
}

func TestHalfBlockValidationAndCancellation(t *testing.T) {
	r := NewHalfBlock(color.NRGBA{A: 255})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Prepare(ctx, imageframe.Scene{}); err != context.Canceled {
		t.Fatalf("cancel: %v", err)
	}
	v := imageframe.Viewport{Cells: image.Rect(0, 0, 1025, 1025), PixelSize: image.Pt(1, 1), CellPixels: image.Pt(1, 1)}
	if _, err := r.Prepare(context.Background(), imageframe.Scene{Viewport: v}); err == nil {
		t.Fatal("oversized viewport accepted")
	}
	v.Cells = image.Rect(0, 0, 1, 1)
	p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	s := imageframe.Scene{Viewport: v, Placements: []imageframe.Placement{{Raster: &imageframe.Raster{Pixels: p}, Source: image.Rect(0, 0, 2, 1), Dest: image.Rect(0, 0, 1, 1), Clip: image.Rect(0, 0, 1, 1)}}}
	if _, err := r.Prepare(context.Background(), s); err == nil {
		t.Fatal("invalid source accepted")
	}
}
