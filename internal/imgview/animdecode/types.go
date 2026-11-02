package animdecode

import (
	"context"
	"image"
	"image/color"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

// Limits bounds input, index storage, canvas pixels and resident decode work.
// MaxWorkingBytes excludes the separately bounded encoded input and metadata.
type Limits struct {
	MaxEncodedBytes  int64
	MaxPixels        int64
	MaxFrames        int
	MaxMetadataBytes int64
	MaxWorkingBytes  int64
	MinDelay         time.Duration
	GIFShortDelay    time.Duration
}

type disposal uint8

const (
	keep disposal = iota
	background
	previous
)

type frame struct {
	rect    image.Rectangle
	encoded time.Duration
	delay   time.Duration
	dispose disposal
	source  bool
	decode  func(context.Context) (image.Image, error)
}

type index struct {
	format     string
	size       image.Point
	plays      uint64
	frames     []frame
	background color.NRGBA
	poster     func(context.Context) (image.Image, error)
	// retained is a conservative byte charge for frame descriptors, closures,
	// chunk spans and synthesized container metadata (not encoded input).
	retained    int64
	eagerBytes  int64 // Decoded GIF frame storage, charged to the working budget.
	encodedWork int64
}

// Source is an on-demand, bounded full-canvas frame source. Calls are serialized.
// It owns an immutable copy of the encoded snapshot until Close.
type Source struct {
	meta   imageframe.SequenceMetadata
	idx    *index
	limits Limits
	data   []byte
	canvas *image.NRGBA
	next   int
}
