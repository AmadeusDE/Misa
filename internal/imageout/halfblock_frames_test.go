package imageout

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

// A half-block animation re-renders the same owner, raster and placement IDs
// with new pixels for each frame, so identity must never be treated as content.
func TestHalfBlockRerendersFramesOfStableResources(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	viewport := imageframe.Viewport{Cells: image.Rect(0, 0, 2, 1), PixelSize: image.Pt(2, 2), CellPixels: image.Pt(1, 2)}
	scene := func(c color.NRGBA, presentation imageframe.Presentation) imageframe.Scene {
		return imageframe.Scene{Owner: 7, Generation: 3, Viewport: viewport, Presentation: presentation, Placements: []imageframe.Placement{{
			ID: 4, Raster: &imageframe.Raster{ID: 4, Generation: 1, Pixels: solidPixels(c, 2, 2)},
			Source: image.Rect(0, 0, 2, 2), Dest: image.Rect(0, 0, 2, 2), Clip: image.Rect(0, 0, 2, 2),
		}}}
	}
	r := NewHalfBlock(color.NRGBA{A: 255})
	first, err := r.Prepare(context.Background(), scene(red, imageframe.Presentation{SequenceID: 9, FrameIndex: 0, Generation: 1}))
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Prepare(context.Background(), scene(blue, imageframe.Presentation{SequenceID: 9, FrameIndex: 1, Generation: 2}))
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range []imageframe.Frame{first, second} {
		want := red
		if i == 1 {
			want = blue
		}
		for _, c := range f.Cells.Cells {
			if c.Foreground != want || c.Background != want {
				t.Fatalf("frame %d cell %+v, want %v", i, c, want)
			}
		}
		if f.Presentation.FrameIndex != i || f.Presentation.SequenceID != 9 || f.Presentation.Generation != uint64(i+1) {
			t.Fatalf("frame %d presentation %+v", i, f.Presentation)
		}
	}
	if first.Cells.Cells[0] == second.Cells.Cells[0] {
		t.Fatal("later frame re-used earlier cells")
	}
}

// Every frame must be a complete render, independent of the previous frame, or
// transparent and resized frames would leave the earlier picture behind.
func TestHalfBlockFramesCarryNoState(t *testing.T) {
	viewport := func(w, h int) imageframe.Viewport {
		return imageframe.Viewport{Cells: image.Rect(0, 0, w, h), PixelSize: image.Pt(w, 2*h), CellPixels: image.Pt(1, 2)}
	}
	opaque := func(w, h int) imageframe.Scene {
		return imageframe.Scene{Owner: 1, Generation: 1, Viewport: viewport(w, h), Placements: []imageframe.Placement{{
			ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: solidPixels(color.NRGBA{R: 255, A: 255}, w, 2*h)},
			Source: image.Rect(0, 0, w, 2*h), Dest: image.Rect(0, 0, w, 2*h), Clip: image.Rect(0, 0, w, 2*h),
		}}}
	}
	clear := func(w, h int) imageframe.Scene {
		return imageframe.Scene{Owner: 1, Generation: 1, Viewport: viewport(w, h), Placements: []imageframe.Placement{{
			ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: solidPixels(color.NRGBA{}, w, 2*h)},
			Source: image.Rect(0, 0, w, 2*h), Dest: image.Rect(0, 0, w, 2*h), Clip: image.Rect(0, 0, w, 2*h),
		}}}
	}
	background := color.NRGBA{G: 40, A: 255}
	r := NewHalfBlock(background)
	for _, s := range []imageframe.Scene{opaque(3, 2), clear(3, 2), opaque(1, 1), clear(1, 1)} {
		got, err := r.Prepare(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		want, err := NewHalfBlock(background).Prepare(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Cells.Cells) != len(want.Cells.Cells) || got.Cells.Bounds != want.Cells.Bounds {
			t.Fatalf("layer %v/%d, want %v/%d", got.Cells.Bounds, len(got.Cells.Cells), want.Cells.Bounds, len(want.Cells.Cells))
		}
		for i := range want.Cells.Cells {
			if got.Cells.Cells[i] != want.Cells.Cells[i] {
				t.Fatalf("cell %d is %+v, want %+v", i, got.Cells.Cells[i], want.Cells.Cells[i])
			}
		}
	}
}

// Prepared frames are retained by hosts, so a later frame must not share or
// overwrite their cells, and rendering must not modify the caller's pixels.
func TestHalfBlockFramesOwnTheirCellsAndPixels(t *testing.T) {
	pix := solidPixels(color.NRGBA{R: 200, A: 255}, 1, 2)
	pix.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	pix.SetNRGBA(0, 1, color.NRGBA{R: 40, G: 50, B: 60, A: 255})
	before := append([]byte(nil), pix.Pix...)
	s := imageframe.Scene{Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, 1, 1), PixelSize: image.Pt(1, 2), CellPixels: image.Pt(1, 2)}, Placements: []imageframe.Placement{{
		ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: pix}, Source: image.Rect(0, 0, 1, 2), Dest: image.Rect(0, 0, 1, 2), Clip: image.Rect(0, 0, 1, 2),
	}}}
	r := NewHalfBlock(color.NRGBA{A: 255})
	first, err := r.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cells.Cells[0] != second.Cells.Cells[0] {
		t.Fatalf("repeated frame changed: %+v %+v", first.Cells.Cells[0], second.Cells.Cells[0])
	}
	second.Cells.Cells[0].Foreground = color.NRGBA{}
	if first.Cells.Cells[0].Foreground != (color.NRGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Fatalf("frames share cells: %+v", first.Cells.Cells[0])
	}
	for i := range before {
		if pix.Pix[i] != before[i] {
			t.Fatalf("raster pixel %d modified: %d, want %d", i, pix.Pix[i], before[i])
		}
	}
}

