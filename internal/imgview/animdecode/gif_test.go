package animdecode

import (
	"bytes"
	"compress/lzw"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

func gifFixture(t testing.TB) []byte {
	t.Helper()
	p := color.Palette{color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}}
	a := image.NewPaletted(image.Rect(0, 0, 2, 1), p)
	a.Pix[1] = 1
	b := image.NewPaletted(image.Rect(1, 0, 2, 1), p)
	b.Pix[0] = 1
	var out bytes.Buffer
	err := gif.EncodeAll(&out, &gif.GIF{Image: []*image.Paletted{a, b}, Delay: []int{1, 4}, Disposal: []byte{gif.DisposalBackground, gif.DisposalPrevious}, LoopCount: 2, Config: image.Config{ColorModel: p, Width: 2, Height: 1}})
	if err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestGIFIndexFrames(t *testing.T) {
	b := gifFixture(t)
	l := Limits{MaxEncodedBytes: 1 << 20, MaxPixels: 100, MaxFrames: 10, MaxMetadataBytes: 1 << 20, MaxWorkingBytes: 1 << 20, MinDelay: 10 * time.Millisecond, GIFShortDelay: 100 * time.Millisecond}
	idx, err := indexGIF(context.Background(), b, l)
	if err != nil {
		t.Fatal(err)
	}
	if idx.size != image.Pt(2, 1) || idx.plays != 3 || len(idx.frames) != 2 {
		t.Fatalf("bad metadata: %+v", idx)
	}
	if idx.frames[0].dispose != background || idx.frames[1].dispose != previous || idx.frames[1].rect != image.Rect(1, 0, 2, 1) {
		t.Fatal("bad frame metadata")
	}
	if idx.frames[0].encoded != 10*time.Millisecond || idx.frames[0].delay != 100*time.Millisecond {
		t.Fatalf("bad delay: %+v", idx.frames[0])
	}
	for i, f := range idx.frames {
		im, err := f.decode(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds() != f.rect {
			t.Fatalf("frame %d bounds %v", i, im.Bounds())
		}
		c := color.NRGBAModel.Convert(im.At(f.rect.Max.X-1, 0)).(color.NRGBA)
		if c != (color.NRGBA{0, 0, 255, 255}) {
			t.Fatalf("frame %d pixel %v", i, c)
		}
	}
}

func TestGIFMalformedAndLimits(t *testing.T) {
	b := gifFixture(t)
	l := Limits{MaxEncodedBytes: 1 << 20, MaxPixels: 100, MaxFrames: 10, MaxMetadataBytes: 1 << 20, MaxWorkingBytes: 1 << 20}
	for _, v := range [][]byte{b[:len(b)-1], b[:15], append(append([]byte{}, b...), 0)} {
		if _, err := indexGIF(context.Background(), v, l); !errors.Is(err, ErrMalformed) {
			t.Errorf("expected malformed, got %v", err)
		}
	}
	l.MaxFrames = 1
	if _, err := indexGIF(context.Background(), b, l); !errors.Is(err, ErrLimit) {
		t.Errorf("expected limit, got %v", err)
	}
}

func FuzzGIFIndex(f *testing.F) {
	f.Add(gifFixture(f))
	f.Add([]byte("GIF89a"))
	f.Add([]byte("not a GIF"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			return
		}
		l := Limits{MaxEncodedBytes: 1 << 16, MaxPixels: 4096, MaxFrames: 32, MaxMetadataBytes: 1 << 20, MaxWorkingBytes: 4 << 20}
		idx, err := indexGIF(context.Background(), b, l)
		if err == nil {
			for _, fr := range idx.frames {
				_, _ = fr.decode(context.Background())
			}
		}
	})
}

func TestGIFCompositeGolden(t *testing.T) {
	p := color.Palette{color.NRGBA{0, 0, 0, 0}, color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}, color.NRGBA{0, 255, 0, 255}}
	mk := func(r image.Rectangle, pix ...byte) *image.Paletted {
		im := image.NewPaletted(r, p)
		copy(im.Pix, pix)
		return im
	}
	frames := []*image.Paletted{mk(image.Rect(0, 0, 3, 1), 1, 1, 1), mk(image.Rect(0, 0, 1, 1), 2), mk(image.Rect(1, 0, 2, 1), 3), mk(image.Rect(2, 0, 3, 1), 2), mk(image.Rect(0, 0, 1, 1), 3)}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: frames, Delay: []int{2, 2, 2, 2, 2}, Disposal: []byte{1, 2, 3, 1, 1}, LoopCount: -1, Config: image.Config{ColorModel: p, Width: 3, Height: 1}}); err != nil {
		t.Fatal(err)
	}
	for _, lazy := range []bool{false, true} {
		l := DefaultLimits()
		if lazy {
			l.MaxWorkingBytes = (512 << 10) + 4096
		}
		idx, err := indexGIF(context.Background(), b.Bytes(), l)
		if err != nil {
			t.Fatal(err)
		}
		if idx.plays != 1 || idx.background.A != 0 {
			t.Fatalf("loop/background: %d %v", idx.plays, idx.background)
		}
		s := &Source{idx: idx, limits: l}
		want := [][]color.NRGBA{
			{{255, 0, 0, 255}, {255, 0, 0, 255}, {255, 0, 0, 255}},
			{{0, 0, 255, 255}, {255, 0, 0, 255}, {255, 0, 0, 255}},
			{{0, 0, 0, 0}, {0, 255, 0, 255}, {255, 0, 0, 255}},
			{{0, 0, 0, 0}, {255, 0, 0, 255}, {0, 0, 255, 255}},
			{{0, 255, 0, 255}, {255, 0, 0, 255}, {0, 0, 255, 255}},
		}
		for i, row := range want {
			got, err := s.DecodeFrame(context.Background(), i)
			if err != nil {
				t.Fatal(err)
			}
			for x, c := range row {
				if v := got.Pixels.NRGBAAt(x, 0); v != c {
					t.Errorf("frame %d x %d: got %v want %v", i, x, v, c)
				}
			}
		}
	}
}

