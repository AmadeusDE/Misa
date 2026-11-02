// Package animdecode indexes and composes bounded animated raster images.
package animdecode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

var (
	ErrLimit     = errors.New("animation limit exceeded")
	ErrMalformed = errors.New("malformed animation")
	ErrClosed    = errors.New("animation source closed")
	sequenceID   atomic.Uint64
)

func DefaultLimits() Limits {
	l := Limits{MaxEncodedBytes: 64 << 20, MaxPixels: 32_000_000, MaxFrames: 10000, MaxMetadataBytes: 8 << 20, MaxWorkingBytes: 64 << 20, MinDelay: 20 * time.Millisecond, GIFShortDelay: 100 * time.Millisecond}
	if runtime.GOOS == "android" {
		l.MaxFrames = 2000
		l.MaxMetadataBytes = 2 << 20
		l.MaxWorkingBytes = 24 << 20
	}
	return l
}

func normalizeLimits(l Limits) Limits {
	d := DefaultLimits()
	if l.MaxEncodedBytes <= 0 {
		l.MaxEncodedBytes = d.MaxEncodedBytes
	}
	if l.MaxPixels <= 0 {
		l.MaxPixels = d.MaxPixels
	}
	if l.MaxFrames <= 0 {
		l.MaxFrames = d.MaxFrames
	}
	if l.MaxMetadataBytes <= 0 {
		l.MaxMetadataBytes = d.MaxMetadataBytes
	}
	if l.MaxWorkingBytes <= 0 {
		l.MaxWorkingBytes = d.MaxWorkingBytes
	}
	if l.MinDelay <= 0 {
		l.MinDelay = d.MinDelay
	}
	if l.GIFShortDelay <= 0 {
		l.GIFShortDelay = d.GIFShortDelay
	}
	return l
}
func validSize(w, h int, l Limits) bool {
	return w > 0 && h > 0 && int64(w) <= l.MaxPixels/int64(h) && int64(w) <= int64(^uint(0)>>1)/4/int64(h)
}
func effectiveDelay(d time.Duration, gif bool, l Limits) time.Duration {
	if gif && d <= 10*time.Millisecond {
		d = l.GIFShortDelay
	}
	if d < l.MinDelay {
		d = l.MinDelay
	}
	return d
}
func checkIndex(idx *index, l Limits) error {
	if idx == nil || !validSize(idx.size.X, idx.size.Y, l) {
		return ErrLimit
	}
	if len(idx.frames) == 0 {
		return ErrMalformed
	}
	if len(idx.frames) > l.MaxFrames || idx.retained < 0 || idx.retained > l.MaxMetadataBytes {
		return ErrLimit
	}
	// Reserve three 4-byte canvas equivalents, PNG 16-bit full image and
	// interlaced pass (8 bytes each), and two 8-byte-per-pixel rows. The
	// 48-byte factor covers all of these even for a one-row image. APNG's retained charge also bounds the temporary chunk reader graph.
	const fixed = int64(512 << 10)
	if l.MaxWorkingBytes < fixed {
		return ErrLimit
	}
	budget := l.MaxWorkingBytes - fixed
	if idx.retained > budget || idx.encodedWork < 0 {
		return ErrLimit
	}
	budget -= idx.retained
	if idx.encodedWork > budget {
		return ErrLimit
	}
	budget -= idx.encodedWork
	if idx.eagerBytes < 0 || idx.eagerBytes > budget {
		return ErrLimit
	}
	budget -= idx.eagerBytes
	pixels := int64(idx.size.X) * int64(idx.size.Y)
	if pixels > budget/48 {
		return ErrLimit
	}
	for _, f := range idx.frames {
		if f.rect.Empty() || !f.rect.In(imageBounds(idx)) || f.decode == nil || f.dispose > previous {
			return ErrMalformed
		}
	}
	return nil
}

// codecWorkingBudget leaves the compositor, index and fixed overhead reserved
// while a codec is active. The codec must enforce the returned residual budget.
func codecWorkingBudget(idx *index, l Limits) int64 {
	if idx == nil || !validSize(idx.size.X, idx.size.Y, l) || l.MaxWorkingBytes < 0 {
		return 0
	}
	budget := l.MaxWorkingBytes
	for _, charge := range []int64{512 << 10, idx.retained, idx.eagerBytes, idx.encodedWork} {
		if charge < 0 || charge > budget {
			return 0
		}
		budget -= charge
	}
	pixels := int64(idx.size.X) * int64(idx.size.Y)
	if pixels > budget/12 {
		return 0
	}
	return budget - pixels*12
}
func metadata(idx *index) imageframe.SequenceMetadata {
	return imageframe.SequenceMetadata{ID: sequenceID.Add(1), Size: idx.size, Frames: len(idx.frames), Plays: idx.plays}
}
func Open(ctx context.Context, data []byte, l Limits) (*Source, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l = normalizeLimits(l)
	if int64(len(data)) > l.MaxEncodedBytes {
		return nil, ErrLimit
	}
	b := bytes.Clone(data)
	var idx *index
	var err error
	switch {
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		idx, err = indexGIF(ctx, b, l)
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		idx, err = indexAPNG(ctx, b, l)
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		idx, err = indexWebP(ctx, b, l)
	default:
		return nil, fmt.Errorf("%w: unknown format", ErrMalformed)
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = checkIndex(idx, l); err != nil {
		return nil, err
	}
	return &Source{idx: idx, meta: metadata(idx), limits: l, data: b}, nil
}
func (s *Source) Metadata() imageframe.SequenceMetadata { return s.meta }
func (s *Source) Format() string {
	if s.idx == nil {
		return ""
	}
	return s.idx.format
}
func (s *Source) FrameTiming(i int) (time.Duration, time.Duration, error) {
	if s.idx == nil {
		return 0, 0, ErrClosed
	}
	if i < 0 || i >= len(s.idx.frames) {
		return 0, 0, fmt.Errorf("frame %d out of range", i)
	}
	f := s.idx.frames[i]
	return f.encoded, f.delay, nil
}
func (s *Source) Close() error { s.idx = nil; s.data = nil; s.canvas = nil; return nil }
