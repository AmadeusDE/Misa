package imageframe

import (
	"context"
	"image"
	"time"
)

// SequenceMetadata describes a bounded sequence. Plays is total plays; zero is
// infinite. Frames counts animation frames, excluding any separate poster.
type SequenceMetadata struct {
	ID     uint64
	Size   image.Point
	Frames int
	Plays  uint64
}

// DecodedFrame is a full-canvas immutable image with its effective duration.
type DecodedFrame struct {
	Index    int
	Pixels   *image.NRGBA
	Duration time.Duration
}

// FrameSource provides on-demand frames rather than an eager frame slice.
// Implementations must impose finite input, metadata, working-memory and decode
// limits, honor cancellation where possible, and release resources on Close.
// A decoder that cannot interrupt an individual codec call must not publish its
// result after cancellation. Callers must not use a source after Close. Calls
// must be serialized unless an implementation documents concurrent use as safe.
type FrameSource interface {
	Metadata() SequenceMetadata
	DecodeFrame(context.Context, int) (DecodedFrame, error)
	Close() error
}

// Clock permits deterministic playback tests. Scheduling policy belongs to the
// playback controller, not to the renderer or frame source.
type Clock interface {
	Now() time.Time
	NewTimer(time.Duration) Timer
}

// Timer can be stopped when an owner is hidden or playback is paused. Stop
// reports whether it stopped a timer that had not already fired or stopped.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}
