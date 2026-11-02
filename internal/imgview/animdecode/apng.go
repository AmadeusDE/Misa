package animdecode

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"time"
)

type apngSpan struct{ start, end int }
type apngImage struct {
	rect  image.Rectangle
	spans []apngSpan
}

func indexAPNG(ctx context.Context, b []byte, l Limits) (*index, error) {
	l = normalizeLimits(l)
	bad := func() (*index, error) { return nil, ErrMalformed }
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(b)) > l.MaxEncodedBytes {
		return nil, ErrLimit
	}
	if len(b) < 8 || !bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return bad()
	}
	idx := &index{format: "apng"}
	var ihdr [13]byte
	var palette, trans []byte
	var images []apngImage
	var poster apngImage
	current := -1
	var actl, idat, idatEnd, iend, plte, trns bool
	var ancillary uint16
	var declared uint32
	var seq uint64
	charge := func(n int64) bool {
		if n > l.MaxMetadataBytes-idx.retained {
			return false
		}
		idx.retained += n
		return true
	}
	addSpan := func(a *apngImage, start, end int) bool {
		if !charge(256) {
			return false
		}
		a.spans = append(a.spans, apngSpan{start, end})
		return true
	}
	for off := 8; off < len(b); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(b)-off < 12 {
			return bad()
		}
		n := uint64(binary.BigEndian.Uint32(b[off:]))
		if n > uint64(len(b)-off-12) {
			return bad()
		}
		end := off + 12 + int(n)
		typ := string(b[off+4 : off+8])
		d := b[off+8 : end-4]
		for _, c := range b[off+4 : off+8] {
			if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
				return bad()
			}
		}
		if b[off+6]&32 != 0 {
			return bad()
		}
		if crc32.ChecksumIEEE(b[off+4:end-4]) != binary.BigEndian.Uint32(b[end-4:end]) {
			return bad()
		}
		if off == 8 && typ != "IHDR" {
			return bad()
		}
		if idat && typ != "IDAT" {
			idatEnd = true
		}
		switch typ {
		case "IHDR":
			if off != 8 || len(d) != 13 {
				return bad()
			}
			copy(ihdr[:], d)
			w, h := uint64(binary.BigEndian.Uint32(d)), uint64(binary.BigEndian.Uint32(d[4:]))
			if w == 0 || h == 0 || w > 0x7fffffff || h > 0x7fffffff {
				return bad()
			}
			if w > uint64(l.MaxPixels) || h > uint64(l.MaxPixels) || w*h > uint64(l.MaxPixels) {
				return nil, ErrLimit
			}
			idx.size = image.Pt(int(w), int(h))
			if !validSize(idx.size.X, idx.size.Y, l) {
				return nil, ErrLimit
			}
			depth, ct := d[8], d[9]
			valid := ct == 0 && (depth == 1 || depth == 2 || depth == 4 || depth == 8 || depth == 16) || ct == 2 && (depth == 8 || depth == 16) || ct == 3 && (depth == 1 || depth == 2 || depth == 4 || depth == 8) || (ct == 4 || ct == 6) && (depth == 8 || depth == 16)
			if !valid || d[10] != 0 || d[11] != 0 || d[12] > 1 {
				return bad()
			}
		case "PLTE":
			if plte || idat || trns || len(d) == 0 || len(d)%3 != 0 || len(d) > 768 || ihdr[9] == 0 || ihdr[9] == 4 || ihdr[9] == 3 && len(d)/3 > 1<<ihdr[8] {
				return bad()
			}
			plte = true
			palette = b[off:end]
		case "tRNS":
			if trns || idat || ihdr[9] == 3 && !plte {
				return bad()
			}
			switch ihdr[9] {
			case 0:
				if len(d) != 2 {
					return bad()
				}
			case 2:
				if len(d) != 6 {
					return bad()
				}
			case 3:
				if len(d) == 0 || len(d) > (len(palette)-12)/3 {
					return bad()
				}
			default:
				return bad()
			}
			trns = true
			trans = b[off:end]
		case "cHRM", "gAMA", "iCCP", "sBIT", "sRGB", "bKGD", "hIST", "pHYs", "tIME":
			if !apngAncillary(typ, d, ihdr, palette, plte, idat, &ancillary) {
				return bad()
			}
		case "acTL":
			if actl || idat || len(d) != 8 {
				return bad()
			}
			declared = binary.BigEndian.Uint32(d)
			if declared == 0 {
				return bad()
			}
			if uint64(declared) > uint64(l.MaxFrames) {
				return nil, ErrLimit
			}
			idx.plays = uint64(binary.BigEndian.Uint32(d[4:]))
			actl = true
		case "fcTL":
			if !actl || len(d) != 26 || uint64(binary.BigEndian.Uint32(d)) != seq {
				return bad()
			}
			seq++
			if current >= 0 && len(images[current].spans) == 0 {
				return bad()
			}
			if uint64(len(images)) >= uint64(declared) {
				return bad()
			}
			w, h, x, y := uint64(binary.BigEndian.Uint32(d[4:])), uint64(binary.BigEndian.Uint32(d[8:])), uint64(binary.BigEndian.Uint32(d[12:])), uint64(binary.BigEndian.Uint32(d[16:]))
			if w == 0 || h == 0 || x+w > uint64(idx.size.X) || y+h > uint64(idx.size.Y) || d[24] > 2 || d[25] > 1 {
				return bad()
			}
			if !idat && (x != 0 || y != 0 || w != uint64(idx.size.X) || h != uint64(idx.size.Y)) {
				return bad()
			}
			if !charge(512) {
				return nil, ErrLimit
			}
			rect := image.Rect(int(x), int(y), int(x+w), int(y+h))
			images = append(images, apngImage{rect: rect})
			current = len(images) - 1
			den := uint64(binary.BigEndian.Uint16(d[22:]))
			if den == 0 {
				den = 100
			}
			delay := time.Duration(uint64(binary.BigEndian.Uint16(d[20:])) * uint64(time.Second) / den)
			idx.frames = append(idx.frames, frame{rect: rect, encoded: delay, delay: effectiveDelay(delay, false, l), dispose: disposal(d[24]), source: d[25] == 0})
		case "IDAT":
			if !actl || idatEnd || ihdr[9] == 3 && !plte {
				return bad()
			}
			idat = true
			if current >= 0 {
				if current != 0 {
					return bad()
				}
				if !addSpan(&images[current], off+8, end-4) {
					return nil, ErrLimit
				}
			} else {
				if !addSpan(&poster, off+8, end-4) {
					return nil, ErrLimit
				}
			}
		case "fdAT":
			if !idat || current < 0 || len(d) < 4 || uint64(binary.BigEndian.Uint32(d)) != seq || len(poster.spans) == 0 && current == 0 {
				return bad()
			}
			seq++
			if !addSpan(&images[current], off+12, end-4) {
				return nil, ErrLimit
			}
		case "IEND":
			if len(d) != 0 || !idat || end != len(b) || current < 0 || len(images[current].spans) == 0 || uint32(len(images)) != declared {
				return bad()
			}
			iend = true
		default:
			if b[off+4]&32 == 0 {
				return bad()
			}
		}
		off = end
		if iend {
			break
		}
	}
	if !iend {
		return bad()
	}
	for i := range images {
		im := images[i]
		idx.frames[i].decode = apngDecoder(b, ihdr, palette, trans, im)
	}
	if len(poster.spans) > 0 {
		poster.rect = image.Rectangle{Max: idx.size}
		idx.poster = apngDecoder(b, ihdr, palette, trans, poster)
	}
	if err := checkIndex(idx, l); err != nil {
		return nil, err
	}
	return idx, nil
}
