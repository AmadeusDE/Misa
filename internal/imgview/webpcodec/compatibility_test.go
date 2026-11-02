package webpcodec

import (
	"bytes"
	"embed"
	"errors"
	"image/color"
	"io/fs"
	"testing"

	upstream "golang.org/x/image/webp"
)

// Original synthetic fixtures, encoded once with Pillow/libwebp. No encoder is
// needed by tests or builds. Pixel patterns are checked independently below.
//
//go:embed testdata/*.webp
var fixtures embed.FS

func TestCompatibility(t *testing.T) {
	names, err := fs.Glob(fixtures, "testdata/*.webp")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			p, err := fixtures.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Decode(bytes.NewReader(p), 8<<20)
			if err != nil {
				t.Fatal(err)
			}
			want, err := upstream.Decode(bytes.NewReader(p))
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds %v != %v", got.Bounds(), want.Bounds())
			}
			for y := 0; y < got.Bounds().Dy(); y++ {
				for x := 0; x < got.Bounds().Dx(); x++ {
					if g, w := color.NRGBAModel.Convert(got.At(x, y)), color.NRGBAModel.Convert(want.At(x, y)); g != w {
						t.Fatalf("(%d,%d): %v != %v", x, y, g, w)
					}
				}
			}
			gc, err := DecodeConfig(bytes.NewReader(p))
			if err != nil {
				t.Fatal(err)
			}
			wc, err := upstream.DecodeConfig(bytes.NewReader(p))
			if err != nil {
				t.Fatal(err)
			}
			if gc != wc {
				t.Fatalf("config %v != %v", gc, wc)
			}
			_, err = Decode(bytes.NewReader(p), 33<<10)
			if !errors.Is(err, ErrLimit) {
				t.Fatalf("low budget: %v", err)
			}
		})
	}
}

func TestLosslessGolden(t *testing.T) {
	p, err := fixtures.ReadFile("testdata/rgba-lossless.webp")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Decode(bytes.NewReader(p), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			want := color.NRGBA{uint8(x * 16), uint8(y * 16), uint8((x + y) * 8), uint8(32 + (x+y)*7)}
			if got := color.NRGBAModel.Convert(m.At(x, y)); got != want {
				t.Fatalf("(%d,%d): %v != %v", x, y, got, want)
			}
		}
	}
}

func TestAlphaCompressionAndFilters(t *testing.T) {
	p, _ := fixtures.ReadFile("testdata/rgb-lossy.webp")
	cfg, err := DecodeConfig(bytes.NewReader(p))
	if err != nil {
		t.Fatal(err)
	}
	for _, compressed := range []bool{false, true} {
		for filter := byte(0); filter < 4; filter++ {
			t.Run(string(rune('0'+filter))+map[bool]string{false: "raw", true: "compressed"}[compressed], func(t *testing.T) {
				x := make([]byte, 10)
				x[0] = 16
				w, h := cfg.Width-1, cfg.Height-1
				for i := 0; i < 3; i++ {
					x[4+i] = byte(w >> (8 * i))
					x[7+i] = byte(h >> (8 * i))
				}
				a := bytes.Repeat([]byte{34}, cfg.Width*cfg.Height)
				flag := filter << 2
				if compressed {
					a = lossless(false)[5:]
					flag |= 1
				}
				body := append(chunk("VP8X", x), chunk("ALPH", append([]byte{flag}, a...))...)
				body = append(body, p[12:]...)
				encoded := webpRIFF(body)
				got, err := Decode(bytes.NewReader(encoded), 1<<20)
				if err != nil {
					t.Fatal(err)
				}
				want, err := upstream.Decode(bytes.NewReader(encoded))
				if err != nil {
					t.Fatal(err)
				}
				for y := 0; y < cfg.Height; y++ {
					for x := 0; x < cfg.Width; x++ {
						if got.At(x, y) != want.At(x, y) {
							t.Fatalf("(%d,%d): %v != %v", x, y, got.At(x, y), want.At(x, y))
						}
					}
				}
			})
		}
	}
}

func TestConfigDoesNotDecodeLosslessPayload(t *testing.T) {
	for _, p := range [][]byte{webpRIFF(chunk("VP8L", lossless(true))), alphaWebP(lossless(true)[5:], true)} {
		cfg, err := DecodeConfig(bytes.NewReader(p))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != 1 || cfg.Height != 1 {
			t.Fatalf("config %v", cfg)
		}
	}
}

func TestVP8CanvasAndPartitionsBeforeAllocation(t *testing.T) {
	p, _ := fixtures.ReadFile("testdata/rgb-lossy.webp")
	p = bytes.Clone(p)
	// The first partition length is a 19-bit field, independent of chunk length.
	p[20], p[21], p[22] = 0xf0, 0xff, 0xff
	_, err := Decode(bytes.NewReader(p), 64<<10)
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized partition: %v", err)
	}
	p[20], p[21], p[22] = 0x10, 0, 0
	p[26], p[27], p[28], p[29] = 0xff, 0x3f, 0xff, 0x3f
	_, err = Decode(bytes.NewReader(p), 1<<20)
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("oversized canvas: %v", err)
	}
}

func TestAlphaDimensionMismatch(t *testing.T) {
	p, _ := fixtures.ReadFile("testdata/rgb-lossy.webp")
	q := alphaWebP([]byte{255}, false)
	body := append(bytes.Clone(q[12:]), p[12:]...)
	_, err := Decode(bytes.NewReader(webpRIFF(body)), 1<<20)
	if err == nil {
		t.Fatal("accepted alpha plane with wrong dimensions")
	}
}
