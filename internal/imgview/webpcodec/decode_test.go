package webpcodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/color"
	"runtime"
	"testing"
)

type bitWriter struct {
	data []byte
	n    uint
}

func (b *bitWriter) put(v uint32, n uint) {
	for i := uint(0); i < n; i++ {
		if b.n%8 == 0 {
			b.data = append(b.data, 0)
		}
		b.data[b.n/8] |= byte((v>>i)&1) << (b.n % 8)
		b.n++
	}
}
func simple(b *bitWriter, v uint32) { b.put(1, 1); b.put(0, 1); b.put(1, 1); b.put(v, 8) }
func pixel(b *bitWriter, r, g, blue, a uint32) {
	b.put(0, 1)
	for _, v := range []uint32{g, r, blue, a, 0} {
		simple(b, v)
	}
}
func lossless(metaBomb bool) []byte {
	b := &bitWriter{}
	b.put(0x2f, 8)
	b.put(0, 14)
	b.put(0, 14)
	b.put(0, 4)
	b.put(0, 1) // no transforms
	b.put(0, 1) // no color cache
	if metaBomb {
		b.put(1, 1)
		b.put(0, 3)
		pixel(b, 255, 255, 0, 255)
	} else {
		b.put(0, 1)
		for _, v := range []uint32{34, 12, 56, 255, 0} {
			simple(b, v)
		}
	}
	return b.data
}
func chunk(tag string, p []byte) []byte {
	b := make([]byte, 8+len(p)+(len(p)&1))
	copy(b, tag)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(p)))
	copy(b[8:], p)
	return b
}
func webpRIFF(p []byte) []byte {
	b := make([]byte, 12)
	copy(b, "RIFF")
	copy(b[8:], "WEBP")
	binary.LittleEndian.PutUint32(b[4:], uint32(4+len(p)))
	return append(b, p...)
}
func alphaWebP(p []byte, compressed bool) []byte {
	x := make([]byte, 10)
	x[0] = 16
	flag := byte(0)
	if compressed {
		flag = 1
	}
	return webpRIFF(append(chunk("VP8X", x), chunk("ALPH", append([]byte{flag}, p...))...))
}

func TestLossless(t *testing.T) {
	m, e := Decode(bytes.NewReader(webpRIFF(chunk("VP8L", lossless(false)))), 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	if got := color.NRGBAModel.Convert(m.At(0, 0)); got != (color.NRGBA{12, 34, 56, 255}) {
		t.Fatalf("pixel: %v", got)
	}
}
func TestMetaHuffmanBombBeforeAllocation(t *testing.T) {
	for _, alpha := range []bool{false, true} {
		t.Run(map[bool]string{false: "VP8L", true: "compressedALPH"}[alpha], func(t *testing.T) {
			p := webpRIFF(chunk("VP8L", lossless(true)))
			if alpha {
				p = alphaWebP(lossless(true)[5:], true)
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, e := Decode(bytes.NewReader(p), 64<<20)
			runtime.ReadMemStats(&after)
			if !errors.Is(e, ErrLimit) {
				t.Fatalf("got %v, want allocation limit", e)
			}
			if n := after.TotalAlloc - before.TotalAlloc; n > 4<<20 {
				t.Fatalf("allocated %d bytes before rejection", n)
			}
		})
	}
}
func TestInvalidBudgets(t *testing.T) {
	p := webpRIFF(chunk("VP8L", lossless(false)))
	for _, budget := range []int64{-1, 0, 1, 1024} {
		_, e := Decode(bytes.NewReader(p), budget)
		if !errors.Is(e, ErrLimit) {
			t.Fatalf("budget %d: %v", budget, e)
		}
	}
}
func TestLargeLosslessCanvasBeforeAllocation(t *testing.T) {
	p := lossless(false)
	for i := 1; i < 4; i++ {
		p[i] = 255
	}
	p[4] = 15
	_, e := Decode(bytes.NewReader(webpRIFF(chunk("VP8L", p))), 1<<20)
	if !errors.Is(e, ErrLimit) {
		t.Fatalf("got %v", e)
	}
}
func TestRawAlphaBeforeAllocation(t *testing.T) {
	x := make([]byte, 10)
	x[0] = 16
	for i := 4; i < 10; i++ {
		x[i] = 255
	}
	p := webpRIFF(append(chunk("VP8X", x), chunk("ALPH", []byte{0})...))
	_, e := Decode(bytes.NewReader(p), 1<<20)
	if !errors.Is(e, ErrLimit) {
		t.Fatalf("got %v", e)
	}
}
func FuzzDecode(f *testing.F) {
	f.Add(webpRIFF(chunk("VP8L", lossless(false))))
	f.Add(webpRIFF(chunk("VP8L", lossless(true))))
	f.Add(alphaWebP(lossless(true)[5:], true))
	names, _ := fixtures.ReadDir("testdata")
	for _, name := range names {
		p, err := fixtures.ReadFile("testdata/" + name.Name())
		if err != nil {
			f.Fatal(err)
		}
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, p []byte) {
		if len(p) > 64<<10 {
			t.Skip()
		}
		_, _ = Decode(bytes.NewReader(p), 256<<10)
	})
}
