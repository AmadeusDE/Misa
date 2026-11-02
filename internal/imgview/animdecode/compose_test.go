package animdecode

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

func TestCompositorDisposalAndReplay(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	green := color.NRGBA{G: 255, A: 255}
	solid := func(c color.NRGBA) func(context.Context) (image.Image, error) {
		return func(context.Context) (image.Image, error) {
			p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			p.SetNRGBA(0, 0, c)
			return p, nil
		}
	}
	idx := &index{size: image.Pt(2, 1), frames: []frame{
		{rect: image.Rect(0, 0, 1, 1), source: true, decode: solid(red)},
		{rect: image.Rect(0, 0, 1, 1), source: true, dispose: previous, decode: solid(blue)},
		{rect: image.Rect(1, 0, 2, 1), source: true, dispose: background, decode: solid(green)},
		{rect: image.Rect(0, 0, 1, 1), source: true, decode: solid(blue)},
	}}
	s := &Source{idx: idx, meta: metadata(idx), limits: DefaultLimits()}
	for _, tt := range []struct {
		i    int
		a, b color.NRGBA
	}{{0, red, color.NRGBA{}}, {1, blue, color.NRGBA{}}, {2, red, green}, {3, blue, color.NRGBA{}}, {1, blue, color.NRGBA{}}} {
		f, err := s.DecodeFrame(context.Background(), tt.i)
		if err != nil {
			t.Fatal(err)
		}
		if f.Pixels.NRGBAAt(0, 0) != tt.a || f.Pixels.NRGBAAt(1, 0) != tt.b {
			t.Fatalf("frame %d: %v %v", tt.i, f.Pixels.NRGBAAt(0, 0), f.Pixels.NRGBAAt(1, 0))
		}
	}
}

func TestLimitsTimingCloseCancel(t *testing.T) {
	l := DefaultLimits()
	if validSize(0, 1, l) || validSize(int(^uint(0)>>1), 2, l) {
		t.Fatal("invalid size")
	}
	if effectiveDelay(10*time.Millisecond, true, l) != 100*time.Millisecond || effectiveDelay(0, false, l) != 20*time.Millisecond {
		t.Fatal("timing")
	}
	idx := &index{size: image.Pt(1, 1), frames: []frame{{rect: image.Rect(0, 0, 1, 1), decode: func(context.Context) (image.Image, error) { return image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil }}}}
	l.MaxWorkingBytes = 1
	if !errors.Is(checkIndex(idx, l), ErrLimit) {
		t.Fatal("budget")
	}
	s := &Source{idx: idx, meta: metadata(idx), limits: DefaultLimits()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.DecodeFrame(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecodeFrame(context.Background(), 0); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestCompositorBlendAndImmutableResults(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	translucent := color.NRGBA{B: 255, A: 128}
	pixel := func(c color.NRGBA) func(context.Context) (image.Image, error) {
		return func(context.Context) (image.Image, error) {
			p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			p.SetNRGBA(0, 0, c)
			return p, nil
		}
	}
	idx := &index{size: image.Pt(1, 1), frames: []frame{{rect: image.Rect(0, 0, 1, 1), source: true, decode: pixel(red)}, {rect: image.Rect(0, 0, 1, 1), decode: pixel(translucent)}, {rect: image.Rect(0, 0, 1, 1), source: true, decode: pixel(translucent)}}}
	s := &Source{idx: idx, meta: metadata(idx), limits: DefaultLimits()}
	first, err := s.DecodeFrame(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.DecodeFrame(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := second.Pixels.NRGBAAt(0, 0); got != (color.NRGBA{R: 127, B: 128, A: 255}) {
		t.Fatalf("over: %v", got)
	}
	third, err := s.DecodeFrame(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := third.Pixels.NRGBAAt(0, 0); got != translucent {
		t.Fatalf("source: %v", got)
	}
	if first.Pixels.NRGBAAt(0, 0) != red {
		t.Fatal("previous result mutated")
	}
}

func TestCancellationAfterCodec(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	idx := &index{size: image.Pt(1, 1), frames: []frame{{rect: image.Rect(0, 0, 1, 1), decode: func(context.Context) (image.Image, error) {
		cancel()
		return image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil
	}}}}
	s := &Source{idx: idx, meta: metadata(idx), limits: DefaultLimits()}
	if _, err := s.DecodeFrame(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.canvas != nil {
		t.Fatal("partial canvas retained")
	}
}

func TestOpenOwnsEncodedSnapshot(t *testing.T) {
	var buf bytes.Buffer
	pal := color.Palette{color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}}
	im := image.NewPaletted(image.Rect(0, 0, 1, 1), pal)
	if err := gif.Encode(&buf, im, nil); err != nil {
		t.Fatal(err)
	}
	input := buf.Bytes()
	s, err := Open(context.Background(), input, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	clear(input)
	f, err := s.DecodeFrame(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Pixels.NRGBAAt(0, 0); got != (color.NRGBA{R: 255, A: 255}) {
		t.Fatal(got)
	}
}

func TestPosterAndBackgroundDisposal(t *testing.T) {
	bg := color.NRGBA{G: 255, A: 255}
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	pixel := func(c color.NRGBA) func(context.Context) (image.Image, error) {
		return func(context.Context) (image.Image, error) {
			p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			p.SetNRGBA(0, 0, c)
			return p, nil
		}
	}
	idx := &index{size: image.Pt(1, 1), background: bg, poster: pixel(blue), frames: []frame{{rect: image.Rect(0, 0, 1, 1), source: true, dispose: background, decode: pixel(red)}, {rect: image.Rect(0, 0, 1, 1), decode: pixel(color.NRGBA{})}}}
	s := &Source{idx: idx, meta: metadata(idx), limits: DefaultLimits()}
	p, err := s.Poster(context.Background())
	if err != nil || p.NRGBAAt(0, 0) != blue {
		t.Fatalf("poster %v %v", p, err)
	}
	f, err := s.DecodeFrame(context.Background(), 1)
	if err != nil || f.Pixels.NRGBAAt(0, 0) != bg {
		t.Fatalf("frame %v %v", f, err)
	}
}