// Cell colors are read from an opaque canvas, so a translucent background must
// composite exactly like its already composited opaque equivalent.
func TestHalfBlockTranslucentBackgroundStaysOpaque(t *testing.T) {
	s := imageframe.Scene{Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, 2, 2), PixelSize: image.Pt(2, 4), CellPixels: image.Pt(1, 2)}}
	got, err := NewHalfBlock(color.NRGBA{G: 100, A: 128}).Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	want, err := NewHalfBlock(color.NRGBA{G: 50, A: 255}).Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want.Cells.Cells {
		if got.Cells.Cells[i] != want.Cells.Cells[i] || got.Cells.Cells[i].Foreground.A != 255 || got.Cells.Cells[i].Background.A != 255 {
			t.Fatalf("cell %d is %+v, want opaque %+v", i, got.Cells.Cells[i], want.Cells.Cells[i])
		}
	}
}

func TestHalfBlockFrameSequenceMatchesSingleFrameRenders(t *testing.T) {
	scenes := []imageframe.Scene{
		{Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, 2, 2), PixelSize: image.Pt(8, 8), CellPixels: image.Pt(4, 4)}, Placements: []imageframe.Placement{{
			ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: solidPixels(color.NRGBA{R: 255, A: 255}, 4, 4)}, Source: image.Rect(0, 0, 4, 4), Dest: image.Rect(0, 0, 8, 8), Clip: image.Rect(0, 0, 8, 8),
		}}},
		{Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, 2, 2), PixelSize: image.Pt(8, 8), CellPixels: image.Pt(4, 4)}, Placements: []imageframe.Placement{{
			ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: solidPixels(color.NRGBA{B: 255, A: 255}, 4, 4)}, Source: image.Rect(0, 0, 4, 4), Dest: image.Rect(-4, 4, 4, 12), Clip: image.Rect(0, 0, 8, 8),
		}}},
		{Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, 1, 1), PixelSize: image.Pt(4, 2), CellPixels: image.Pt(4, 2)}, Placements: []imageframe.Placement{{
			ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: solidPixels(color.NRGBA{G: 255, A: 255}, 4, 4)}, Source: image.Rect(0, 0, 4, 4), Dest: image.Rect(0, 0, 4, 2), Clip: image.Rect(0, 0, 4, 2),
		}}},
	}
	r := NewHalfBlock(color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	for i, s := range scenes {
		got, err := r.Prepare(context.Background(), s)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		want, err := NewHalfBlock(color.NRGBA{R: 1, G: 2, B: 3, A: 255}).Prepare(context.Background(), s)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if got.Cells.Bounds != want.Cells.Bounds || len(got.Cells.Cells) != len(want.Cells.Cells) {
			t.Fatalf("frame %d layer mismatch", i)
		}
		for j := range want.Cells.Cells {
			if got.Cells.Cells[j] != want.Cells.Cells[j] {
				t.Fatalf("frame %d cell %d is %+v, want %+v", i, j, got.Cells.Cells[j], want.Cells.Cells[j])
			}
		}
	}
}

func TestHalfBlockCanceledFrameKeepsRendererUsable(t *testing.T) {
	s := imageframe.Scene{Viewport: imageframe.Viewport{Cells: image.Rect(0, 0, 1, 1), PixelSize: image.Pt(1, 2), CellPixels: image.Pt(1, 2)}, Placements: []imageframe.Placement{{
		ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: solidPixels(color.NRGBA{R: 9, A: 255}, 1, 2)}, Source: image.Rect(0, 0, 1, 2), Dest: image.Rect(0, 0, 1, 2), Clip: image.Rect(0, 0, 1, 2),
	}}}
	r := NewHalfBlock(color.NRGBA{A: 255})
	if _, err := r.Prepare(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Prepare(ctx, s); err != context.Canceled {
		t.Fatalf("canceled frame: %v", err)
	}
	f, err := r.Prepare(context.Background(), s)
	if err != nil || f.Cells.Cells[0].Foreground != (color.NRGBA{R: 9, A: 255}) {
		t.Fatalf("frame after cancellation: %+v %v", f.Cells, err)
	}
}

// BenchmarkHalfBlockFrame measures one animation frame for a 200x60 cell pane
// with 8x16 pixel cells and a 800x600 source.
func BenchmarkHalfBlockFrame(b *testing.B) {
	viewport := imageframe.Viewport{Cells: image.Rect(0, 0, 200, 60), PixelSize: image.Pt(1600, 1920), CellPixels: image.Pt(8, 16)}
	pix := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			pix.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: 255})
		}
	}
	s := imageframe.Scene{Viewport: viewport, Placements: []imageframe.Placement{{
		ID: 1, Raster: &imageframe.Raster{ID: 1, Pixels: pix}, Source: pix.Bounds(), Dest: image.Rect(400, 480, 1200, 1440), Clip: image.Rect(0, 0, 1600, 1920),
	}}}
	r := NewHalfBlock(color.NRGBA{A: 255})
	b.ReportAllocs()
	for b.Loop() {
		if _, err := r.Prepare(context.Background(), s); err != nil {
			b.Fatal(err)
		}
	}
}

func solidPixels(c color.NRGBA, w, h int) *image.NRGBA {
	p := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p.SetNRGBA(x, y, c)
		}
	}
	return p
}
