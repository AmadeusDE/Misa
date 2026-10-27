package gui

import "image"

// canvas is a premultiplied ARGB8888 pixel buffer (B, G, R, A in memory).
// Opaque windows hand it to the compositor as XRGB8888, which ignores A.
type canvas struct {
	pix    []byte
	w, h   int
	stride int
}

func (c *canvas) bounds() image.Rectangle { return image.Rect(0, 0, c.w, c.h) }

func (c *canvas) fill(r image.Rectangle, col rgb) { c.fillAlpha(r, col, 0xff) }

// fillAlpha covers r with col at opacity a.
func (c *canvas) fillAlpha(r image.Rectangle, col rgb, a uint8) {
	b, g, red := premultiply(col.b, a), premultiply(col.g, a), premultiply(col.r, a)
	r = r.Intersect(c.bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := c.pix[y*c.stride+r.Min.X*4 : y*c.stride+r.Max.X*4]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = b, g, red, a
		}
	}
}

func premultiply(v, a uint8) uint8 { return uint8((int(v)*int(a) + 127) / 255) }

// blend draws opaque col through mask, whose origin lands at at, inside clip.
func (c *canvas) blend(mask *image.Alpha, at image.Point, clip image.Rectangle, col rgb) {
	r := mask.Rect.Add(at).Intersect(clip).Intersect(c.bounds())
	src := [4]int{int(col.b), int(col.g), int(col.r), 255}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		base := (y-at.Y-mask.Rect.Min.Y)*mask.Stride - mask.Rect.Min.X - at.X
		px := c.pix[y*c.stride:]
		for x := r.Min.X; x < r.Max.X; x++ {
			a := int(mask.Pix[base+x])
			if a == 0 {
				continue
			}
			i := x * 4
			if a == 255 {
				px[i], px[i+1], px[i+2], px[i+3] = col.b, col.g, col.r, 0xff
				continue
			}
			for k, s := range src {
				px[i+k] = uint8((int(px[i+k])*(255-a) + s*a) / 255)
			}
		}
	}
}
