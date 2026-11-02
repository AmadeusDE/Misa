package imageout

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

func TestKittySubimageStride(t *testing.T) {
	parent := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	copy(parent.Pix[4:12], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	copy(parent.Pix[16:24], []byte{9, 10, 11, 12, 13, 14, 15, 16})
	sub := parent.SubImage(image.Rect(1, 0, 3, 2)).(*image.NRGBA)
	out, err := KittyTransmit(1, sub, image.Point{}, image.Pt(8, 16))
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[s\x1b[1;1H\x1b_Ga=T,t=d,f=32,i=1,p=1,s=2,v=2,C=1,q=2,m=0;AQIDBAUGBwgJCgsMDQ4PEA==\x1b\\\x1b[u"
	if string(out) != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestKittyByteBound(t *testing.T) {
	im := &image.NRGBA{Rect: image.Rect(0, 0, 16384, 1025)}
	_, err := KittyTransmit(1, im, image.Point{}, image.Pt(1, 1))
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("expected byte limit error, got %v", err)
	}
}

func TestKittyTransmitScaledPlacement(t *testing.T) {
	b, err := KittyTransmitScaled(42, image.NewNRGBA(image.Rect(0, 0, 50, 25)), image.Pt(0, 10), image.Pt(10, 20), image.Pt(400, 200), kittygfx.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(",c=40,r=11")) {
		t.Fatalf("missing scaled destination: %q", b[:min(len(b), 180)])
	}
}
