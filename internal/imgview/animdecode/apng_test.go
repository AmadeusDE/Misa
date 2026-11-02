package animdecode

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"testing"
	"time"
)

func testChunk(typ string, data []byte) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.WriteString(typ)
	b.Write(data)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(b.Bytes()[4:]))
	return b.Bytes()
}
func testPixels(t testing.TB, colors ...color.NRGBA) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zlib.NewWriter(&b)
	z.Write([]byte{0})
	for _, c := range colors {
		z.Write([]byte{c.R, c.G, c.B, c.A})
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func testPNGData(t testing.TB, c color.NRGBA) []byte { return testPixels(t, c) }
func apngFixture(t testing.TB, poster bool) []byte {
	t.Helper()
	var b bytes.Buffer
	b.Write([]byte("\x89PNG\r\n\x1a\n"))
	ih := make([]byte, 13)
	binary.BigEndian.PutUint32(ih, 2)
	binary.BigEndian.PutUint32(ih[4:], 1)
	ih[8] = 8
	ih[9] = 6
	b.Write(testChunk("IHDR", ih))
	ac := make([]byte, 8)
	binary.BigEndian.PutUint32(ac, 2)
	binary.BigEndian.PutUint32(ac[4:], 3)
	b.Write(testChunk("acTL", ac))
	seq := uint32(0)
	ctrl := func(x uint32, dis, blend byte) {
		p := make([]byte, 26)
		binary.BigEndian.PutUint32(p, seq)
		seq++
		binary.BigEndian.PutUint32(p[4:], 1)
		binary.BigEndian.PutUint32(p[8:], 1)
		binary.BigEndian.PutUint32(p[12:], x)
		binary.BigEndian.PutUint16(p[20:], 1)
		binary.BigEndian.PutUint16(p[22:], 10)
		p[24] = dis
		p[25] = blend
		b.Write(testChunk("fcTL", p))
	}
	red := testPNGData(t, color.NRGBA{255, 0, 0, 255})
	blue := testPNGData(t, color.NRGBA{0, 0, 255, 128})
	if poster { // Default image must match canvas dimensions.
		b.Write(testChunk("IDAT", testPixels(t, color.NRGBA{0, 255, 0, 255}, color.NRGBA{})))
		ctrl(0, 2, 0)
		d := append(make([]byte, 4), red...)
		binary.BigEndian.PutUint32(d, seq)
		seq++
		b.Write(testChunk("fdAT", d))
	} else { // Included first frame must cover the canvas.
		ctrl(0, 2, 0)
		raw := b.Bytes()
		_ = raw // replace first control width and image with a 2x1 image
		// fixture constructed separately below
		return apngIncludedFixture(t)
	}
	ctrl(1, 1, 1)
	d := append(make([]byte, 4), blue...)
	binary.BigEndian.PutUint32(d, seq)
	b.Write(testChunk("fdAT", d))
	b.Write(testChunk("IEND", nil))
	return b.Bytes()
}
func apngIncludedFixture(t testing.TB) []byte {
	t.Helper()
	var b bytes.Buffer
	b.Write([]byte("\x89PNG\r\n\x1a\n"))
	ih := make([]byte, 13)
	binary.BigEndian.PutUint32(ih, 1)
	binary.BigEndian.PutUint32(ih[4:], 1)
	ih[8] = 8
	ih[9] = 6
	b.Write(testChunk("IHDR", ih))
	ac := make([]byte, 8)
	binary.BigEndian.PutUint32(ac, 1)
	b.Write(testChunk("acTL", ac))
	fc := make([]byte, 26)
	binary.BigEndian.PutUint32(fc[4:], 1)
	binary.BigEndian.PutUint32(fc[8:], 1)
	fc[24] = 2
	b.Write(testChunk("fcTL", fc))
	b.Write(testChunk("IDAT", testPNGData(t, color.NRGBA{255, 0, 0, 255})))
	b.Write(testChunk("IEND", nil))
	return b.Bytes()
}
func apngLimits() Limits {
	return Limits{MaxEncodedBytes: 1 << 20, MaxPixels: 100, MaxFrames: 10, MaxMetadataBytes: 1 << 20, MaxWorkingBytes: 1 << 20, MinDelay: 10 * time.Millisecond}
}
func TestAPNGIndex(t *testing.T) {
	for _, poster := range []bool{false, true} {
		idx, err := indexAPNG(context.Background(), apngFixture(t, poster), apngLimits())
		if err != nil {
			t.Fatal(err)
		}
		if (idx.poster != nil) != poster {
			t.Fatal("poster mismatch")
		}
		if idx.frames[0].dispose != previous || !idx.frames[0].source {
			t.Fatal("first control")
		}
		im, err := idx.frames[0].decode(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if color.NRGBAModel.Convert(im.At(0, 0)) != (color.NRGBA{255, 0, 0, 255}) {
			t.Fatal("red pixel")
		}
		if poster {
			if idx.plays != 3 || len(idx.frames) != 2 || idx.frames[1].dispose != background || idx.frames[1].source || idx.frames[1].encoded != 100*time.Millisecond {
				t.Fatal("metadata")
			}
			im, err = idx.frames[1].decode(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if color.NRGBAModel.Convert(im.At(1, 0)) != (color.NRGBA{0, 0, 255, 128}) {
				t.Fatal("blue pixel")
			}
			im, err = idx.poster(context.Background())
			if err != nil || im.Bounds() != image.Rect(0, 0, 2, 1) {
				t.Fatalf("poster %v %v", im, err)
			}
		}
	}
}
func TestAPNGMalformed(t *testing.T) {
	b := apngFixture(t, true)
	for _, v := range [][]byte{b[:len(b)-1], append(append([]byte{}, b...), 0), append([]byte{}, b...)} {
		if len(v) == len(b) {
			v[29] ^= 1
		}
		if _, err := indexAPNG(context.Background(), v, apngLimits()); !errors.Is(err, ErrMalformed) {
			t.Errorf("got %v", err)
		}
	}
	l := apngLimits()
	l.MaxFrames = 1
	if _, err := indexAPNG(context.Background(), b, l); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit: %v", err)
	}
}
func FuzzAPNGIndex(f *testing.F) {
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Add(apngFixture(f, true))
	f.Add(apngIncludedFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			return
		}
		l := apngLimits()
		l.MaxEncodedBytes = 1 << 16
		idx, err := indexAPNG(context.Background(), b, l)
		if err == nil {
			for _, fr := range idx.frames {
				_, _ = fr.decode(context.Background())
			}
			if idx.poster != nil {
				_, _ = idx.poster(context.Background())
			}
			if source, openErr := Open(context.Background(), b, l); openErr == nil {
				_, _ = source.DecodeFrame(context.Background(), len(idx.frames)-1)
				_ = source.Close()
			}
		}
	})
}

