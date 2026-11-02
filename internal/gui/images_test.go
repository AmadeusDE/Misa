package gui

import (
	"encoding/base64"
	"image"
	"image/color"
	"testing"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
	"github.com/AmadeusDE/Misa/internal/term"
)

// gfxRaster decodes raw RGB or RGBA pixels into the immutable raster a direct
// Kitty transmission produces.
func gfxRaster(t testing.TB, w, h, format int, pix []byte) *kittygfx.Raster {
	t.Helper()
	cmd := kittygfx.Command{Action: kittygfx.ActionTransmit, Format: format, Width: w, Height: h}
	r, err := kittygfx.Decode(cmd, []byte(base64.StdEncoding.EncodeToString(pix)), kittygfx.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// solidRaster is a w×h raster of one non-premultiplied RGBA colour.
func solidRaster(t testing.TB, w, h int, c color.NRGBA) *kittygfx.Raster {
	t.Helper()
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = c.R, c.G, c.B, c.A
	}
	return gfxRaster(t, w, h, 32, pix)
}

// gfxPlace is one placement of a test snapshot: the whole raster as its source
// window, clipped to the screen exactly where it is anchored.
func gfxPlace(id uint32, gen uint64, img image.Image, cell image.Rectangle) term.GraphicsPlacement {
	b := img.Bounds()
	return term.GraphicsPlacement{
		ImageID:         id,
		ImageGeneration: gen,
		Raster:          img,
		CellRect:        cell,
		Clip:            cell,
		Source:          image.Rect(0, 0, b.Dx(), b.Dy()),
	}
}

// gfxScreen is a snapshot of text with placements over it, as a terminal with
// graphics enabled produces one.
func gfxScreen(rows, cols int, out string, ps ...term.GraphicsPlacement) *term.Snapshot {
	s := screen(rows, cols, out)
	s.GraphicsGeneration = 1
	s.Graphics = ps
	return s
}

// pixelAt reads a canvas pixel as colour and alpha whatever its channel order.
func pixelAt(c *canvas, x, y int) (rgb, uint8) {
	i := y*c.stride + x*4
	if c.order == orderRGBA {
		return rgb{c.pix[i], c.pix[i+1], c.pix[i+2]}, c.pix[i+3]
	}
	return rgb{c.pix[i+2], c.pix[i+1], c.pix[i]}, c.pix[i+3]
}

func TestGraphicsCompositeAlphaOver(t *testing.T) {
	// The canvas is premultiplied and holds the default background (0x28 grey)
	// before an image is composited. Every expectation below is that
	// background blended with the source by hand.
	bg := rgb{0x28, 0x28, 0x28}
	for _, order := range []struct {
		name string
		o    pixelOrder
	}{{"BGRA", orderBGRA}, {"RGBA", orderRGBA}} {
		t.Run(order.name, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				format int
				src    [4]uint8
				want   rgb
				alpha  uint8
			}{
				// f=24 carries no alpha: an RGB image is opaque.
				{"opaque rgb", 24, [4]uint8{255, 0, 0, 0}, rgb{255, 0, 0}, 255},
				{"opaque rgba", 32, [4]uint8{0, 0, 255, 255}, rgb{0, 0, 255}, 255},
				// (40*127 + 200*128)/255 = 120, (40*127 + 100*128)/255 = 70,
				// (40*127 + 50*128)/255 = 45.
				{"half alpha", 32, [4]uint8{200, 100, 50, 128}, rgb{120, 70, 45}, 255},
				{"transparent", 32, [4]uint8{200, 100, 50, 0}, bg, 255},
			} {
				t.Run(tc.name, func(t *testing.T) {
					r := testRenderer(t)
					cw, ch := r.fonts.cellW, r.fonts.cellH
					c := canvasFor(r, 1, 1)
					c.order = order.o
					var pix []byte
					if tc.format == 24 {
						pix = []byte{tc.src[0], tc.src[1], tc.src[2]}
					} else {
						pix = tc.src[:]
					}
					img := gfxRaster(t, 1, 1, tc.format, pix)
					cell := image.Rect(0, 0, 1, 1)
					s := gfxScreen(1, 1, "", gfxPlace(1, 1, img, cell))
					r.draw(c, s, cursorState{}, &frame{})
					if got, a := pixelAt(c, r.pad, r.pad); got != tc.want || a != tc.alpha {
						t.Fatalf("pixel = %v alpha %d, want %v alpha %d", got, a, tc.want, tc.alpha)
					}
					if x := r.pad + cw - 1; colourAt(c, x, r.pad+ch-1) != tc.want {
						t.Fatalf("far corner = %v, want %v", colourAt(c, x, r.pad+ch-1), tc.want)
					}
				})
			}
		})
	}
}

