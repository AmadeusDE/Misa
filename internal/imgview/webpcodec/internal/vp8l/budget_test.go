package vp8l

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"unsafe"
)

func TestBudget(t *testing.T) {
	b := NewBudget(17)
	if b.Reserve(8) != nil || b.remaining != 9 {
		t.Fatal("reservation not charged")
	}
	for _, n := range []int64{-1, 10, math.MaxInt64} {
		if !errors.Is(b.Reserve(n), ErrLimit) || b.remaining != 9 {
			t.Fatalf("bad reservation %d changed budget", n)
		}
	}
	if b.Reserve(9) != nil || b.remaining != 0 || !errors.Is(b.Reserve(1), ErrLimit) {
		t.Fatal("boundary")
	}
}
func TestHuffmanAllocationGuards(t *testing.T) {
	h := hTree{}
	if err := h.buildSimple(1, [2]uint32{}, 256, NewBudget(7)); !errors.Is(err, ErrLimit) || h.nodes != nil {
		t.Fatalf("simple nodes: %v", err)
	}
	lengths := make([]uint32, 256)
	for i := range lengths {
		lengths[i] = 8
	}
	if err := h.build(lengths, NewBudget(8*511-1)); !errors.Is(err, ErrLimit) || h.nodes != nil {
		t.Fatalf("tree nodes: %v", err)
	}
	if _, err := codeLengthsToCodes(lengths, NewBudget(4*256-1)); !errors.Is(err, ErrLimit) {
		t.Fatalf("codes: %v", err)
	}
	if err := h.build(lengths, NewBudget(8*511+4*256-1)); !errors.Is(err, ErrLimit) {
		t.Fatalf("cumulative nodes/codes: %v", err)
	}
	if err := h.build(lengths, NewBudget(8*511+4*256)); err != nil {
		t.Fatal(err)
	}
}
func TestColorCacheAllocationGuard(t *testing.T) {
	d := decoder{r: bytes.NewReader([]byte{1 | 11<<1}), budget: NewBudget(4*(1<<11) - 1)}
	_, err := d.decodePix(1, 1, 0, false)
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("cache: %v", err)
	}
}
func TestGroupAllocationGuard(t *testing.T) {
	d := decoder{r: bytes.NewReader(nil), budget: NewBudget(int64(unsafe.Sizeof(hGroup{})) - 1)}
	_, _, _, err := d.decodeHuffmanGroups(1, 1, false, 0)
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("group: %v", err)
	}
}

type testBits struct {
	data []byte
	n    uint
}

func (b *testBits) put(v uint32, n uint) {
	for i := uint(0); i < n; i++ {
		if b.n%8 == 0 {
			b.data = append(b.data, 0)
		}
		b.data[b.n/8] |= byte((v>>i)&1) << (b.n % 8)
		b.n++
	}
}
func (b *testBits) pix(top bool) {
	b.put(0, 1)
	if top {
		b.put(0, 1)
	}
	for _, v := range []uint32{0, 0, 0, 255, 0} {
		b.put(1, 1)
		b.put(0, 1)
		b.put(1, 1)
		b.put(v, 8)
	}
}

func TestPixelCapacityAllocationGuard(t *testing.T) {
	b := testBits{}
	b.pix(false)
	cost := int64(unsafe.Sizeof(hGroup{})) + 5*8 + 1024
	d := decoder{r: bytes.NewReader(b.data), budget: NewBudget(cost - 1)}
	if _, err := d.decodePix(8, 1, 1024, false); !errors.Is(err, ErrLimit) {
		t.Fatalf("capacity: %v", err)
	}
	d = decoder{r: bytes.NewReader(b.data), budget: NewBudget(cost)}
	p, err := d.decodePix(8, 1, 1024, false)
	if err != nil || len(p) != 32 || cap(p) != 1024 || d.budget.remaining != 0 {
		t.Fatalf("capacity boundary: %d/%d %v", len(p), cap(p), err)
	}
}
func TestPaletteExpansionAllocationGuard(t *testing.T) {
	b := testBits{}
	b.put(0x2f, 8)
	b.put(15, 14)
	b.put(0, 14)
	b.put(0, 4)
	b.put(1, 1)
	b.put(transformTypeColorIndexing, 2)
	b.put(0, 8)
	b.pix(false)
	b.put(0, 1)
	b.pix(true)
	// One palette buffer (minCap 1024), two Huffman groups with five
	// singleton nodes each, packed pixels, and the expanded 16-pixel output.
	cost := 2*int64(unsafe.Sizeof(hGroup{})) + 2*5*8 + 1024 + 8 + 64
	if _, err := Decode(bytes.NewReader(b.data), NewBudget(cost-1)); !errors.Is(err, ErrLimit) {
		t.Fatalf("palette expansion: %v", err)
	}
	budget := NewBudget(cost)
	m, err := Decode(bytes.NewReader(b.data), budget)
	if err != nil || m.Bounds().Dx() != 16 || budget.remaining != 0 {
		t.Fatalf("expansion boundary: %v %v", m, err)
	}
}
