package imgview

import (
	"context"
	"errors"
	"image"
	"testing"

	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

func TestCacheWorkingReservation(t *testing.T) {
	c := NewCache(12)
	pixel := func() Decoded { return Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 1, 1))} }
	c.Put("a", pixel())
	c.Put("b", pixel())
	c.Put("c", pixel())
	release, err := c.reserve(8)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bytes() != 4 {
		t.Fatal(c.Bytes())
	}
	if c.Put("d", Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 2, 1))}) {
		t.Fatal("reservation ignored")
	}
	if _, err := c.reserve(8); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	release()
	release() // Releases are idempotent.
	if !c.Put("d", Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 2, 1))}) {
		t.Fatal("reservation leaked")
	}
	if c.Bytes() != 12 {
		t.Fatal(c.Bytes())
	}
}

func TestAnimationPosterSharesCacheBudget(t *testing.T) {
	c := NewCache(1 << 20)
	c.Put("old", Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 2, 2))})
	l := Limits{workCache: c, Animation: animdecode.Limits{MaxWorkingBytes: 1 << 20}}
	_, err := DecodeContext(context.Background(), animationFile(t, integrationAPNG(t)), l)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bytes() != 0 {
		t.Fatal("animation working reservation did not evict cached pixels")
	}
	if !c.Put("new", Decoded{Pixels: image.NewNRGBA(image.Rect(0, 0, 2, 2))}) {
		t.Fatal("reservation leaked")
	}
}
