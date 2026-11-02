//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package imgview

import "image"

// MeasureCellPixels reports the zero point where no terminal size query exists.
func MeasureCellPixels() image.Point { return image.Point{} }
