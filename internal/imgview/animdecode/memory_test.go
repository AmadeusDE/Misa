package animdecode

import (
	"context"
	"encoding/binary"
	"errors"
	"image"
	"testing"
)

func TestAggregateMemoryReservation(t *testing.T) {
	f := frame{rect: image.Rect(0, 0, 1, 1), decode: func(context.Context) (image.Image, error) { return image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil }}
	idx := &index{format: "apng", size: image.Pt(1, 1), frames: []frame{f}, retained: 4096}
	l := DefaultLimits()
	l.MaxWorkingBytes = 1 << 18
	if !errors.Is(checkIndex(idx, l), ErrLimit) {
		t.Fatal("fixed codec and per-span scratch not reserved")
	}
	l.MaxWorkingBytes = 1 << 20
	if err := checkIndex(idx, l); err != nil {
		t.Fatal(err)
	}
	idx.retained = 1 << 20
	if !errors.Is(checkIndex(idx, l), ErrLimit) {
		t.Fatal("per-span scratch excluded from working budget")
	}
}
func TestWorkingReservationOverflow(t *testing.T) {
	l := DefaultLimits()
	l.MaxPixels = int64(^uint64(0) >> 1)
	l.MaxWorkingBytes = int64(^uint64(0) >> 1)
	idx := &index{size: image.Pt(int(^uint(0)>>1)/4, 1), frames: []frame{{rect: image.Rect(0, 0, 1, 1), decode: func(context.Context) (image.Image, error) { return nil, nil }}}}
	if !errors.Is(checkIndex(idx, l), ErrLimit) {
		t.Fatal("overflowing reservation accepted")
	}
}

func TestEagerFramesUseWorkingNotMetadataBudget(t *testing.T) {
	f := frame{rect: image.Rect(0, 0, 1, 1), decode: func(context.Context) (image.Image, error) { return image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil }}
	idx := &index{size: image.Pt(1, 1), frames: []frame{f}, retained: 512, eagerBytes: 64 << 10}
	l := DefaultLimits()
	l.MaxMetadataBytes = 512
	l.MaxWorkingBytes = 1 << 20
	if err := checkIndex(idx, l); err != nil {
		t.Fatalf("eager pixels charged as metadata: %v", err)
	}
	l.MaxWorkingBytes = (512 << 10) + 512 + (64 << 10) + 31
	if !errors.Is(checkIndex(idx, l), ErrLimit) {
		t.Fatal("eager memory not included in working reservation")
	}
	idx.eagerBytes = -1
	if !errors.Is(checkIndex(idx, DefaultLimits()), ErrLimit) {
		t.Fatal("negative eager charge")
	}
}

func TestGIFPredecodeUsesWorkingBudget(t *testing.T) {
	l := DefaultLimits()
	l.MaxMetadataBytes = 1024
	idx, err := indexGIF(context.Background(), gifFixture(t), l)
	if err != nil {
		t.Fatal(err)
	}
	if idx.eagerBytes == 0 {
		t.Fatal("small GIF eager pixels should use working budget, not metadata budget")
	}
	if idx.retained > l.MaxMetadataBytes {
		t.Fatal("index metadata exceeds budget")
	}
}

func TestCodecWorkingBudget(t *testing.T) {
	idx := &index{size: image.Pt(10, 5), retained: 400, eagerBytes: 300, encodedWork: 200}
	l := DefaultLimits()
	l.MaxWorkingBytes = 2 << 20
	got := codecWorkingBudget(idx, l)
	const outside = int64((512 << 10) + 12*10*5 + 400 + 300 + 200)
	if got != l.MaxWorkingBytes-outside {
		t.Fatalf("codec budget = %d, outside = %d", got, outside)
	}
	if got+outside > l.MaxWorkingBytes {
		t.Fatal("combined reservations exceed budget")
	}
	l.MaxWorkingBytes = outside - 1
	if got := codecWorkingBudget(idx, l); got != 0 {
		t.Fatalf("insufficient budget = %d", got)
	}
	idx.retained = -1
	if got := codecWorkingBudget(idx, DefaultLimits()); got != 0 {
		t.Fatalf("negative charge budget = %d", got)
	}
}

func TestCodecWorkingBudgetRejectsOverflow(t *testing.T) {
	l := DefaultLimits()
	l.MaxPixels = int64(^uint64(0) >> 1)
	l.MaxWorkingBytes = int64(^uint64(0) >> 1)
	idx := &index{size: image.Pt(int(^uint(0)>>1)/4, 1)}
	if got := codecWorkingBudget(idx, l); got != 0 {
		t.Fatalf("overflowing pixel reservation = %d", got)
	}
}

func TestPNG16BitWideCanvasReservation(t *testing.T) {
	const width = 10000
	f := frame{rect: image.Rect(0, 0, width, 1), decode: func(context.Context) (image.Image, error) { return nil, nil }}
	idx := &index{format: "apng", size: image.Pt(width, 1), frames: []frame{f}}
	l := DefaultLimits()
	l.MaxWorkingBytes = (512 << 10) + 40*width
	if !errors.Is(checkIndex(idx, l), ErrLimit) {
		t.Fatal("PNG16 row buffers and interlace pass not reserved")
	}
	l.MaxWorkingBytes = (512 << 10) + 48*width
	if err := checkIndex(idx, l); err != nil {
		t.Fatalf("exact reservation: %v", err)
	}
}

func TestWebPReservesFramePayloadNotWholeSequence(t *testing.T) {
	b := wpFixture(t)
	// Bounded ignored metadata remains in the encoded snapshot, but is never
	// copied by the frame codec. It must not consume the codec-work allowance.
	b = append(b, wpChunk("JUNK", make([]byte, 1<<20))...)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	s, err := Open(context.Background(), b, Limits{MaxWorkingBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DecodeFrame(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
}
