package imageout

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"

	"github.com/AmadeusDE/Misa/internal/imageframe"
	xdraw "golang.org/x/image/draw"
)

// HalfBlockRenderer renders a scene as ordinary truecolor cells, without terminal I/O.
// Transparent pixels are composited over the supplied background. A renderer is
// stateless and can be shared; Release and Close have no resources to delete.
type HalfBlockRenderer struct{ background color.NRGBA }

// NewHalfBlock creates a renderer with a solid canvas background. Background
// alpha is composited over black, since terminal cell colors must be opaque.
func NewHalfBlock(background color.NRGBA) *HalfBlockRenderer {
	return &HalfBlockRenderer{background: background}
}

func (*HalfBlockRenderer) Capabilities() imageframe.Capabilities { return imageframe.Capabilities{} }
func (*HalfBlockRenderer) Release(owner uint64) imageframe.Frame {
	return imageframe.Frame{Owner: owner}
}
func (*HalfBlockRenderer) Close() imageframe.Frame { return imageframe.Frame{} }

// These limits bound both output allocation and the amount of overlapping
// placement work. Coordinates are also bounded before any arithmetic.
const halfMaxCells = 1 << 20
const halfMaxWork = 16 << 20
const halfMaxCoordinate = 1 << 20
const halfMaxPlacements = 1024

func boundedRect(r image.Rectangle) bool {
	return r.Min.X >= -halfMaxCoordinate && r.Min.Y >= -halfMaxCoordinate && r.Max.X <= halfMaxCoordinate && r.Max.Y <= halfMaxCoordinate
}

// Prepare replaces the complete viewport cell rectangle, including blank areas.
// PixelSize and CellPixels define the physical aspect ratio; one cell is
// sampled as one column by two rows. Destination and clip are viewport-local.
// Placements are composited in order with source-over alpha. Scaling uses
// ApproxBiLinear. Raster pixels remain owned by the caller and are not modified.
func (h *HalfBlockRenderer) Prepare(ctx context.Context, s imageframe.Scene) (imageframe.Frame, error) {
	if err := ctx.Err(); err != nil {
		return imageframe.Frame{}, err
	}
	f := imageframe.Frame{Owner: s.Owner, Generation: s.Generation, Presentation: s.Presentation}
	v := s.Viewport
	if v.Cells.Empty() {
		f.Cells = &imageframe.CellLayer{Bounds: v.Cells}
		return f, nil
	}
	if !boundedRect(v.Cells) || v.PixelSize.X <= 0 || v.PixelSize.Y <= 0 || v.PixelSize.X > halfMaxCoordinate || v.PixelSize.Y > halfMaxCoordinate || v.CellPixels.X <= 0 || v.CellPixels.Y <= 0 || v.CellPixels.X > halfMaxCoordinate || v.CellPixels.Y > halfMaxCoordinate || len(s.Placements) > halfMaxPlacements {
		return imageframe.Frame{}, errors.New("halfblock: invalid or oversized viewport")
	}
	w, height := v.Cells.Dx(), v.Cells.Dy()
	if w > halfMaxCells/height {
		return imageframe.Frame{}, errors.New("halfblock: too many cells")
	}
	// A pair of sample rows is used per cell. Drawing into RGBA performs
	// premultiplied alpha composition; cells below are converted back to NRGBA.
	canvas := image.NewRGBA(image.Rect(0, 0, w, 2*height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(h.background), image.Point{}, draw.Over)
	// Map pixel edges into the sample grid, rounding to nearest sample edge.
	// Coordinates and products are bounded above, independently of platform int.
	scale := func(n, mul, den int) int { // signed rounding
		p := int64(n) * int64(mul)
		d := int64(den)
		if p < 0 {
			return int(-((-p + d/2) / d))
		}
		return int((p + d/2) / d)
	}
	mapRect := func(r image.Rectangle) image.Rectangle {
		return image.Rect(scale(r.Min.X, 1, v.CellPixels.X), scale(r.Min.Y, 2, v.CellPixels.Y), scale(r.Max.X, 1, v.CellPixels.X), scale(r.Max.Y, 2, v.CellPixels.Y))
	}
	validPixels := mapRect(image.Rectangle{Max: v.PixelSize}).Intersect(canvas.Bounds())
	work := 0
	for _, p := range s.Placements {
		if err := ctx.Err(); err != nil {
			return imageframe.Frame{}, err
		}
		if p.Raster == nil || p.Raster.Pixels == nil || p.Source.Empty() || p.Dest.Empty() || p.Clip.Empty() {
			continue
		}
		if !boundedRect(p.Dest) || !boundedRect(p.Clip) || !boundedRect(p.Source) || !p.Source.In(p.Raster.Pixels.Bounds()) {
			return imageframe.Frame{}, errors.New("halfblock: invalid placement")
		}
		dst := mapRect(p.Dest)
		clip := mapRect(p.Clip).Intersect(validPixels).Intersect(dst)
		if dst.Empty() || clip.Empty() {
			continue
		}
		area := clip.Dx() * clip.Dy()
		if area > halfMaxWork-work {
			return imageframe.Frame{}, errors.New("halfblock: placement work limit exceeded")
		}
		work += area
		xdraw.ApproxBiLinear.Scale(canvas.SubImage(clip).(*image.RGBA), dst, p.Raster.Pixels, p.Source, draw.Over, nil)
	}
	cells := make([]imageframe.Cell, w*height)
	for y := 0; y < height; y++ {
		if err := ctx.Err(); err != nil {
			return imageframe.Frame{}, err
		}
		for x := 0; x < w; x++ {
			top := color.NRGBAModel.Convert(canvas.At(x, 2*y)).(color.NRGBA)
			bottom := color.NRGBAModel.Convert(canvas.At(x, 2*y+1)).(color.NRGBA)
			cells[y*w+x] = imageframe.Cell{Text: "▀", Foreground: top, Background: bottom}
		}
	}
	f.Cells = &imageframe.CellLayer{Bounds: v.Cells, Cells: cells}
	return f, nil
}
