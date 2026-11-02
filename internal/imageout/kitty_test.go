package imageout

import (
	"image"
	"strings"
	"testing"
)

func TestKittyGolden(t *testing.T) {
	q, err := KittyQuery(31)
	if err != nil {
		t.Fatal(err)
	}
	if string(q) != "\x1b_Ga=q,t=d,f=24,i=31,s=1,v=1;AAAA\x1b\\\x1b[c" {
		t.Fatalf("query %q", q)
	}
	im := &image.NRGBA{Pix: []byte{1, 2, 3, 4, 5, 6, 7, 8}, Stride: 8, Rect: image.Rect(10, 20, 12, 21)}
	out, err := KittyTransmit(42, im, image.Pt(19, 41), image.Pt(8, 16))
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[s\x1b[3;3H\x1b_Ga=T,t=d,f=32,i=42,p=1,s=2,v=1,X=3,Y=9,C=1,q=2,m=0;AQIDBAUGBwg=\x1b\\\x1b[u"
	if string(out) != want {
		t.Fatalf("transmit %q\nwant %q", out, want)
	}
	if got := string(KittyDelete(42)); got != "\x1b_Ga=d,i=42,q=2,d=I\x1b\\" {
		t.Fatalf("delete %q", got)
	}
}

func TestKittyContinuationGolden(t *testing.T) {
	im := image.NewNRGBA(image.Rect(0, 0, 1024, 1))
	out, err := KittyTransmit(9, im, image.Point{}, image.Pt(8, 16))
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[s\x1b[1;1H\x1b_Ga=T,t=d,f=32,i=9,p=1,s=1024,v=1,C=1,q=2,m=1;" + strings.Repeat("A", 4096) + "\x1b\\\x1b_Gm=0,q=2;" + strings.Repeat("A", 1366) + "==\x1b\\\x1b[u"
	if string(out) != want {
		t.Fatalf("continuation differs: got len %d want %d", len(out), len(want))
	}
}

func TestKittyInvalid(t *testing.T) {
	if _, err := KittyQuery(0); err == nil {
		t.Error("zero query id")
	}
	good := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	cases := []struct {
		id        uint32
		im        *image.NRGBA
		off, cell image.Point
	}{
		{0, good, image.Point{}, image.Pt(1, 1)}, {1, nil, image.Point{}, image.Pt(1, 1)},
		{1, good, image.Pt(-1, 0), image.Pt(1, 1)}, {1, good, image.Point{}, image.Pt(0, 1)},
		{1, &image.NRGBA{Rect: image.Rect(0, 0, 2, 2), Stride: 1, Pix: make([]byte, 16)}, image.Point{}, image.Pt(1, 1)},
		{1, &image.NRGBA{Rect: image.Rect(0, 0, 2, 2), Stride: 8, Pix: make([]byte, 15)}, image.Point{}, image.Pt(1, 1)},
		{1, &image.NRGBA{Rect: image.Rect(0, 0, 16385, 1)}, image.Point{}, image.Pt(1, 1)},
	}
	for _, tc := range cases {
		if _, err := KittyTransmit(tc.id, tc.im, tc.off, tc.cell); err == nil {
			t.Errorf("accepted %+v", tc)
		}
	}
	if len(KittyDelete(0)) != 0 {
		t.Error("zero id must not delete globally")
	}
}
