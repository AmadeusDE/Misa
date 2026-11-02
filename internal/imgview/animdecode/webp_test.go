package animdecode

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	_ "golang.org/x/image/webp"
	"image"
	"image/color"
	"testing"
	"time"
)

// The static frame is the Go x/image BSD-licensed fixture already used by imgview.
const webpStatic = "UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA=="

func wpChunk(tag string, data []byte) []byte {
	b := make([]byte, 8+len(data)+(len(data)&1))
	copy(b, tag)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(data)))
	copy(b[8:], data)
	return b
}
func wp24(b []byte, n int) { b[0] = byte(n); b[1] = byte(n >> 8); b[2] = byte(n >> 16) }
func wpFixture(t testing.TB) []byte {
	t.Helper()
	s, e := base64.StdEncoding.DecodeString(webpStatic)
	if e != nil {
		t.Fatal(e)
	}
	c, _, e := image.DecodeConfig(bytes.NewReader(s))
	if e != nil {
		t.Fatal(e)
	}
	x := make([]byte, 10)
	x[0] = 2
	wp24(x[4:], c.Width+1)
	wp24(x[7:], c.Height+1)
	h := make([]byte, 16)
	wp24(h, 1)
	wp24(h[3:], 1)
	wp24(h[6:], c.Width-1)
	wp24(h[9:], c.Height-1)
	wp24(h[12:], 7)
	h[15] = 3
	p := append(wpChunk("VP8X", x), wpChunk("ANIM", []byte{3, 2, 1, 255, 2, 0})...)
	p = append(p, wpChunk("ANMF", append(h, s[12:]...))...)
	b := make([]byte, 12)
	copy(b, "RIFF")
	copy(b[8:], "WEBP")
	binary.LittleEndian.PutUint32(b[4:], uint32(4+len(p)))
	return append(b, p...)
}
func TestWebPIndex(t *testing.T) {
	b := wpFixture(t)
	l := normalizeLimits(Limits{})
	idx, e := indexWebP(context.Background(), b, l)
	if e != nil {
		t.Fatal(e)
	}
	if idx.plays != 2 || idx.background != (color.NRGBA{1, 2, 3, 255}) || len(idx.frames) != 1 {
		t.Fatalf("metadata %+v", idx)
	}
	f := idx.frames[0]
	if f.rect.Min != image.Pt(2, 2) || f.dispose != background || !f.source || f.encoded != 7*time.Millisecond || f.delay != l.MinDelay {
		t.Fatalf("frame %+v", f)
	}
	im, e := f.decode(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	static, _ := base64.StdEncoding.DecodeString(webpStatic)
	want, _, e := image.Decode(bytes.NewReader(static))
	if e != nil {
		t.Fatal(e)
	}
	for y := 0; y < im.Bounds().Dy(); y++ {
		for x := 0; x < im.Bounds().Dx(); x++ {
			if color.NRGBAModel.Convert(im.At(x, y)) != color.NRGBAModel.Convert(want.At(x, y)) {
				t.Fatalf("pixel %d,%d", x, y)
			}
		}
	}
}
func TestWebPMalformed(t *testing.T) {
	b := wpFixture(t)
	l := normalizeLimits(Limits{})
	for _, v := range [][]byte{b[:len(b)-1], append(bytes.Clone(b), 0)} {
		if _, e := indexWebP(context.Background(), v, l); !errors.Is(e, ErrMalformed) {
			t.Fatalf("%v", e)
		}
	}
	v := bytes.Clone(b)
	v[20] |= 0x80
	if _, e := indexWebP(context.Background(), v, l); !errors.Is(e, ErrMalformed) {
		t.Fatalf("reserved %v", e)
	}
	v = bytes.Clone(b)
	v[60]++
	if _, e := indexWebP(context.Background(), v, l); !errors.Is(e, ErrMalformed) {
		t.Fatalf("mismatch %v", e)
	}
	l.MaxFrames = 1
	b = append(b, b[44:]...)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	if _, e := indexWebP(context.Background(), b, l); !errors.Is(e, ErrLimit) {
		t.Fatalf("limit %v", e)
	}
}
func FuzzWebPIndex(f *testing.F) {
	f.Add([]byte("RIFF"))
	f.Add(wpFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		l := normalizeLimits(Limits{MaxPixels: 4096, MaxFrames: 32, MaxMetadataBytes: 65536, MaxWorkingBytes: 1 << 20})
		idx, e := indexWebP(context.Background(), b, l)
		if e == nil {
			for _, fr := range idx.frames {
				_, _ = fr.decode(context.Background())
			}
		}
	})
}

// Authored 4x4 RGBA image (220,20,30,128), encoded with ffmpeg/libwebp.
const webpAlphaLossy = "UklGRl4AAABXRUJQVlA4WAoAAAAQAAAAAwAAAwAAQUxQSAoAAAABB1DAiAhERP8DVlA4IC4AAACQAQCdASoEAAQAAgA0JaACdLoAA5gA/vBTH/c1FbFvQ/92T/+2T/+2T/3CAAAA"

