package imageout

import (
	"image"
	"testing"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

func TestKittyTransmitLimitedGolden(t *testing.T) {
	im := &image.NRGBA{Rect: image.Rect(0, 0, 2, 1), Stride: 8, Pix: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	limits := kittygfx.DefaultLimits()
	limits.MaxPayload = 50
	out, err := KittyTransmitLimited(1, im, image.Point{}, image.Pt(1, 1), limits)
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[s\x1b[1;1H\x1b_Ga=T,t=d,f=32,i=1,p=1,s=2,v=1,C=1,q=2,m=1;AQID\x1b\\\x1b_Gm=1,q=2;BAUG\x1b\\\x1b_Gm=0,q=2;Bwg=\x1b\\\x1b[u"
	if string(out) != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestKittyTransmitLimitedRejects(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	for _, mutate := range []func(*kittygfx.Limits){
		func(l *kittygfx.Limits) { l.MaxWidth = 1 },
		func(l *kittygfx.Limits) { l.MaxImageBytes = 7 },
		func(l *kittygfx.Limits) { l.MaxTerminalBytes = 7 },
		func(l *kittygfx.Limits) { l.MaxInFlightBytes = 11 },
		func(l *kittygfx.Limits) { l.MaxPayload = 49 },
	} {
		l := kittygfx.DefaultLimits()
		mutate(&l)
		out, err := KittyTransmitLimited(1, im, image.Point{}, image.Pt(1, 1), l)
		if err == nil || len(out) != 0 {
			t.Errorf("limits %+v: out=%q err=%v", l, out, err)
		}
	}
}