func TestAPNGInvalidControls(t *testing.T) {
	base := apngFixture(t, true)
	mutate := func(typ string, nth int, change func([]byte)) []byte {
		b := append([]byte(nil), base...)
		for off := 8; off < len(b); {
			n := int(binary.BigEndian.Uint32(b[off:]))
			end := off + n + 12
			if string(b[off+4:off+8]) == typ {
				if nth == 0 {
					change(b[off+8 : end-4])
					binary.BigEndian.PutUint32(b[end-4:], crc32.ChecksumIEEE(b[off+4:end-4]))
					return b
				}
				nth--
			}
			off = end
		}
		t.Fatal("missing chunk")
		return nil
	}
	cases := [][]byte{
		mutate("acTL", 0, func(d []byte) { binary.BigEndian.PutUint32(d, 3) }),
		mutate("fcTL", 0, func(d []byte) { binary.BigEndian.PutUint32(d, 8) }),
		mutate("fcTL", 0, func(d []byte) { binary.BigEndian.PutUint32(d[12:], 2) }),
		mutate("fcTL", 0, func(d []byte) { d[24] = 3 }),
		mutate("fcTL", 1, func(d []byte) { d[25] = 2 }),
		mutate("fdAT", 0, func(d []byte) { binary.BigEndian.PutUint32(d, 9) }),
	}
	for i, b := range cases {
		if _, err := indexAPNG(context.Background(), b, apngLimits()); !errors.Is(err, ErrMalformed) {
			t.Errorf("case %d: %v", i, err)
		}
	}
	l := apngLimits()
	l.MaxMetadataBytes = 100
	if _, err := indexAPNG(context.Background(), base, l); !errors.Is(err, ErrLimit) {
		t.Fatalf("metadata limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := indexAPNG(ctx, base, apngLimits()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAPNGPaletteTransparencyAndComposition(t *testing.T) {
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	z.Write([]byte{0, 0})
	z.Close()
	var b bytes.Buffer
	b.Write([]byte("\x89PNG\r\n\x1a\n"))
	ih := make([]byte, 13)
	binary.BigEndian.PutUint32(ih, 1)
	binary.BigEndian.PutUint32(ih[4:], 1)
	ih[8] = 8
	ih[9] = 3
	b.Write(testChunk("IHDR", ih))
	b.Write(testChunk("PLTE", []byte{12, 34, 56}))
	b.Write(testChunk("tRNS", []byte{128}))
	ac := make([]byte, 8)
	binary.BigEndian.PutUint32(ac, 1)
	b.Write(testChunk("acTL", ac))
	fc := make([]byte, 26)
	binary.BigEndian.PutUint32(fc[4:], 1)
	binary.BigEndian.PutUint32(fc[8:], 1)
	b.Write(testChunk("fcTL", fc))
	b.Write(testChunk("IDAT", compressed.Bytes()))
	b.Write(testChunk("IEND", nil))
	idx, err := indexAPNG(context.Background(), b.Bytes(), apngLimits())
	if err != nil {
		t.Fatal(err)
	}
	im, err := idx.frames[0].decode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(im.At(0, 0)); got != (color.NRGBA{12, 34, 56, 128}) {
		t.Fatal(got)
	}
	idx, err = indexAPNG(context.Background(), apngFixture(t, true), apngLimits())
	if err != nil {
		t.Fatal(err)
	}
	s := &Source{idx: idx, meta: metadata(idx), limits: apngLimits()}
	first, err := s.DecodeFrame(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.Pixels.NRGBAAt(0, 0) != (color.NRGBA{255, 0, 0, 255}) || first.Pixels.NRGBAAt(1, 0) != (color.NRGBA{}) {
		t.Fatal("first canvas")
	}
	second, err := s.DecodeFrame(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if second.Pixels.NRGBAAt(0, 0) != (color.NRGBA{}) || second.Pixels.NRGBAAt(1, 0) != (color.NRGBA{0, 0, 255, 128}) {
		t.Fatal("previous disposal or poster contamination")
	}
}

func TestAPNGChunkOrderingAndSplitData(t *testing.T) {
	base := apngIncludedFixture(t)
	chunks := map[string][]byte{}
	for off := 8; off < len(base); {
		n := int(binary.BigEndian.Uint32(base[off:]))
		end := off + n + 12
		chunks[string(base[off+4:off+8])] = append([]byte(nil), base[off:end]...)
		off = end
	}
	makePNG := func(parts ...[]byte) []byte {
		out := []byte("\x89PNG\r\n\x1a\n")
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	idat := chunks["IDAT"]
	raw := idat[8 : len(idat)-4]
	mid := len(raw) / 2
	left, right := testChunk("IDAT", raw[:mid]), testChunk("IDAT", raw[mid:])
	split := makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], left, testChunk("IDAT", nil), right, chunks["IEND"])
	idx, err := indexAPNG(context.Background(), split, apngLimits())
	if err != nil {
		t.Fatal(err)
	}
	im, err := idx.frames[0].decode(context.Background())
	if err != nil || color.NRGBAModel.Convert(im.At(0, 0)) != (color.NRGBA{255, 0, 0, 255}) {
		t.Fatalf("split data: %v %v", im, err)
	}
	invalid := [][]byte{
		makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], left, testChunk("tEXt", []byte("a\x00b")), right, chunks["IEND"]),
		makePNG(chunks["IHDR"], chunks["fcTL"], chunks["acTL"], idat, chunks["IEND"]),
		makePNG(chunks["IHDR"], idat, chunks["acTL"], chunks["fcTL"], chunks["IEND"]),
		makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], idat, testChunk("IEND", []byte{0})),
		makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], idat, testChunk("PLTE", []byte{0, 0, 0}), chunks["IEND"]),
		makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], idat, testChunk("tRNS", []byte{0}), chunks["IEND"]),
		makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], idat, testChunk("ABCD", nil), chunks["IEND"]),
		makePNG(chunks["IHDR"], chunks["acTL"], chunks["fcTL"], idat, testChunk("gAMA", []byte{0, 0, 177, 143}), chunks["IEND"]),
		makePNG(chunks["IHDR"], testChunk("gAMA", []byte{0, 0, 177, 143}), testChunk("gAMA", []byte{0, 0, 177, 143}), chunks["acTL"], chunks["fcTL"], idat, chunks["IEND"]),
		makePNG(chunks["IHDR"], testChunk("pHYs", []byte{0}), chunks["acTL"], chunks["fcTL"], idat, chunks["IEND"]),
	}
	for i, b := range invalid {
		if _, err := indexAPNG(context.Background(), b, apngLimits()); !errors.Is(err, ErrMalformed) {
			t.Errorf("case %d: %v", i, err)
		}
	}
}
