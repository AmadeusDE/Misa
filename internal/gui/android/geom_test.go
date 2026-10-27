package android

import (
	"image"
	"testing"

	"github.com/AmadeusDE/Misa/internal/gui"
)

func TestGeometryInsetsDensityAndGrid(t *testing.T) {
	g := Geometry{Width: 1080, Height: 2400, Density: 3, Insets: Insets{Top: 80, Bottom: 120}}
	cw, ch := g.Content()
	if cw != 1080 || ch != 2200 {
		t.Fatalf("content %d×%d", cw, ch)
	}
	w, h, scale := g.Logical()
	if w != 360 || h != 733 || scale != 3 {
		t.Fatalf("logical %d×%d scale %v", w, h, scale)
	}
	if FontPx(11, 3) != 33 || FontPx(0, 0) != 11 {
		t.Fatalf("font scale %v %v", FontPx(11, 3), FontPx(0, 0))
	}
	rows, cols := Grid(cw, ch, 10, 20, 4)
	if rows != 109 || cols != 107 {
		t.Fatalf("grid %d×%d", rows, cols)
	}
	lx, ly := g.LogicalPoint(30, 86)
	if lx != 10 || ly != 2 {
		t.Fatalf("point %v,%v", lx, ly)
	}
}

func TestGeometryClampsAndNonIntegerDensity(t *testing.T) {
	g := Geometry{Width: 1000, Height: 500, Density: 2.5, Insets: Insets{Left: -4, Right: 10}}
	w, h, scale := g.Logical()
	// Content width is 1000-0-10 = 990; round(990/2.5) = 396. Height 500/2.5 = 200.
	if w != 396 || h != 200 || scale != 2.5 {
		t.Fatalf("logical %d×%d scale %v", w, h, scale)
	}
	covered := Geometry{Width: 40, Height: 20, Density: 1, Insets: Insets{Left: 30, Right: 30, Top: 20}}
	cw, ch := covered.Content()
	if cw != 0 || ch != 0 {
		t.Fatalf("covered content %d×%d", cw, ch)
	}
	rows, cols := Grid(0, 0, 8, 16, 4)
	if rows != 1 || cols != 1 {
		t.Fatalf("empty grid %d×%d", rows, cols)
	}
	if plain := (Geometry{Width: 80, Height: 40}).Scale(); plain != 1 {
		t.Fatalf("scale %v", plain)
	}
}

func TestDamageBounds(t *testing.T) {
	full, ok := DamageBounds(nil, 2, 3, 10, 8)
	if !ok || full != image.Rect(0, 0, 10, 8) {
		t.Fatalf("full %+v %v", full, ok)
	}
	u, ok := DamageBounds([]image.Rectangle{
		image.Rect(0, 0, 2, 1),
		image.Rect(1, 2, 4, 5),
	}, 3, 1, 10, 8)
	if !ok || u != image.Rect(3, 1, 7, 6) {
		t.Fatalf("union %+v", u)
	}
	if _, ok := DamageBounds([]image.Rectangle{image.Rect(0, 0, 0, 0)}, 0, 0, 4, 4); ok {
		t.Fatal("empty damage reported a region")
	}
}

func TestBlitStrideOriginAndSwap(t *testing.T) {
	const dstStride = 16
	dst := make([]byte, dstStride*2)
	for i := range dst {
		dst[i] = 9
	}
	src := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	Blit(dst, dstStride, 2, 2, src, 8, 2, 1, 0, 0, image.Rect(0, 0, 2, 1), false)
	if string(dst[0:8]) != string(src) {
		t.Fatalf("row %v", dst[0:8])
	}
	if dst[8] != 9 || dst[dstStride] != 9 {
		t.Fatalf("padding or next row changed: %v", dst)
	}

	dst = make([]byte, 4*3*3)
	one := []byte{10, 20, 30, 40}
	Blit(dst, 12, 3, 3, one, 4, 1, 1, 1, 1, image.Rect(0, 0, 3, 3), false)
	px := dst[(1*3+1)*4 : (1*3+1)*4+4]
	if string(px) != string(one) {
		t.Fatalf("origin pixel %v", px)
	}
	if dst[0] != 0 || dst[3] != 255 {
		t.Fatalf("outside pixel %v", dst[:4])
	}

	out := make([]byte, 4)
	Blit(out, 4, 1, 1, []byte{1, 2, 3, 4}, 4, 1, 1, 0, 0, image.Rect(0, 0, 1, 1), true)
	if out[0] != 3 || out[1] != 2 || out[2] != 1 || out[3] != 4 {
		t.Fatalf("swap %v", out)
	}
}

func TestBackendPresentsDamageAtOrigin(t *testing.T) {
	b := newBackend(nil)
	s := newMemSurface(4, 3)
	for i := range s.pix {
		s.pix[i] = 1
	}
	b.setSurface(s)
	b.setPresenting(true)
	b.winW, b.winH = 4, 3
	b.originX, b.originY = 1, 1
	buf := b.Buffer(2, 1)
	copy(buf.Pix, []byte{9, 8, 7, 6, 5, 4, 3, 2})
	b.Present(buf.Pix, buf.Stride, gui.FormatRGBA8888, []image.Rectangle{image.Rect(0, 0, 1, 1)})
	if s.presents != 1 {
		t.Fatalf("presents %d", s.presents)
	}
	got := s.pix[(1*4+1)*4 : (1*4+1)*4+4]
	if got[0] != 9 || got[1] != 8 || got[2] != 7 || got[3] != 6 {
		t.Fatalf("pixel %v", got)
	}
	if s.pix[0] != 1 || s.pix[(1*4+2)*4] != 1 {
		t.Fatal("damage wrote outside the rect")
	}
	if b.Format() != gui.FormatRGBA8888 || !b.CanPresent() {
		t.Fatal("format or present")
	}
}
