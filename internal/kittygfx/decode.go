package kittygfx

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
)

var _ image.Image = (*Raster)(nil)

// Raster is an immutable decoded image. It exposes no mutable slice; use At,
// Bounds, Format or CopyPixels.
type Raster struct {
	width, height int
	format        int
	pixels        []byte
}

func (r *Raster) ColorModel() color.Model { return color.NRGBAModel }

func (r *Raster) Bounds() image.Rectangle { return image.Rect(0, 0, r.width, r.height) }

// Format is the pixel layout the raster was decoded from: 24 (RGB) or 32 (RGBA).
func (r *Raster) Format() int { return r.format }

func (r *Raster) At(x, y int) color.Color {
	if x < 0 || y < 0 || x >= r.width || y >= r.height {
		return color.NRGBA{}
	}
	i := y*r.stride() + x*r.bytesPerPixel()
	if r.format == 24 {
		return color.NRGBA{r.pixels[i], r.pixels[i+1], r.pixels[i+2], 0xff}
	}
	return color.NRGBA{r.pixels[i], r.pixels[i+1], r.pixels[i+2], r.pixels[i+3]}
}

// CopyPixels copies the row-major RGB or RGBA pixels into dst, which must hold
// at least Width*Height*3 (format 24) or *4 (format 32) bytes.
func (r *Raster) CopyPixels(dst []byte) error {
	if len(dst) < len(r.pixels) {
		return fmt.Errorf("kittygfx: CopyPixels needs %d bytes, got %d", len(r.pixels), len(dst))
	}
	copy(dst, r.pixels)
	return nil
}

func (r *Raster) bytesPerPixel() int {
	if r.format == 24 {
		return 3
	}
	return 4
}

func (r *Raster) stride() int { return r.width * r.bytesPerPixel() }

// Decode decodes one complete encoded transfer into an immutable Raster.
// Only direct, raw RGB (f=24) and RGBA (f=32) transfers are supported.
func Decode(cmd Command, encoded []byte, lim Limits) (*Raster, error) {
	lim = lim.Normalize()
	switch cmd.Action {
	case ActionTransmit, ActionTransmitPlace:
	case ActionPlace, ActionDelete, ActionQuery:
		return nil, fmt.Errorf("%w: action %q carries no image data", ErrUnsupported, cmd.Action)
	default:
		return nil, fmt.Errorf("%w: action %q", ErrUnsupported, cmd.Action)
	}
	if cmd.Transmission != 0 && cmd.Transmission != 'd' {
		return nil, fmt.Errorf("%w: transmission medium %q", ErrUnsupported, cmd.Transmission)
	}
	var bytesPerPixel int
	switch cmd.Format {
	case 24:
		bytesPerPixel = 3
	case 32:
		bytesPerPixel = 4
	case 0:
		return nil, fmt.Errorf("%w: missing format", ErrMalformed)
	default:
		return nil, fmt.Errorf("%w: format %d", ErrUnsupported, cmd.Format)
	}
	if cmd.Width <= 0 || cmd.Height <= 0 {
		return nil, fmt.Errorf("%w: source size %dx%d", ErrMalformed, cmd.Width, cmd.Height)
	}
	if cmd.Width > lim.MaxWidth || cmd.Height > lim.MaxHeight {
		return nil, fmt.Errorf("%w: source size %dx%d exceeds %dx%d", ErrTooLarge,
			cmd.Width, cmd.Height, lim.MaxWidth, lim.MaxHeight)
	}
	size, ok := pixelBytes(cmd.Width, cmd.Height, bytesPerPixel)
	if !ok || size > lim.MaxImageBytes || size > lim.MaxTerminalBytes {
		return nil, fmt.Errorf("%w: %dx%d pixels do not fit the byte budget", ErrTooLarge, cmd.Width, cmd.Height)
	}
	if size > int64(maxInt) {
		return nil, fmt.Errorf("%w: %d pixel bytes do not fit an int", ErrTooLarge, size)
	}
	decoded, err := decodedLength(encoded)
	if err != nil {
		return nil, err
	}
	if decoded < size {
		return nil, fmt.Errorf("%w: %d decoded bytes for %d pixel bytes", ErrMalformed, decoded, size)
	}
	if decoded > size {
		return nil, fmt.Errorf("%w: %d decoded bytes for %d pixel bytes", ErrTooLarge, decoded, size)
	}
	pixels := make([]byte, size)
	if n, err := base64.StdEncoding.Decode(pixels, encoded); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	} else if int64(n) != size {
		return nil, fmt.Errorf("%w: %d decoded bytes for %d pixel bytes", ErrMalformed, n, size)
	}
	return &Raster{width: cmd.Width, height: cmd.Height, format: cmd.Format, pixels: pixels}, nil
}

// decodedLength is the exact byte count encoded holds. base64.DecodedLen alone
// is only an upper bound, and Decode into a destination of that upper bound's
// size would be a mismatch the standard library does not tolerate.
func decodedLength(encoded []byte) (int64, error) {
	if len(encoded)%4 != 0 {
		return 0, fmt.Errorf("%w: %d encoded bytes is not a base64 quantum", ErrMalformed, len(encoded))
	}
	padding := 0
	if n := len(encoded); n > 0 && encoded[n-1] == '=' {
		padding++
		if n > 1 && encoded[n-2] == '=' {
			padding++
		}
	}
	return int64(base64.StdEncoding.DecodedLen(len(encoded)) - padding), nil
}

const maxInt = int64(^uint(0) >> 1)

func pixelBytes(width, height, bytesPerPixel int) (int64, bool) {
	if width <= 0 || height <= 0 || bytesPerPixel <= 0 {
		return 0, false
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if int64(width) > maxInt64/int64(height) {
		return 0, false
	}
	area := int64(width) * int64(height)
	if area > maxInt64/int64(bytesPerPixel) {
		return 0, false
	}
	return area * int64(bytesPerPixel), true
}
