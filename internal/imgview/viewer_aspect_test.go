package imgview

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/imageframe"
	"github.com/AmadeusDE/Misa/internal/imageout"
)

// placementPixels is the on-screen size of a placement in physical pixels. A
// sample is one cell wide and half a cell tall, so the scene's local pixels only
// keep the image's shape when CellPixels describes the real cell.
func placementPixels(v imageframe.Viewport, p imageframe.Placement, cell image.Point) (float64, float64) {
	cols := float64(p.Dest.Dx()) / float64(v.CellPixels.X)
	halfRows := float64(p.Dest.Dy()) * 2 / float64(v.CellPixels.Y)
	return cols * float64(cell.X), halfRows * float64(cell.Y) / 2
}

func cellSizeViewer(t *testing.T, cell image.Point, thumbnail bool) *Viewer {
	t.Helper()
	v, err := New([]string{"one"}, thumbnail, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.SetCellSize(cell, true)
	v.Resize(60, 21)
	return v
}

func TestImageAspectSurvivesCellSize(t *testing.T) {
	cells := []image.Point{{8, 16}, {10, 18}, {9, 20}, {12, 24}}
	sizes := []image.Point{{100, 100}, {300, 100}, {100, 300}, {640, 480}}
	fits := []fitMode{fitWhole, fitNoUpscale, fitWidth, fitHeight, fitActual}
	for _, cell := range cells {
		for _, size := range sizes {
			for _, fit := range fits {
				v := cellSizeViewer(t, cell, false)
				v.fit = fit
				v.SetImage(0, image.NewNRGBA(image.Rectangle{Max: size}), nil)
				s := v.Scene(1, 1)
				if len(s.Placements) != 1 {
					t.Fatalf("cell %v size %v: %d placements", cell, size, len(s.Placements))
				}
				w, h := placementPixels(s.Viewport, s.Placements[0], cell)
				want := float64(size.X) / float64(size.Y)
				tolerance := math.Max(float64(cell.X), float64(cell.Y)/2)
				if math.Abs(w/h-want)*h > tolerance {
					t.Errorf("cell %v size %v fit %d: %vx%v physical, want ratio %v", cell, size, fit, w, h, want)
				}
			}
		}
	}
}

func TestSceneViewportUsesRealCellPixels(t *testing.T) {
	for _, cell := range []image.Point{{8, 16}, {9, 20}} {
		v := cellSizeViewer(t, cell, false)
		s := v.Scene(1, 1)
		if got, want := s.Viewport.PixelSize, image.Pt(60*cell.X, 20*cell.Y); got != want {
			t.Errorf("cell %v: PixelSize %v, want %v", cell, got, want)
		}
		if s.Viewport.CellPixels != cell || !s.Viewport.ExactPixels {
			t.Errorf("cell %v: viewport %+v", cell, s.Viewport)
		}
		if got, want := s.Viewport.Cells, image.Rect(0, 0, 60, 20); got != want {
			t.Errorf("cell %v: cells %v, want %v", cell, got, want)
		}
	}
}

func TestActualSizeIsOneImagePixelPerPhysicalPixel(t *testing.T) {
	v := cellSizeViewer(t, image.Pt(9, 20), false)
	v.fit = fitActual
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 100, 80)), nil)
	if got, want := v.Scene(1, 1).Placements[0].Dest.Size(), image.Pt(100, 80); got != want {
		t.Fatalf("actual size %v, want %v", got, want)
	}
}

func TestGridThumbnailsKeepAspect(t *testing.T) {
	for _, cell := range []image.Point{{8, 16}, {10, 18}, {9, 20}, {12, 24}} {
		for _, size := range []image.Point{{100, 100}, {300, 100}, {100, 300}} {
			v := cellSizeViewer(t, cell, true)
			v.SetImage(0, image.NewNRGBA(image.Rectangle{Max: size}), nil)
			s := v.Scene(1, 1)
			if len(s.Placements) != 1 {
				t.Fatalf("cell %v size %v: %d placements", cell, size, len(s.Placements))
			}
			w, h := placementPixels(s.Viewport, s.Placements[0], cell)
			want := float64(size.X) / float64(size.Y)
			tolerance := math.Max(float64(cell.X), float64(cell.Y)/2)
			if math.Abs(w/h-want)*h > tolerance {
				t.Errorf("cell %v size %v: %vx%v physical, want ratio %v", cell, size, w, h, want)
			}
		}
	}
}

func TestFallbackCellPixelsKeepAspect(t *testing.T) {
	v, err := New([]string{"one"}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(60, 21)
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 100, 100)), nil)
	s := v.Scene(1, 1)
	if s.Viewport.ExactPixels {
		t.Fatal("unmeasured viewport claims exact pixels")
	}
	w, h := placementPixels(s.Viewport, s.Placements[0], s.Viewport.CellPixels)
	if math.Abs(w-h) > float64(s.Viewport.CellPixels.Y)/2 {
		t.Fatalf("fallback cell %v: %vx%v physical", s.Viewport.CellPixels, w, h)
	}
}

func TestPanStepsFollowCellSize(t *testing.T) {
	v := cellSizeViewer(t, image.Pt(10, 20), false)
	v.fit = fitActual
	v.SetImage(0, image.NewNRGBA(image.Rect(0, 0, 1200, 1200)), nil)
	v.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	before := v.pan
	v.Update(key("l"))
	if got := v.pan.X - before.X; got != 4*10 {
		t.Fatalf("horizontal pan step %d, want 40", got)
	}
	v.Update(key("j"))
	if got := v.pan.Y - before.Y; got != 2*20 {
		t.Fatalf("vertical pan step %d, want 40", got)
	}
	v.Update(tea.MouseMotionMsg{X: 3, Y: 4})
	if got := v.pan.X - before.X; got != 40-3*10 {
		t.Fatalf("drag pan x %d", got)
	}
	if got := v.pan.Y - before.Y; got != 40-4*20 {
		t.Fatalf("drag pan y %d", got)
	}
}

// imageBounds is the cell rectangle a half-block frame draws the image in.
func imageBounds(t *testing.T, frame imageframe.Frame, bg color.NRGBA) image.Rectangle {
	t.Helper()
	layer := frame.Cells
	if layer == nil {
		t.Fatal("no cell layer")
	}
	b := image.Rectangle{}
	for y := 0; y < layer.Bounds.Dy(); y++ {
		for x := 0; x < layer.Bounds.Dx(); x++ {
			c := layer.Cells[y*layer.Bounds.Dx()+x]
			if c.Foreground == bg && c.Background == bg {
				continue
			}
			b = b.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return b
}

func TestHalfBlockCellsMatchImageAspect(t *testing.T) {
	bg := color.NRGBA{R: 32, G: 32, B: 32, A: 255}
	for _, cell := range []image.Point{{8, 16}, {10, 18}, {9, 20}, {12, 24}} {
		v := cellSizeViewer(t, cell, false)
		img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{R: 255, A: 255}), image.Point{}, draw.Src)
		v.SetImage(0, img, nil)
		frame, err := imageout.NewHalfBlock(bg).Prepare(context.Background(), v.Scene(1, 1))
		if err != nil {
			t.Fatal(err)
		}
		b := imageBounds(t, frame, bg)
		if b.Empty() {
			t.Fatalf("cell %v: no image cells", cell)
		}
		width, height := b.Dx()*cell.X, b.Dy()*cell.Y
		if delta := width - height; delta > max(cell.X, cell.Y) || -delta > max(cell.X, cell.Y) {
			t.Errorf("cell %v: image covers %dx%d physical pixels", cell, width, height)
		}
	}
}
