package imgview

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
)

// isAnimated detects required animation containers before the static decoder.
// Malformed container boundaries are rejected instead of falling back to a poster.
func isAnimated(b []byte) (bool, error) {
	if len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")) {
		for p := 8; p < len(b); {
			if len(b)-p < 12 {
				return false, errors.New("truncated PNG chunk")
			}
			n := uint64(binary.BigEndian.Uint32(b[p:]))
			if n > uint64(len(b)-p-12) {
				return false, errors.New("truncated PNG chunk")
			}
			typ := string(b[p+4 : p+8])
			if typ == "acTL" {
				return true, nil
			}
			if typ == "IEND" {
				return false, nil
			}
			p += int(n) + 12
		}
		return false, nil
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		if len(b) >= 21 && string(b[12:16]) == "VP8X" && b[20]&2 != 0 {
			return true, nil
		}
		return false, nil
	}
	if len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a") {
		return animatedGIF(b)
	}
	return false, nil
}
func animatedGIF(b []byte) (bool, error) {
	if len(b) < 13 {
		return false, errors.New("truncated GIF")
	}
	p := 13
	if b[10]&0x80 != 0 {
		p += 3 * (1 << ((b[10] & 7) + 1))
	}
	frames := 0
	skip := func() bool {
		for {
			if p >= len(b) {
				return false
			}
			n := int(b[p])
			p++
			if n == 0 {
				return true
			}
			if n > len(b)-p {
				return false
			}
			p += n
		}
	}
	for p < len(b) {
		c := b[p]
		p++
		switch c {
		case 0x3b:
			return false, nil
		case 0x21:
			if p >= len(b) {
				return false, errors.New("truncated GIF extension")
			}
			p++
			if !skip() {
				return false, errors.New("truncated GIF extension")
			}
		case 0x2c:
			frames++
			if frames > 1 {
				return true, nil
			}
			if len(b)-p < 9 {
				return false, errors.New("truncated GIF frame")
			}
			flags := b[p+8]
			p += 9
			if flags&0x80 != 0 {
				p += 3 * (1 << ((flags & 7) + 1))
			}
			if p >= len(b) {
				return false, errors.New("truncated GIF palette")
			}
			p++
			if !skip() {
				return false, errors.New("truncated GIF data")
			}
		default:
			return false, errors.New("invalid GIF block")
		}
	}
	return false, errors.New("truncated GIF")
}

type webpFrame struct {
	rect      image.Rectangle
	raw, data []byte
	noBlend   bool
}

// webpPoster validates the RIFF boundaries and wraps the first frame's image
// chunks as a standalone WebP, retaining ALPH when present.
func webpPoster(b []byte) (image.Rectangle, webpFrame, color.NRGBA, error) {
	var f webpFrame
	var bg color.NRGBA
	bad := func() (image.Rectangle, webpFrame, color.NRGBA, error) {
		return image.Rectangle{}, f, bg, errors.New("invalid animated WebP")
	}
	if len(b) < 12 || uint64(binary.LittleEndian.Uint32(b[4:]))+8 != uint64(len(b)) {
		return bad()
	}
	var canvas image.Rectangle
	haveAnim := false
	for p := 12; p < len(b); {
		if len(b)-p < 8 {
			return bad()
		}
		n := uint64(binary.LittleEndian.Uint32(b[p+4:]))
		end := uint64(p) + 8 + n + (n & 1)
		if end > uint64(len(b)) {
			return bad()
		}
		typ := string(b[p : p+4])
		chunk := b[p+8 : p+8+int(n)]
		switch typ {
		case "VP8X":
			if len(chunk) != 10 || chunk[0]&2 == 0 || !canvas.Empty() {
				return bad()
			}
			canvas = image.Rect(0, 0, int(le24(chunk[4:]))+1, int(le24(chunk[7:]))+1)
		case "ANIM":
			if len(chunk) != 6 || canvas.Empty() || haveAnim {
				return bad()
			}
			haveAnim = true
			bg = color.NRGBA{R: chunk[2], G: chunk[1], B: chunk[0], A: chunk[3]}
		case "ANMF":
			if !haveAnim || len(chunk) < 16 {
				return bad()
			}
			x, y := int(le24(chunk))*2, int(le24(chunk[3:]))*2
			w, h := int(le24(chunk[6:]))+1, int(le24(chunk[9:]))+1
			f.rect = image.Rect(x, y, x+w, y+h)
			f.noBlend = chunk[15]&2 != 0
			if !f.rect.In(canvas) || chunk[15]&0xfc != 0 {
				return bad()
			}
			payload := chunk[16:]
			var imageChunk []byte
			alpha := false
			for q := 0; q < len(payload); {
				if len(payload)-q < 8 {
					return bad()
				}
				size := uint64(binary.LittleEndian.Uint32(payload[q+4:]))
				e := uint64(q) + 8 + size + (size & 1)
				if e > uint64(len(payload)) {
					return bad()
				}
				switch string(payload[q : q+4]) {
				case "ALPH":
					if alpha || imageChunk != nil {
						return bad()
					}
					alpha = true
				case "VP8 ", "VP8L":
					if imageChunk != nil || (alpha && string(payload[q:q+4]) == "VP8L") {
						return bad()
					}
					imageChunk = payload[q:int(e)]
				default:
					return bad()
				}
				q = int(e)
			}
			if imageChunk == nil {
				return bad()
			}
			wrap := func(chunks []byte) []byte {
				out := make([]byte, 12, 12+len(chunks))
				copy(out, "RIFF")
				binary.LittleEndian.PutUint32(out[4:], uint32(4+len(chunks)))
				copy(out[8:], "WEBP")
				return append(out, chunks...)
			}
			f.raw = wrap(imageChunk)
			if alpha {
				hdr := make([]byte, 18)
				copy(hdr, "VP8X")
				binary.LittleEndian.PutUint32(hdr[4:], 10)
				hdr[8] = 0x10
				put24(hdr[12:], uint32(w-1))
				put24(hdr[15:], uint32(h-1))
				f.data = wrap(append(hdr, payload...))
			} else {
				f.data = wrap(payload)
			}
			return canvas, f, bg, nil
		}
		p = int(end)
	}
	return bad()
}
func le24(b []byte) uint32     { return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 }
func put24(b []byte, n uint32) { b[0] = byte(n); b[1] = byte(n >> 8); b[2] = byte(n >> 16) }