func TestGIFLocalPaletteInterlace(t *testing.T) {
	// Eight one-pixel rows, transmitted in GIF's four interlace passes.
	b := []byte("GIF89a")
	b = append(b, 1, 0, 8, 0, 0x80, 0, 0, 255, 0, 0, 0, 0, 255)
	b = append(b, 0x2c, 0, 0, 0, 0, 1, 0, 8, 0, 0xc0, 0, 0, 0, 0, 255, 0, 2)
	var data bytes.Buffer
	z := lzw.NewWriter(&data, lzw.LSB, 2)
	_, err := z.Write([]byte{0, 0, 0, 0, 1, 1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	b = append(b, byte(data.Len()))
	b = append(b, data.Bytes()...)
	b = append(b, 0, 0x3b)
	for _, lazy := range []bool{false, true} {
		l := DefaultLimits()
		if lazy {
			l.MaxWorkingBytes = (512 << 10) + 4096
		}
		idx, err := indexGIF(context.Background(), b, l)
		if err != nil {
			t.Fatal(err)
		}
		im, err := idx.frames[0].decode(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 8; y++ {
			want := color.NRGBA{0, 0, 0, 255}
			if y%2 == 1 {
				want = color.NRGBA{0, 255, 0, 255}
			}
			if got := color.NRGBAModel.Convert(im.At(0, y)).(color.NRGBA); got != want {
				t.Errorf("row %d = %v, want %v", y, got, want)
			}
		}
	}
}

func TestGIFRejectFrameMetadata(t *testing.T) {
	b := gifFixture(t)
	cases := map[string]func([]byte){
		"background index": func(v []byte) { v[11] = 255 },
		"disposal":         func(v []byte) { i := bytes.Index(v, []byte{0x21, 0xf9, 4}); v[i+3] = (v[i+3] & 3) | 16 },
		"empty rectangle":  func(v []byte) { i := bytes.IndexByte(v, 0x2c); v[i+5], v[i+6] = 0, 0 },
		"outside canvas":   func(v []byte) { i := bytes.IndexByte(v, 0x2c); v[i+1], v[i+2] = 2, 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := bytes.Clone(b)
			mutate(v)
			if _, err := indexGIF(context.Background(), v, DefaultLimits()); !errors.Is(err, ErrMalformed) {
				t.Fatalf("got %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := indexGIF(ctx, b, DefaultLimits()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestGIFSmallPredecodeBudget(t *testing.T) {
	b := gifFixture(t)
	l := DefaultLimits()
	idx, err := indexGIF(context.Background(), b, l)
	if err != nil {
		t.Fatal(err)
	}
	a, err := idx.frames[0].decode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c, err := idx.frames[0].decode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a != c {
		t.Fatal("small sequence was not predecoded")
	}
	if idx.eagerBytes == 0 {
		t.Fatal("predecoded frames not charged")
	}
	l.MaxWorkingBytes = (512 << 10) + 4096
	idx, err = indexGIF(context.Background(), b, l)
	if err != nil {
		t.Fatal(err)
	}
	a, err = idx.frames[0].decode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c, err = idx.frames[0].decode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("working budget did not disable predecode")
	}
}