func TestWebPAlphaLossyComposition(t *testing.T) {
	static, err := base64.StdEncoding.DecodeString(webpAlphaLossy)
	if err != nil {
		t.Fatal(err)
	}
	x := make([]byte, 10)
	x[0] = 0x12
	wp24(x[4:], 5)
	wp24(x[7:], 5)
	h := make([]byte, 16)
	wp24(h, 1)
	wp24(h[3:], 1)
	wp24(h[6:], 3)
	wp24(h[9:], 3)
	h[15] = 3
	chunk := wpChunk("ANMF", append(h, static[30:]...))
	h[15] = 0
	p := append(wpChunk("VP8X", x), wpChunk("ANIM", []byte{0, 0, 0, 255, 0, 0})...)
	p = append(p, chunk...)
	p = append(p, wpChunk("ANMF", append(h, static[30:]...))...)
	b := make([]byte, 12)
	copy(b, "RIFF")
	copy(b[8:], "WEBP")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(p)+4))
	b = append(b, p...)
	l := normalizeLimits(Limits{})
	idx, err := indexWebP(context.Background(), b, l)
	if err != nil {
		t.Fatal(err)
	}
	s := Source{idx: idx, limits: l}
	first, err := s.DecodeFrame(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.DecodeFrame(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// Independently decoded with ffmpeg: lossy VP8 (218,21,28), alpha 128.
	// Different compliant VP8 decoders can differ in color conversion; allow a small tolerance.
	near := func(got, want color.NRGBA) bool {
		if got.A != want.A {
			return false
		}
		for _, v := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}} {
			d := int(v[0]) - int(v[1])
			if d < -12 || d > 12 {
				return false
			}
		}
		return true
	}
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			a, b := first.Pixels.NRGBAAt(x, y), second.Pixels.NRGBAAt(x, y)
			wa, wb := color.NRGBA{0, 0, 0, 255}, color.NRGBA{0, 0, 0, 255}
			if x >= 2 && y >= 2 {
				wa = color.NRGBA{218, 21, 28, 128}
				wb = color.NRGBA{109, 11, 14, 255}
			}
			if !near(a, wa) || !near(b, wb) {
				t.Fatalf("(%d,%d) got %v / %v want %v / %v", x, y, a, b, wa, wb)
			}
		}
	}
}

func TestWebPStructuralRejections(t *testing.T) {
	b := wpFixture(t)
	l := normalizeLimits(Limits{})
	pos := bytes.Index(b, []byte("ANMF"))
	if pos < 0 {
		t.Fatal("frame missing")
	}
	tests := map[string]func([]byte) []byte{
		"riff magic":        func(v []byte) []byte { v[0] = 'X'; return v },
		"vp8x reserved":     func(v []byte) []byte { v[21] = 1; return v },
		"frame flags":       func(v []byte) []byte { v[pos+23] |= 4; return v },
		"offset outside":    func(v []byte) []byte { wp24(v[pos+8:], 999); return v },
		"missing animation": func(v []byte) []byte { copy(v[30:], "JUNK"); return v },
		"nonzero padding":   func(v []byte) []byte { v[len(v)-1] = 1; return v },
		"metadata budget":   func(v []byte) []byte { return v },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			limits := l
			if name == "metadata budget" {
				limits.MaxMetadataBytes = 1
			}
			_, e := indexWebP(context.Background(), edit(bytes.Clone(b)), limits)
			want := ErrMalformed
			if name == "metadata budget" {
				want = ErrLimit
			}
			if !errors.Is(e, want) {
				t.Fatalf("got %v want %v", e, want)
			}
		})
	}
}

// Original 2x2 RGBA image encoded by ffmpeg/libwebp in lossless mode.
const webpOriginalLossless = "UklGRjYAAABXRUJQVlA4TCkAAAAvAUAAEB8gEEiS3R9hDYEAwf+YKyMjICE8l+UCsFdVKAkQAEUZieh/MAA="

func TestWebPLosslessIndependentCanvasGolden(t *testing.T) {
	static, err := base64.StdEncoding.DecodeString(webpOriginalLossless)
	if err != nil {
		t.Fatal(err)
	}
	x := make([]byte, 10)
	x[0] = 0x12
	wp24(x[4:], 3)
	wp24(x[7:], 3)
	h := make([]byte, 16)
	wp24(h, 1)
	wp24(h[3:], 1)
	wp24(h[6:], 1)
	wp24(h[9:], 1)
	h[15] = 3
	chunk := wpChunk("ANMF", append(h, static[12:]...))
	wp24(h, 0)
	wp24(h[3:], 0)
	h[15] = 2
	p := append(wpChunk("VP8X", x), wpChunk("ANIM", []byte{30, 20, 10, 255, 1, 0})...)
	p = append(p, chunk...)
	p = append(p, wpChunk("ANMF", append(h, static[12:]...))...)
	b := make([]byte, 12)
	copy(b, "RIFF")
	copy(b[8:], "WEBP")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(p)+4))
	b = append(b, p...)
	l := normalizeLimits(Limits{})
	idx, err := indexWebP(context.Background(), b, l)
	if err != nil {
		t.Fatal(err)
	}
	s := Source{idx: idx, limits: l}
	bg := color.NRGBA{10, 20, 30, 255}
	pixels := [2][2]color.NRGBA{{{255, 0, 0, 255}, {0, 255, 0, 128}}, {{0, 0, 0, 0}, {200, 100, 50, 255}}}
	for n := 0; n < 2; n++ {
		fr, err := s.DecodeFrame(context.Background(), n)
		if err != nil {
			t.Fatal(err)
		}
		offset := 2
		if n == 1 {
			offset = 0
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				want := bg
				if x >= offset && x < offset+2 && y >= offset && y < offset+2 {
					want = pixels[y-offset][x-offset]
				}
				if got := fr.Pixels.NRGBAAt(x, y); got != want {
					t.Fatalf("frame %d (%d,%d): %v want %v", n, x, y, got, want)
				}
			}
		}
	}
}

func TestWebPFrameDescriptorCharge(t *testing.T) {
	l := normalizeLimits(Limits{})
	l.MaxMetadataBytes = 511
	if _, err := indexWebP(context.Background(), wpFixture(t), l); !errors.Is(err, ErrLimit) {
		t.Fatalf("got %v", err)
	}
}
