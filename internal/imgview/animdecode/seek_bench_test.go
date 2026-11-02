package animdecode

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func BenchmarkLazyGIFSeek(b *testing.B) {
	const count = 1000
	palette := color.Palette{color.Black, color.White}
	pixels := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	anim := &gif.GIF{Config: image.Config{ColorModel: palette, Width: 1, Height: 1}, Image: make([]*image.Paletted, count), Delay: make([]int, count)}
	for i := range anim.Image {
		anim.Image[i] = pixels
		anim.Delay[i] = 1
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, anim); err != nil {
		b.Fatal(err)
	}
	limits := DefaultLimits()
	limits.MaxFrames = count
	limits.MaxWorkingBytes = 1 << 20
	source, err := Open(context.Background(), encoded.Bytes(), limits)
	if err != nil {
		b.Fatal(err)
	}
	defer source.Close()
	if source.Metadata().Frames != count || source.idx.eagerBytes != 0 {
		b.Fatal("expected a lazy 1000-frame source")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := source.DecodeFrame(context.Background(), count-1); err != nil {
			b.Fatal(err)
		}
		if _, err := source.DecodeFrame(context.Background(), 0); err != nil {
			b.Fatal(err)
		}
	}
}
