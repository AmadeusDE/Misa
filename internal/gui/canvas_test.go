package gui

import (
	"image"
	"testing"
)

func newCanvas(w, h int) *canvas {
	return &canvas{pix: make([]byte, w*h*4), w: w, h: h, stride: w * 4}
}

func (c *canvas) at(x, y int) rgb {
	i := y*c.stride + x*4
	return rgb{c.pix[i+2], c.pix[i+1], c.pix[i]}
}

func TestCanvasFillClips(t *testing.T) {
	c := newCanvas(4, 4)
	c.fill(image.Rect(2, 2, 10, 10), rgb{1, 2, 3})
	if c.at(3, 3) != (rgb{1, 2, 3}) || c.at(1, 1) != (rgb{}) {
		t.Fatalf("fill: %v %v", c.at(3, 3), c.at(1, 1))
	}
	if c.pix[(3*4+3)*4+3] != 0xff {
		t.Error("fill left the X byte unset")
	}
}

func TestCanvasBlendHonoursMaskOriginAndClip(t *testing.T) {
	c := newCanvas(6, 2)
	mask := image.NewAlpha(image.Rect(-1, 0, 2, 1))
	mask.Pix = []byte{255, 128, 255}
	c.blend(mask, image.Pt(2, 1), image.Rect(0, 0, 3, 2), rgb{255, 255, 255})
	if got := c.at(1, 1); got != (rgb{255, 255, 255}) {
		t.Errorf("x=1: %v", got)
	}
	if got := c.at(2, 1); got.r != 128 {
		t.Errorf("x=2 half blend: %v", got)
	}
	if got := c.at(3, 1); got != (rgb{}) {
		t.Errorf("x=3 outside clip: %v", got)
	}
}

func (c *canvas) alphaAt(x, y int) uint8 { return c.pix[y*c.stride+x*4+3] }

func TestCanvasTranslucentFillIsPremultiplied(t *testing.T) {
	c := newCanvas(1, 1)
	c.fillAlpha(c.bounds(), rgb{200, 100, 0}, 128)
	if got := c.at(0, 0); got != (rgb{100, 50, 0}) {
		t.Errorf("premultiplied = %v", got)
	}
	if a := c.alphaAt(0, 0); a != 128 {
		t.Errorf("alpha = %d", a)
	}
}

func TestBlendOverTransparentRaisesAlpha(t *testing.T) {
	c := newCanvas(2, 1)
	c.fillAlpha(c.bounds(), rgb{}, 0)
	mask := image.NewAlpha(image.Rect(0, 0, 2, 1))
	mask.Pix = []byte{255, 128}
	c.blend(mask, image.Point{}, c.bounds(), rgb{255, 255, 255})
	if c.alphaAt(0, 0) != 255 || c.at(0, 0) != (rgb{255, 255, 255}) {
		t.Errorf("full coverage = %v alpha %d", c.at(0, 0), c.alphaAt(0, 0))
	}
	if a := c.alphaAt(1, 0); a != 128 {
		t.Errorf("half coverage alpha = %d", a)
	}
	if got := c.at(1, 0); got.r != 128 {
		t.Errorf("half coverage stays premultiplied: %v", got)
	}
}
