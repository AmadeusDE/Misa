package imgview

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func TestDecodeFormatsAndLimits(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.SetNRGBA(1, 0, color.NRGBA{200, 30, 40, 128})
	for _, tc := range []struct {
		name   string
		encode func(*bytes.Buffer) error
		format string
	}{{"png", func(b *bytes.Buffer) error { return png.Encode(b, img) }, "png"}, {"jpeg", func(b *bytes.Buffer) error { return jpeg.Encode(b, img, nil) }, "jpeg"}, {"gif", func(b *bytes.Buffer) error { return gif.Encode(b, img, nil) }, "gif"}, {"bmp", func(b *bytes.Buffer) error { return bmp.Encode(b, img) }, "bmp"}, {"tiff", func(b *bytes.Buffer) error { return tiff.Encode(b, img, nil) }, "tiff"}} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := tc.encode(&b); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(t.TempDir(), "wrong.ext")
			if err := os.WriteFile(p, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			d, err := Decode(p, Limits{})
			if err != nil || d.Format != tc.format || d.Pixels.Bounds().Size() != image.Pt(3, 2) {
				t.Fatalf("%+v %v", d, err)
			}
			if _, err = Decode(p, Limits{MaxPixels: 5}); !errors.Is(err, ErrLimit) {
				t.Fatalf("pixel limit: %v", err)
			}
			if _, err = Decode(p, Limits{MaxEncodedBytes: 1}); !errors.Is(err, ErrLimit) {
				t.Fatalf("file limit: %v", err)
			}
		})
	}
	if _, err := Decode(t.TempDir(), Limits{}); err == nil {
		t.Fatal("directory accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DecodeContext(ctx, "missing", Limits{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestAnimationDetection(t *testing.T) {
	pal := color.Palette{color.Transparent, color.White}
	frame := image.NewPaletted(image.Rect(1, 1, 2, 2), pal)
	frame.SetColorIndex(1, 1, 1)
	var g bytes.Buffer
	if err := gif.EncodeAll(&g, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{1, 1}, Config: image.Config{Width: 3, Height: 3, ColorModel: pal}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		data  []byte
		size  image.Point
		pixel image.Point
	}{{"gif", g.Bytes(), image.Pt(3, 3), image.Pt(1, 1)}, {"apng", integrationAPNG(t), image.Pt(1, 1), image.Pt(0, 0)}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			d, err := Decode(path, Limits{})
			if err != nil || !d.Animated || d.Pixels == nil || d.Pixels.Bounds().Size() != tc.size {
				t.Fatalf("%+v %v", d, err)
			}
			if d.Pixels.NRGBAAt(tc.pixel.X, tc.pixel.Y).A != 255 {
				t.Fatal("missing poster pixel")
			}
			if _, err := Decode(path, Limits{MaxPixels: 1, Animation: animdecode.Limits{MaxWorkingBytes: 1}}); !errors.Is(err, ErrLimit) {
				t.Fatalf("limit: %v", err)
			}
		})
	}
}