// colourAt is pixelAt's colour, for the tests that do not check alpha.
func colourAt(c *canvas, x, y int) rgb {
	got, _ := pixelAt(c, x, y)
	return got
}

func TestGraphicsPlacementUsesCellMetricsAndOffset(t *testing.T) {
	r := testRenderer(t)
	cw, ch := r.fonts.cellW, r.fonts.cellH
	img := solidRaster(t, 4, 4, color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	c := canvasFor(r, 3, 6)
	p := gfxPlace(1, 1, img, image.Rect(2, 1, 4, 2))
	p.PixelOffset = image.Pt(3, 4)
	s := gfxScreen(3, 6, "", p)
	r.draw(c, s, cursorState{}, &frame{})

	x0, y0 := r.pad+2*cw+3, r.pad+1*ch+4
	if got := colourAt(c, x0, y0); got != (rgb{0, 255, 0}) {
		t.Fatalf("first image pixel = %v, want green", got)
	}
	if got := colourAt(c, x0-1, y0); got != r.pal.bg {
		t.Fatalf("pixel left of the offset = %v, want the background", got)
	}
	if got := colourAt(c, x0, y0-1); got != r.pal.bg {
		t.Fatalf("pixel above the offset = %v, want the background", got)
	}
	// The placement ends at its last cell's edge, so the offset shortens it.
	if got := colourAt(c, r.pad+4*cw-1, r.pad+2*ch-1); got != (rgb{0, 255, 0}) {
		t.Fatalf("last image pixel = %v, want green", got)
	}
	if got := colourAt(c, r.pad+4*cw, r.pad+2*ch-1); got != r.pal.bg {
		t.Fatalf("pixel right of the cells = %v, want the background", got)
	}
}

func TestGraphicsClipCutsAtScreenEdge(t *testing.T) {
	r := testRenderer(t)
	cw, ch := r.fonts.cellW, r.fonts.cellH
	img := solidRaster(t, 8, 8, color.NRGBA{B: 255, A: 255})
	// Anchored on the last row of a 3-row screen: the placement claims 4 rows
	// but only the one inside the screen is visible.
	p := gfxPlace(1, 1, img, image.Rect(0, 2, 2, 6))
	p.Clip = image.Rect(0, 2, 2, 3)
	// The second placement claims three columns but the screen has two.
	q := gfxPlace(2, 1, img, image.Rect(1, 0, 4, 1))
	q.Clip = image.Rect(1, 0, 2, 1)
	c := canvasFor(r, 3, 2)
	r.draw(c, gfxScreen(3, 2, "", p, q), cursorState{}, &frame{})
	if got := colourAt(c, r.pad, r.pad+3*ch-1); got != (rgb{0, 0, 255}) {
		t.Fatalf("visible row = %v, want blue", got)
	}
	// The rows below the screen do not exist on the canvas; the padding under
	// the grid stays background.
	if got := colourAt(c, r.pad, r.pad+3*ch+1); got != r.pal.bg {
		t.Fatalf("below the grid = %v, want the background", got)
	}
	if got := colourAt(c, r.pad+cw+cw-1, r.pad); got != (rgb{0, 0, 255}) {
		t.Fatalf("last visible column = %v, want blue", got)
	}
	if got := colourAt(c, r.pad+2*cw+1, r.pad); got != r.pal.bg {
		t.Fatalf("padding right of the grid = %v, want the background", got)
	}
}

func TestGraphicsScalesSourceToCells(t *testing.T) {
	r := testRenderer(t)
	cw, ch := r.fonts.cellW, r.fonts.cellH
	// Four source pixels become the four quadrants of one cell.
	pix := []byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 255, 255, 255, 255,
	}
	img := gfxRaster(t, 2, 2, 32, pix)
	c := canvasFor(r, 1, 1)
	r.draw(c, gfxScreen(1, 1, "", gfxPlace(1, 1, img, image.Rect(0, 0, 1, 1))), cursorState{}, &frame{})
	for _, tc := range []struct {
		name string
		x, y int
		want rgb
	}{
		{"top left", r.pad, r.pad, rgb{255, 0, 0}},
		{"top right", r.pad + cw - 1, r.pad, rgb{0, 255, 0}},
		{"bottom left", r.pad, r.pad + ch - 1, rgb{0, 0, 255}},
		{"bottom right", r.pad + cw - 1, r.pad + ch - 1, rgb{255, 255, 255}},
	} {
		if got := colourAt(c, tc.x, tc.y); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestGraphicsSourceWindowCrops(t *testing.T) {
	r := testRenderer(t)
	// A 2×1 raster whose left half is red: a source window over the right half
	// must paint only the green pixels.
	img := gfxRaster(t, 2, 1, 32, []byte{255, 0, 0, 255, 0, 255, 0, 255})
	p := gfxPlace(1, 1, img, image.Rect(0, 0, 1, 1))
	p.Source = image.Rect(1, 0, 2, 1)
	c := canvasFor(r, 1, 1)
	r.draw(c, gfxScreen(1, 1, "", p), cursorState{}, &frame{})
	if got := colourAt(c, r.pad, r.pad); got != (rgb{0, 255, 0}) {
		t.Fatalf("pixel = %v, want the cropped green half", got)
	}
}

func TestGraphicsReplacementDamagesOnlyItsRect(t *testing.T) {
	r := testRenderer(t)
	c := canvasFor(r, 4, 8)
	cell := image.Rect(2, 1, 4, 3)
	red := gfxScreen(4, 8, "hello", gfxPlace(1, 1, solidRaster(t, 4, 4, color.NRGBA{R: 255, A: 255}), cell))
	var prev frame
	cur := cursorState{}
	r.draw(c, red, cur, &prev)

	// The same id and geometry with new pixels: only its generation changes.
	blue := gfxScreen(4, 8, "hello", gfxPlace(1, 2, solidRaster(t, 4, 4, color.NRGBA{B: 255, A: 255}), cell))
	rects := gfxDamage(&prev, r.drawn(blue))
	want := r.placementRect(&blue.Graphics[0])
	if len(rects) != 1 || rects[0] != want {
		t.Fatalf("damage = %v, want just %v", rects, want)
	}
	r.draw(c, blue, cur, &prev)
	if got := colourAt(c, want.Min.X, want.Min.Y); got != (rgb{0, 0, 255}) {
		t.Fatalf("replaced pixel = %v, want blue", got)
	}
	// Nothing outside the placement was touched.
	if got := colourAt(c, r.pad, r.pad); got == (rgb{0, 0, 255}) {
		t.Fatal("the replacement painted outside its cell rectangle")
	}
}

func TestGraphicsDeletionRestoresCellsAndText(t *testing.T) {
	r := testRenderer(t)
	cw, ch := r.fonts.cellW, r.fonts.cellH
	c := canvasFor(r, 1, 4)
	text := "\x1b[41mab\x1b[m"
	green := solidRaster(t, 4, 4, color.NRGBA{G: 255, A: 255})
	with := gfxScreen(1, 4, text, gfxPlace(1, 1, green, image.Rect(0, 0, 2, 1)))
	var prev frame
	cur := cursorState{}
	r.draw(c, with, cur, &prev)
	if got := colourAt(c, r.pad+1, r.pad+ch-1); got != (rgb{0, 255, 0}) {
		t.Fatalf("image pixel = %v, want green", got)
	}

	without := gfxScreen(1, 4, text)
	rects := gfxDamage(&prev, r.drawn(without))
	if len(rects) != 1 || rects[0] != image.Rect(r.pad, r.pad, r.pad+2*cw, r.pad+ch) {
		t.Fatalf("damage = %v, want the image rectangle", rects)
	}
	r.draw(c, without, cur, &prev)
	// The whole buffer has to be what the screen draws without any image at
	// all: the red background and the glyphs over it are back.
	ref := canvasFor(r, 1, 4)
	testRenderer(t).draw(ref, without, cur, &frame{})
	if string(c.pix) != string(ref.pix) {
		for y := r.pad; y < r.pad+ch; y++ {
			for x := r.pad; x < r.pad+2*cw; x++ {
				if got, want := colourAt(c, x, y), colourAt(ref, x, y); got != want {
					t.Fatalf("restored pixel at %d,%d = %v, want %v", x, y, got, want)
				}
			}
		}
		t.Fatal("the buffer differs from the same screen drawn without the image")
	}
}

func TestGraphicsLayerSitsUnderText(t *testing.T) {
	r := testRenderer(t)
	img := solidRaster(t, 4, 4, color.NRGBA{R: 255, A: 255})
	c := canvasFor(r, 1, 2)
	s := gfxScreen(1, 2, "\x1b[30;41mX\x1b[m", gfxPlace(1, 1, img, image.Rect(0, 0, 2, 1)))
	r.draw(c, s, cursorState{}, &frame{})
	// The glyph's own foreground can only come from the text layer: the image
	// is pure red and the cell background is red too.
	glyph := r.pal.colors[0]
	var found bool
	for y := r.pad; y < r.pad+r.fonts.cellH && !found; y++ {
		for x := r.pad; x < r.pad+2*r.fonts.cellW; x++ {
			if colourAt(c, x, y) == glyph {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no glyph pixel over the image: text is not the top layer")
	}
}

func TestGraphicsCellMetricsChangeRepaintsWhole(t *testing.T) {
	r := testRenderer(t)
	s := gfxScreen(2, 4, "ab", gfxPlace(1, 1, solidRaster(t, 2, 2, color.NRGBA{R: 255, A: 255}), image.Rect(0, 0, 1, 1)))
	var prev frame
	r.draw(canvasFor(r, 2, 4), s, cursorState{}, &prev)
	if _, full := changedRows(s, cursorState{}, &prev, r.metrics()); full {
		t.Fatal("an unchanged screen after a draw must not be full")
	}
	r.pad++
	if _, full := changedRows(s, cursorState{}, &prev, r.metrics()); !full {
		t.Fatal("a cell metric change must redraw the whole buffer")
	}
}

// sizedApp is an app whose window is exactly rows×cols of its renderer's cells,
// with a graphics-enabled terminal.
func sizedApp(t *testing.T, rows, cols int) (*app, *fakeBackend) {
	t.Helper()
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW = cols*a.rend.fonts.cellW + 2*a.rend.pad
	a.logicalH = rows*a.rend.fonts.cellH + 2*a.rend.pad
	a.term = term.New(rows, cols)
	a.term.SetGraphicsEnabled(true)
	// Hide the cursor: these tests are about the image layer.
	a.term.ProcessOutput([]byte("\x1b[?25l"))
	return a, fb
}

// defaultBG is the canvas colour of a cell the app painted no background for:
// the palette background premultiplied by the configured opacity.
func defaultBG(a *app) rgb {
	bg, al := a.rend.pal.bg, a.rend.pal.alpha
	return rgb{premultiply(bg.r, al), premultiply(bg.g, al), premultiply(bg.b, al)}
}

// kittyAPC wraps a graphics body in the APC introducer and ST terminator.
func kittyAPC(body string) []byte { return []byte("\x1b_G" + body + "\x1b\\") }

func TestGraphicsImageOnlyChangePresents(t *testing.T) {
	a, fb := sizedApp(t, 2, 4)
	pix := []byte{255, 0, 0, 255}
	body := "a=T,f=32,s=1,v=1,i=7,c=1,r=1;" + base64.StdEncoding.EncodeToString(pix)
	a.term.ProcessOutput(kittyAPC(body))
	a.requestDraw()
	if len(fb.frames) != 1 {
		t.Fatalf("frames = %d, want the first draw", len(fb.frames))
	}

	a.requestDraw()
	if len(fb.frames) != 1 {
		t.Fatalf("frames = %d, an unchanged snapshot must not present", len(fb.frames))
	}

	// Replacing the image under the same id changes no cell at all.
	a.term.ProcessOutput(kittyAPC("a=t,f=32,s=1,v=1,i=7;" + base64.StdEncoding.EncodeToString([]byte{0, 0, 255, 255})))
	a.requestDraw()
	if len(fb.frames) != 2 {
		t.Fatalf("frames = %d, an image-only change must present", len(fb.frames))
	}
	want := image.Rect(a.rend.pad, a.rend.pad, a.rend.pad+a.rend.fonts.cellW, a.rend.pad+a.rend.fonts.cellH)
	if d := fb.frames[1].damage; len(d) != 1 || d[0] != want {
		t.Fatalf("damage = %v, want just the placement %v", d, want)
	}
	if got, _ := pixelAt(&canvas{pix: fb.frames[1].pix, w: fb.pw, h: fb.ph, stride: fb.stride}, a.rend.pad, a.rend.pad); got != (rgb{0, 0, 255}) {
		t.Fatalf("replaced pixel = %v, want blue", got)
	}
}

func TestGraphicsDeletionPresentsAndRestores(t *testing.T) {
	a, fb := sizedApp(t, 2, 4)
	pix := []byte{255, 0, 0, 255}
	a.term.ProcessOutput(kittyAPC("a=T,f=32,s=1,v=1,i=7,c=1,r=1;" + base64.StdEncoding.EncodeToString(pix)))
	a.requestDraw()
	a.term.ProcessOutput(kittyAPC("a=d,d=i,i=7"))
	a.requestDraw()
	if len(fb.frames) != 2 {
		t.Fatalf("frames = %d, want a frame for the deletion", len(fb.frames))
	}
	c := &canvas{pix: fb.frames[1].pix, w: fb.pw, h: fb.ph, stride: fb.stride}
	if got, _ := pixelAt(c, a.rend.pad, a.rend.pad); got != defaultBG(a) {
		t.Fatalf("pixel after deletion = %v, want the background", got)
	}
}

func TestGraphicsFontChangeRepaintsWhole(t *testing.T) {
	a, fb := sizedApp(t, 2, 4)
	a.term.ProcessOutput(kittyAPC("a=T,f=32,s=1,v=1,i=7,c=1,r=1;" + base64.StdEncoding.EncodeToString([]byte{255, 0, 0, 255})))
	a.requestDraw()
	a.onFont(Event{FontSize: a.fontSize + 2})
	if len(fb.frames) < 2 {
		t.Fatal("a font change presented no frame")
	}
	pw, ph := a.pixelSize()
	if d := fb.frames[len(fb.frames)-1].damage; len(d) != 1 || d[0] != image.Rect(0, 0, pw, ph) {
		t.Fatalf("damage after a font change = %v, want the whole buffer", d)
	}
}

func TestGraphicsRotatingBuffersConverge(t *testing.T) {
	r := testRenderer(t)
	canvases := [2]*canvas{canvasFor(r, 3, 5), canvasFor(r, 3, 5)}
	var frames [2]frame
	cell := image.Rect(1, 1, 3, 2)
	shots := []*term.Snapshot{
		gfxScreen(3, 5, "one", gfxPlace(1, 1, solidRaster(t, 4, 4, color.NRGBA{R: 255, A: 255}), cell)),
		gfxScreen(3, 5, "two", gfxPlace(2, 2, solidRaster(t, 4, 4, color.NRGBA{B: 255, A: 255}), cell)),
		gfxScreen(3, 5, "two"),
	}
	cur := cursorState{}
	for round := 0; round < 2; round++ {
		for i, s := range shots {
			r.draw(canvases[(round*len(shots)+i)%2], s, cur, &frames[(round*len(shots)+i)%2])
		}
	}
	// Drawing the same last snapshot into both buffers has to converge.
	r.draw(canvases[0], shots[len(shots)-1], cur, &frames[0])
	r.draw(canvases[1], shots[len(shots)-1], cur, &frames[1])
	if string(canvases[0].pix) != string(canvases[1].pix) {
		t.Fatal("the two buffers did not converge to the same picture")
	}
}

func TestGraphicsKittyAPCReachesTheCanvas(t *testing.T) {
	for _, format := range []struct {
		name string
		f    PixelFormat
		o    pixelOrder
	}{{"BGRA", FormatBGRA8888, orderBGRA}, {"RGBA", FormatRGBA8888, orderRGBA}} {
		t.Run(format.name, func(t *testing.T) {
			a, fb := sizedApp(t, 2, 4)
			fb.format = format.f
			// A 2×2 RGBA image: red, green, blue, white.
			pix := []byte{
				255, 0, 0, 255, 0, 255, 0, 255,
				0, 0, 255, 255, 255, 255, 255, 255,
			}
			body := "a=T,f=32,s=2,v=2,i=7,c=1,r=1;" + base64.StdEncoding.EncodeToString(pix)
			a.term.ProcessOutput(kittyAPC(body))
			a.requestDraw()
			if len(fb.frames) != 1 {
				t.Fatalf("frames = %d, want one", len(fb.frames))
			}
			c := &canvas{pix: fb.frames[0].pix, w: fb.pw, h: fb.ph, stride: fb.stride, order: format.o}
			// The 2×2 source is scaled over the whole first cell.
			cw, ch := a.rend.fonts.cellW, a.rend.fonts.cellH
			for _, tc := range []struct {
				name string
				x, y int
				want rgb
			}{
				{"top left", a.rend.pad, a.rend.pad, rgb{255, 0, 0}},
				{"top right", a.rend.pad + cw - 1, a.rend.pad, rgb{0, 255, 0}},
				{"bottom left", a.rend.pad, a.rend.pad + ch - 1, rgb{0, 0, 255}},
				{"bottom right", a.rend.pad + cw - 1, a.rend.pad + ch - 1, rgb{255, 255, 255}},
			} {
				if got := colourAt(c, tc.x, tc.y); got != tc.want {
					t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
				}
			}
			// The rest of the screen keeps the cell background.
			if got := colourAt(c, a.rend.pad+cw, a.rend.pad); got != defaultBG(a) {
				t.Errorf("second cell = %v, want the background", got)
			}
		})
	}
}

func BenchmarkDrawFullScreenImage(b *testing.B) {
	r := testRenderer(b)
	c := canvasFor(r, benchRows, benchCols)
	w, h := benchCols*r.fonts.cellW, benchRows*r.fonts.cellH
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(i), byte(i>>8), byte(i>>16), 0xc0
	}
	img := gfxRaster(b, w, h, 32, pix)
	s := gfxScreen(benchRows, benchCols, "", gfxPlace(1, 1, img, image.Rect(0, 0, benchCols, benchRows)))
	cur := cursorState{}
	b.ResetTimer()
	for b.Loop() {
		var prev frame
		r.draw(c, s, cur, &prev)
	}
}
