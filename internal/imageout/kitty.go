package imageout

import (
	"encoding/base64"
	"fmt"
	"image"
	"strconv"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

const kittyChunk = 4096
const kittyMaxBytes = 64 << 20

// KittyQuery sends a direct RGB probe followed by DA1, whose reply bounds the
// query response window. IDs must be nonzero and unique among pending probes.
func KittyQuery(id uint32) ([]byte, error) {
	if id == 0 {
		return nil, fmt.Errorf("kitty: zero image ID")
	}
	b, err := kittygfx.Encode(kittygfx.Command{Action: kittygfx.ActionQuery, Transmission: 'd', Format: 24, ID: id, Width: 1, Height: 1}, []byte("AAAA"))
	if err != nil {
		return nil, err
	}
	return append(b, "\x1b[c"...), nil
}

// KittyTransmit sends one straight-alpha RGBA image and places it at the
// viewport-relative pixel offset. cell gives the positive pixel cell size.
// Images are limited to 16384 pixels per dimension and 64 MiB of RGBA data;
// callers should downscale larger images before transmission.
// The caller owns id and must delete only its own images. Cursor state is saved
// and restored; transmissions with the same id replace the previous image.
func KittyTransmit(id uint32, pixels *image.NRGBA, offset, cell image.Point) ([]byte, error) {
	return KittyTransmitLimited(id, pixels, offset, cell, kittygfx.DefaultLimits())
}

// KittyTransmitLimited applies the receiver's normalized graphics limits as
// well as the output helper's 64 MiB cap. MaxPayload conservatively includes
// the full APC framing and control header, not just its encoded payload.
func KittyTransmitLimited(id uint32, pixels *image.NRGBA, offset, cell image.Point, limits kittygfx.Limits) ([]byte, error) {
	return kittyTransmit(id, pixels, offset, cell, image.Point{}, limits)
}

// KittyTransmitScaled places a reduced raster in a display-sized cell rectangle.
// Kitty positions placements in cells, so the far edge rounds up to a cell.
func KittyTransmitScaled(id uint32, pixels *image.NRGBA, offset, cell, display image.Point, limits kittygfx.Limits) ([]byte, error) {
	if display.X <= 0 || display.Y <= 0 {
		return nil, fmt.Errorf("kitty: invalid display size")
	}
	return kittyTransmit(id, pixels, offset, cell, display, limits)
}

func kittyTransmit(id uint32, pixels *image.NRGBA, offset, cell, display image.Point, limits kittygfx.Limits) ([]byte, error) {
	limits = limits.Normalize()
	if id == 0 || pixels == nil || offset.X < 0 || offset.Y < 0 || cell.X <= 0 || cell.Y <= 0 {
		return nil, fmt.Errorf("kitty: invalid image, ID or geometry")
	}
	r := pixels.Rect
	if r.Max.X <= r.Min.X || r.Max.Y <= r.Min.Y {
		return nil, fmt.Errorf("kitty: empty image")
	}
	w, h := uint(r.Max.X)-uint(r.Min.X), uint(r.Max.Y)-uint(r.Min.Y)
	if w > 16384 || h > 16384 || w > uint(limits.MaxWidth) || h > uint(limits.MaxHeight) || w*h*4 > kittyMaxBytes || int64(w*h*4) > limits.MaxImageBytes || int64(w*h*4) > limits.MaxTerminalBytes {
		return nil, fmt.Errorf("kitty: image exceeds limits")
	}
	width, height := int(w), int(h)
	row := width * 4
	if pixels.Stride < row || len(pixels.Pix) < row || (height-1) > (len(pixels.Pix)-row)/pixels.Stride {
		return nil, fmt.Errorf("kitty: invalid pixel stride or buffer")
	}
	maxInt := int(^uint(0) >> 1)
	col, line := offset.X/cell.X, offset.Y/cell.Y
	if col == maxInt || line == maxInt {
		return nil, fmt.Errorf("kitty: cursor position overflow")
	}
	encodedLen := base64.StdEncoding.EncodedLen(row * height)
	if int64(encodedLen) > limits.MaxInFlightBytes {
		return nil, fmt.Errorf("kitty: encoded image exceeds in-flight limit")
	}
	// Build the first control header separately so even unusually small payload
	// limits account for every APC byte. Continuation headers are shorter.
	header := fmt.Appendf(nil, "\x1b_Ga=T,t=d,f=32,i=%d,p=1,s=%d,v=%d", id, width, height)
	if x := offset.X % cell.X; x != 0 {
		header = fmt.Appendf(header, ",X=%d", x)
	}
	if y := offset.Y % cell.Y; y != 0 {
		header = fmt.Appendf(header, ",Y=%d", y)
	}
	if display != (image.Point{}) {
		if display.X > maxInt-offset.X%cell.X || display.Y > maxInt-offset.Y%cell.Y {
			return nil, fmt.Errorf("kitty: display size overflow")
		}
		cols := 1 + (offset.X%cell.X+display.X-1)/cell.X
		rows := 1 + (offset.Y%cell.Y+display.Y-1)/cell.Y
		header = fmt.Appendf(header, ",c=%d,r=%d", cols, rows)
	}
	header = append(header, ",C=1,q=2"...)
	// ,m=N; and ST follow the first header.
	chunk := min(kittyChunk, limits.MaxPayload-len(header)-7)
	chunk -= chunk % 4
	if chunk < 4 {
		return nil, fmt.Errorf("kitty: payload limit cannot fit header and base64 data")
	}
	raw := make([]byte, row*height)
	for y := 0; y < height; y++ {
		copy(raw[y*row:(y+1)*row], pixels.Pix[y*pixels.Stride:y*pixels.Stride+row])
	}
	encoded := make([]byte, encodedLen)
	base64.StdEncoding.Encode(encoded, raw)
	out := make([]byte, 0, len(encoded)+len(encoded)/chunk*24+160)
	out = fmt.Appendf(out, "\x1b[s\x1b[%d;%dH", line+1, col+1)
	out = append(out, header...)
	for pos := 0; pos < len(encoded); {
		end := pos + chunk
		if end > len(encoded) {
			end = len(encoded)
		}
		more := '0'
		if end < len(encoded) {
			more = '1'
		}
		if pos > 0 {
			out = append(out, "\x1b_Gm="...)
			out = append(out, byte(more))
			out = append(out, ",q=2"...)
		} else {
			out = append(out, ",m="...)
			out = append(out, byte(more))
		}
		out = append(out, ';')
		out = append(out, encoded[pos:end]...)
		out = append(out, "\x1b\\"...)
		pos = end
	}
	return append(out, "\x1b[u"...), nil
}

// KittyDelete deletes the specified owned image and its placements. Zero is a
// no-op, never a request to delete all images.
func KittyDelete(id uint32) []byte {
	if id == 0 {
		return nil
	}
	out := append([]byte("\x1b_Ga=d,i="), strconv.FormatUint(uint64(id), 10)...)
	return append(out, ",q=2,d=I\x1b\\"...)
}
