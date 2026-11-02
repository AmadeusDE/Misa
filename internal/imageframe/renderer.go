package imageframe

import (
	"context"
	"image"
	"image/color"
)

// Presentation identifies the requested image, not a physical display event.
// Generation increases for each requested presentation within an owner epoch.
type Presentation struct {
	SequenceID uint64
	FrameIndex int
	Generation uint64
}

// Acknowledgement means a frame's effects were serialized and committed to the
// output boundary. It does not establish that the physical display showed it.
// Generation is the committed frame’s owner lifecycle epoch.
type Acknowledgement struct {
	Owner        uint64
	Generation   uint64
	Presentation Presentation
}

// Cell is a transport-neutral cell, with Text containing its displayed grapheme.
type Cell struct {
	Text                   string
	Foreground, Background color.NRGBA
}

// CellLayer replaces the rectangular cell area in row-major order. Nil on Frame
// means no cell update; a non-nil empty layer is distinct from no update.
type CellLayer struct {
	Bounds image.Rectangle
	Cells  []Cell
}

type EffectKind uint8

const (
	Transfer EffectKind = iota + 1
	Place
	Delete
)

// Effect identifies resources in the owner namespace. Transfer carries Raster;
// Place carries PlacementData; Delete targets Resource and optionally Placement
// (zero Placement deletes the resource). The commit phases are deletes, the
// complete text and cell layers, transfers, then placements. Order is preserved
// within each phase. A producer must not rely on ordering across phases.
type Effect struct {
	Kind                       EffectKind
	Owner, Resource, Placement uint64
	Raster                     *Raster
	PlacementData              *Placement
}

// Frame.Generation is the owner lifecycle epoch from Scene.Generation. The host
// invalidates it on lifecycle changes such as resize, hide, or release.
type Frame struct {
	Owner        uint64
	Generation   uint64
	Cells        *CellLayer
	Effects      []Effect
	Presentation Presentation
}

// Renderer prepares immutable output for the host's serialized commit. Prepare,
// Release and Close must not write to any output stream. Release affects only
// resources owned by owner; Close releases this renderer's resources.
type Renderer interface {
	Capabilities() Capabilities
	Prepare(context.Context, Scene) (Frame, error)
	Release(owner uint64) Frame
	Close() Frame
}