// This lossless WebP image is from x/image/testdata (Go project BSD license).
func TestAnimatedWebPPoster(t *testing.T) {
	static, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(static))
	if err != nil {
		t.Fatal(err)
	}
	chunk := func(name string, data []byte) []byte {
		out := make([]byte, 8+len(data)+(len(data)&1))
		copy(out, name)
		binary.LittleEndian.PutUint32(out[4:], uint32(len(data)))
		copy(out[8:], data)
		return out
	}
	vp8x := make([]byte, 10)
	vp8x[0] = 2
	put24(vp8x[4:], uint32(cfg.Width+1))
	put24(vp8x[7:], uint32(cfg.Height+1))
	anim := []byte{3, 2, 1, 255, 0, 0}
	fh := make([]byte, 16)
	put24(fh, 1)
	put24(fh[3:], 1)
	put24(fh[6:], uint32(cfg.Width-1))
	put24(fh[9:], uint32(cfg.Height-1))
	payload := append(chunk("VP8X", vp8x), chunk("ANIM", anim)...)
	payload = append(payload, chunk("ANMF", append(fh, static[12:]...))...)
	data := make([]byte, 12)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(4+len(payload)))
	copy(data[8:], "WEBP")
	data = append(data, payload...)
	path := filepath.Join(t.TempDir(), "a.webp")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	d, err := Decode(path, Limits{})
	if err != nil || !d.Animated || d.Pixels == nil {
		t.Fatalf("%+v %v", d, err)
	}
	if d.Pixels.Bounds().Size() != image.Pt(cfg.Width+2, cfg.Height+2) || d.Pixels.NRGBAAt(0, 0) != (color.NRGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatal("incorrect canvas or background")
	}
	expected, _, err := image.Decode(bytes.NewReader(static))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			if color.NRGBAModel.Convert(expected.At(x, y)) != color.NRGBAModel.Convert(d.Pixels.At(x+2, y+2)) {
				t.Fatalf("pixel %d,%d", x, y)
			}
		}
	}
	if _, err := Decode(path, Limits{MaxPixels: 1}); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit: %v", err)
	}
}
func TestOrientation(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(1 + y*2 + x), A: 255})
		}
	}
	want := [][]uint8{{1, 2, 3, 4, 5, 6}, {2, 1, 4, 3, 6, 5}, {6, 5, 4, 3, 2, 1}, {5, 6, 3, 4, 1, 2}, {1, 3, 5, 2, 4, 6}, {5, 3, 1, 6, 4, 2}, {6, 4, 2, 5, 3, 1}, {2, 4, 6, 1, 3, 5}}
	for o := 1; o <= 8; o++ {
		out := Transform(src, Orientation(o))
		var got []uint8
		for y := 0; y < out.Bounds().Dy(); y++ {
			for x := 0; x < out.Bounds().Dx(); x++ {
				got = append(got, out.NRGBAAt(x, y).R)
			}
		}
		if !bytes.Equal(got, want[o-1]) {
			t.Fatalf("orientation %d: %v", o, got)
		}
	}
	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		for i := 1; i <= 8; i++ {
			b := make([]byte, 26)
			if order == binary.BigEndian {
				copy(b, "MM")
			} else {
				copy(b, "II")
			}
			order.PutUint16(b[2:], 42)
			order.PutUint32(b[4:], 8)
			order.PutUint16(b[8:], 1)
			order.PutUint16(b[10:], 0x112)
			order.PutUint16(b[12:], 3)
			order.PutUint32(b[14:], 1)
			order.PutUint16(b[18:], uint16(i))
			if got := tiffOrientation(b); got != Orientation(i) {
				t.Fatalf("orientation %d: %d", i, got)
			}
			jpg := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(b) + 8), 'E', 'x', 'i', 'f', 0, 0}, b...)
			if exifOrientation(jpg) != Orientation(i) {
				t.Fatal("JPEG EXIF")
			}
		}
	}
}
func TestCache(t *testing.T) {
	c := NewCache(8)
	makeD := func() Decoded { return Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 1, 1))} }
	c.Put("a", makeD())
	c.Put("b", makeD())
	c.Get("a")
	c.Put("c", makeD())
	if _, ok := c.Get("b"); ok {
		t.Fatal("LRU")
	}
	if c.Bytes() != 8 {
		t.Fatal(c.Bytes())
	}
	c.Delete("a")
	if c.Bytes() != 4 {
		t.Fatal(c.Bytes())
	}
	if c.Put("large", Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 3, 1))}) {
		t.Fatal("oversized")
	}
}
func TestLoader(t *testing.T) {
	l := NewLoader(1, 2)
	defer l.Close()
	if !l.Submit(Request{Path: "missing", Generation: 42, Visible: true}) {
		t.Fatal("submit")
	}
	r := <-l.Results()
	if r.Request.Generation != 42 || r.Err == nil {
		t.Fatal(r)
	}
}
func FuzzOrientation(f *testing.F) {
	f.Add([]byte("Exif"))
	f.Fuzz(func(t *testing.T, b []byte) { _ = tiffOrientation(b); _ = exifOrientation(b); _, _ = isAnimated(b) })
}

func put24(b []byte, n uint32) { b[0] = byte(n); b[1] = byte(n >> 8); b[2] = byte(n >> 16) }
