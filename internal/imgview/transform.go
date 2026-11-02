package imgview

import (
	"encoding/binary"
	"image"
)

// Orientation uses EXIF's eight coordinate mappings. Invalid values mean identity.
type Orientation int

const (
	Normal         Orientation = 1
	FlipHorizontal Orientation = 2
	Rotate180      Orientation = 3
	FlipVertical   Orientation = 4
	Transpose      Orientation = 5
	Rotate90CW     Orientation = 6
	Transverse     Orientation = 7
	Rotate90CCW    Orientation = 8
)

// Transform returns a fresh, zero-origin raster; it never mutates src.
func Transform(src *image.NRGBA, o Orientation) *image.NRGBA {
	if o < 1 || o > 8 {
		o = Normal
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch o {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.SetNRGBA(dx, dy, src.NRGBAAt(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// exifOrientation scans only JPEG APP1 Exif data and a bounded TIFF IFD.
func exifOrientation(b []byte) Orientation {
	if len(b) < 4 || b[0] != 0xff || b[1] != 0xd8 {
		return Normal
	}
	for p := 2; p+4 <= len(b); {
		if b[p] != 0xff {
			return Normal
		}
		for p < len(b) && b[p] == 0xff {
			p++
		}
		if p >= len(b) {
			break
		}
		marker := b[p]
		p++
		if marker == 0xda || marker == 0xd9 {
			break
		}
		if marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if p+2 > len(b) {
			break
		}
		n := int(binary.BigEndian.Uint16(b[p:]))
		if n < 2 || n > len(b)-p {
			break
		}
		if marker == 0xe1 && n >= 8 && string(b[p+2:p+8]) == "Exif\x00\x00" {
			return tiffOrientation(b[p+8 : p+n])
		}
		p += n
	}
	return Normal
}
func tiffOrientation(b []byte) Orientation {
	if len(b) < 8 {
		return Normal
	}
	var order binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return Normal
	}
	if order.Uint16(b[2:]) != 42 {
		return Normal
	}
	off := uint64(order.Uint32(b[4:]))
	if off > uint64(len(b)-2) {
		return Normal
	}
	p := int(off)
	n := int(order.Uint16(b[p:]))
	p += 2
	if n > (len(b)-p)/12 {
		return Normal
	}
	for i := 0; i < n; i++ {
		e := b[p+i*12:]
		if order.Uint16(e) == 0x112 && order.Uint16(e[2:]) == 3 && order.Uint32(e[4:]) == 1 {
			v := Orientation(order.Uint16(e[8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return Normal
		}
	}
	return Normal
}
