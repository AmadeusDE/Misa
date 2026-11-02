package imgview

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
	xdraw "golang.org/x/image/draw"
)

// orientedImage maps the transformed source lazily, avoiding an allocation at
// source resolution when only a small viewport is visible.
type orientedImage struct {
	src          *image.NRGBA
	rotation     int
	flipX, flipY bool
}

func (o orientedImage) ColorModel() color.Model { return color.NRGBAModel }
func (o orientedImage) Bounds() image.Rectangle {
	s := o.src.Bounds().Size()
	if o.rotation%2 != 0 {
		s.X, s.Y = s.Y, s.X
	}
	return image.Rectangle{Max: s}
}
func (o orientedImage) At(x, y int) color.Color {
	w, h := o.src.Bounds().Dx(), o.src.Bounds().Dy()
	a, b := x, y
	switch o.rotation {
	case 1:
		a, b = y, h-1-x
	case 2:
		a, b = w-1-x, h-1-y
	case 3:
		a, b = w-1-y, x
	}
	if o.flipX {
		a = w - 1 - a
	}
	if o.flipY {
		b = h - 1 - b
	}
	return o.src.NRGBAAt(o.src.Rect.Min.X+a, o.src.Rect.Min.Y+b)
}

// ApproxBiLinear's RGBA64 destination path requires an RGBA64Image source;
// without this method x/image/draw silently skips scaled writes to NRGBA.
func (o orientedImage) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := o.At(x, y).RGBA()
	return color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: uint16(a)}
}

const standaloneMaxPixels = 16 << 20

func standaloneRaster(v *Viewer) (*image.NRGBA, image.Point, bool) {
	p, offset, _, ok := standaloneRasterLimited(v, kittygfx.DefaultLimits())
	return p, offset, ok
}

func standaloneRasterLimited(v *Viewer, limits kittygfx.Limits) (*image.NRGBA, image.Point, image.Point, bool) {
	limits = limits.Normalize()
	if v.help || v.grid || len(v.items) == 0 || v.items[v.index].err != nil {
		return nil, image.Point{}, image.Point{}, false
	}
	pix := v.pixels(v.index)
	if pix == nil {
		return nil, image.Point{}, image.Point{}, false
	}
	it := v.items[v.index]
	src := orientedImage{pix, it.rotation, it.flipX, it.flipY}
	viewport := image.Rectangle{Max: v.viewportPixels()}
	if viewport.Empty() {
		return nil, image.Point{}, image.Point{}, false
	}
	dest := imageDestination(src.Bounds().Size(), viewport, v.scale(), v.pan)
	visible := dest.Intersect(viewport)
	if visible.Empty() {
		return nil, image.Point{}, image.Point{}, false
	}
	w, h := visible.Dx(), visible.Dy()
	maxBytes := min(int64(standaloneMaxPixels)*4, limits.MaxImageBytes, limits.MaxTerminalBytes, (limits.MaxInFlightBytes/4)*3)
	maxPixels := maxBytes / 4
	if maxPixels < 1 {
		return nil, image.Point{}, image.Point{}, false
	}
	ratio := math.Min(1, math.Min(float64(min(16384, limits.MaxWidth))/float64(w), float64(min(16384, limits.MaxHeight))/float64(h)))
	ratio = math.Min(ratio, math.Sqrt(float64(maxPixels)/float64(w)/float64(h)))
	ow, oh := max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))
	// Rounding a very narrow image up to one pixel can exceed the area cap.
	if int64(ow)*int64(oh) > maxPixels {
		if ow > oh {
			ow = int(maxPixels / int64(oh))
		} else {
			oh = int(maxPixels / int64(ow))
		}
	}
	if ow <= 0 || oh <= 0 {
		return nil, image.Point{}, image.Point{}, false
	}
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	rel := dest.Sub(visible.Min)
	sx := func(x int) int { return int(math.Round(float64(x) * float64(ow) / float64(w))) }
	sy := func(y int) int { return int(math.Round(float64(y) * float64(oh) / float64(h))) }
	scaled := image.Rect(sx(rel.Min.X), sy(rel.Min.Y), sx(rel.Max.X), sy(rel.Max.Y))
	xdraw.ApproxBiLinear.Scale(out, scaled, src, src.Bounds(), draw.Src, nil)
	return out, visible.Min, visible.Size(), true
}
