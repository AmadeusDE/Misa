package imgview

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

func animationFile(t *testing.T, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "animation")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func integrationAPNG(t *testing.T) []byte {
	t.Helper()
	chunk := func(typ string, data []byte) []byte {
		out := make([]byte, 12+len(data))
		binary.BigEndian.PutUint32(out, uint32(len(data)))
		copy(out[4:], typ)
		copy(out[8:], data)
		binary.BigEndian.PutUint32(out[8+len(data):], crc32.ChecksumIEEE(out[4:8+len(data)]))
		return out
	}
	makePNG := func(c color.NRGBA) []byte {
		p := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		p.SetNRGBA(0, 0, c)
		var b bytes.Buffer
		if err := png.Encode(&b, p); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	poster := makePNG(color.NRGBA{B: 255, A: 255})
	frame := makePNG(color.NRGBA{R: 255, A: 255})
	out := append([]byte{}, poster[:33]...)
	actl := make([]byte, 8)
	binary.BigEndian.PutUint32(actl, 1)
	binary.BigEndian.PutUint32(actl[4:], 2)
	out = append(out, chunk("acTL", actl)...)
	for p := 33; p < len(poster); {
		n := int(binary.BigEndian.Uint32(poster[p:]))
		if string(poster[p+4:p+8]) == "IDAT" {
			out = append(out, poster[p:p+12+n]...)
		}
		p += 12 + n
	}
	fctl := make([]byte, 26)
	binary.BigEndian.PutUint32(fctl[4:], 1)
	binary.BigEndian.PutUint32(fctl[8:], 1)
	binary.BigEndian.PutUint16(fctl[20:], 1)
	out = append(out, chunk("fcTL", fctl)...)
	seq := uint32(1)
	for p := 33; p < len(frame); {
		n := int(binary.BigEndian.Uint32(frame[p:]))
		if string(frame[p+4:p+8]) == "IDAT" {
			data := make([]byte, 4+n)
			binary.BigEndian.PutUint32(data, seq)
			seq++
			copy(data[4:], frame[p+8:p+8+n])
			out = append(out, chunk("fdAT", data)...)
		}
		p += 12 + n
	}
	return append(out, chunk("IEND", nil)...)
}

func TestAnimationSourceAndPosterIntegration(t *testing.T) {
	ctx := context.Background()
	path := animationFile(t, integrationAPNG(t))
	s, err := OpenAnimation(ctx, path, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if m := s.Metadata(); m.Frames != 1 || m.Plays != 2 {
		t.Fatal(m)
	}
	f, err := s.DecodeFrame(ctx, 0)
	if err != nil || f.Pixels.NRGBAAt(0, 0) != (color.NRGBA{R: 255, A: 255}) {
		t.Fatalf("frame %+v %v", f, err)
	}
	d, err := Decode(path, Limits{})
	if err != nil || !d.Animated || d.Format != "png" || d.Pixels.NRGBAAt(0, 0) != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("poster %+v %v", d, err)
	}
	if _, err := OpenAnimation(ctx, path, Limits{Animation: animdecode.Limits{MaxWorkingBytes: 1}}); !errors.Is(err, ErrLimit) {
		t.Fatalf("working limit: %v", err)
	}
}

func TestAnimationGIFIntegrationLimitsAndTiming(t *testing.T) {
	p := color.Palette{color.Transparent, color.NRGBA{R: 255, A: 255}}
	a := image.NewPaletted(image.Rect(0, 0, 1, 1), p)
	a.SetColorIndex(0, 0, 1)
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{a, a}, Delay: []int{0, 2}, LoopCount: 2}); err != nil {
		t.Fatal(err)
	}
	path := animationFile(t, b.Bytes())
	l := Limits{Animation: animdecode.Limits{GIFShortDelay: 150 * time.Millisecond}}
	s, err := OpenAnimation(context.Background(), path, l)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Metadata().Plays != 3 {
		t.Fatal(s.Metadata())
	}
	f, err := s.DecodeFrame(context.Background(), 0)
	if err != nil || f.Duration != 150*time.Millisecond {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := Decode(path, Limits{Animation: animdecode.Limits{MaxFrames: 1}}); !errors.Is(err, ErrLimit) {
		t.Fatalf("frame limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := OpenAnimation(ctx, path, l); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := OpenAnimation(context.Background(), t.TempDir(), l); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestAnimationRejectsIncompleteAPNGInsteadOfPosterFallback(t *testing.T) {
	b := integrationAPNG(t)
	// The advertised animation count cannot be satisfied, even though its PNG
	// default image remains perfectly decodable by the static codec.
	p := bytes.Index(b, []byte("acTL"))
	binary.BigEndian.PutUint32(b[p+4:], 99)
	binary.BigEndian.PutUint32(b[p+12:], crc32.ChecksumIEEE(b[p:p+12]))
	if _, err := Decode(animationFile(t, b), Limits{}); err == nil {
		t.Fatal("malformed animation fell back to static poster")
	}
}

func TestStaticWebPUsesBoundedCodec(t *testing.T) {
	// Original synthetic 2x2 RGBA fixture, encoded with ffmpeg/libwebp.
	b, err := base64.StdEncoding.DecodeString("UklGRjYAAABXRUJQVlA4TCkAAAAvAUAAEB8gEEiS3R9hDYEAwf+YKyMjICE8l+UCsFdVKAkQAEUZieh/MAA=")
	if err != nil {
		t.Fatal(err)
	}
	path := animationFile(t, b)
	if _, err := Decode(path, Limits{Animation: animdecode.Limits{MaxWorkingBytes: 1}}); !errors.Is(err, ErrLimit) {
		t.Fatalf("static WebP bypassed codec budget: %v", err)
	}
	d, err := Decode(path, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Pixels.NRGBAAt(1, 0); got != (color.NRGBA{G: 255, A: 128}) {
		t.Fatal(got)
	}
}

func TestStaticWebPRejectsHiddenCodecDimensionsBeforeDecode(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString("UklGRjYAAABXRUJQVlA4TCkAAAAvAUAAEB8gEEiS3R9hDYEAwf+YKyMjICE8l+UCsFdVKAkQAEUZieh/MAA=")
	if err != nil {
		t.Fatal(err)
	}
	x := make([]byte, 18)
	copy(x, "VP8X")
	binary.LittleEndian.PutUint32(x[4:], 10)
	payload := append(x, b[12:]...)
	data := append([]byte{}, b[:12]...)
	binary.LittleEndian.PutUint32(data[4:], uint32(4+len(payload)))
	data = append(data, payload...)
	_, err = Decode(animationFile(t, data), Limits{Animation: animdecode.Limits{MaxWorkingBytes: 1}})
	if err == nil || errors.Is(err, ErrLimit) {
		t.Fatalf("compressed dimensions were not checked before codec allocation: %v", err)
	}
}
