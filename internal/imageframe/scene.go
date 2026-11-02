// Package imageframe defines transport-independent image presentation contracts.
package imageframe

import "image"

// Viewport relates local pixels to absolute host cells. CellPixels is the
// measured or estimated size of one host cell; ExactPixels marks a measurement.
type Viewport struct {
	Cells       image.Rectangle
	PixelSize   image.Point
	CellPixels  image.Point
	ExactPixels bool
}

type Capabilities struct {
	Pixels, Alpha, ExactCellPixels, NativeAnimation bool
	MaxImageBytes                                   int64
}

// Raster pixels must not be mutated after publication. ID and Generation identify
// a version of a resource within its owner.
type Raster struct {
	ID, Generation uint64
	Pixels         *image.NRGBA
}

// Source is in raster pixels; Dest and Clip are viewport-local pixels.
type Placement struct {
	ID                 uint64
	Raster             *Raster
	Source, Dest, Clip image.Rectangle
}

// Scene.Generation is the owner lifecycle epoch, distinct from the requested
// presentation generation. Prepared frames retain this epoch.
type Scene struct {
	Owner, Generation uint64
	Viewport          Viewport
	Placements        []Placement
	Presentation      Presentation
	Playback          PlaybackState
}

// PlaybackState keeps user intent independent from whether the owner is visible.
type PlaybackState struct {
	Playing bool
	Visible bool
}

// Active reports whether user intent and visibility both allow playback.
func (s PlaybackState) Active() bool { return s.Playing && s.Visible }
