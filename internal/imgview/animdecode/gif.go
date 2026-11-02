package animdecode

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"time"
)

func indexGIF(ctx context.Context, b []byte, l Limits) (*index, error) {
	bad := func() (*index, error) { return nil, ErrMalformed }
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(b)) > l.MaxEncodedBytes {
		return nil, ErrLimit
	}
	if len(b) < 13 || (string(b[:6]) != "GIF87a" && string(b[:6]) != "GIF89a") {
		return bad()
	}
	u16 := func(p []byte) int { return int(binary.LittleEndian.Uint16(p)) }
	w, h := u16(b[6:]), u16(b[8:])
	if !validSize(w, h, l) {
		return nil, ErrLimit
	}
	packed := b[10]
	pos := 13
	if packed&0x80 != 0 {
		n := 3 * (1 << ((packed & 7) + 1))
		if len(b)-pos < n || int(b[11]) >= n/3 {
			return bad()
		}
		pos += n
	}
	header := b[:pos]
	idx := &index{format: "gif", size: image.Pt(w, h), plays: 1}
	if packed&0x80 != 0 && int(b[11]) < (len(header)-13)/3 {
		p := 13 + int(b[11])*3
		idx.background = color.NRGBA{b[p], b[p+1], b[p+2], 255}
	}
	var gce []byte
	loopSeen := false
	eagerSafe := true
	var eagerBytes int64 = 64 << 10
	var largestFrame int64
	// Consume a chain of data sub-blocks, returning its first terminator position.
	blocks := func(start int) (int, bool) {
		p := start
		for {
			if p >= len(b) {
				return 0, false
			}
			n := int(b[p])
			p++
			if n == 0 {
				return p, true
			}
			if n > len(b)-p {
				return 0, false
			}
			p += n
		}
	}
	for pos < len(b) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch b[pos] {
		case 0x3b:
			if pos != len(b)-1 || len(idx.frames) == 0 || gce != nil {
				return bad()
			}
			if err := checkIndex(idx, l); err != nil {
				return nil, err
			}
			eagerBytes += largestFrame // Interlace decoding may hold a second pixel buffer.
			candidate := *idx
			candidate.eagerBytes = eagerBytes
			if eagerSafe && eagerBytes <= l.MaxWorkingBytes/4 && checkIndex(&candidate, l) == nil {
				all, err := gif.DecodeAll(&gifContextReader{ctx: ctx, r: bytes.NewReader(b)})
				if err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					return nil, fmt.Errorf("%w: GIF frames: %v", ErrMalformed, err)
				}
				if len(all.Image) != len(idx.frames) {
					return bad()
				}
				for i, im := range all.Image {
					im := im
					idx.frames[i].decode = func(ctx context.Context) (image.Image, error) {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						return im, nil
					}
				}
				idx.eagerBytes = eagerBytes
			}
			return idx, nil
		case 0x21:
			if len(b)-pos < 2 {
				return bad()
			}
			label := b[pos+1]
			pos += 2
			switch label {
			case 0xf9:
				if gce != nil || len(b)-pos < 6 || b[pos] != 4 || b[pos+5] != 0 || b[pos+1]&0xe0 != 0 || (b[pos+1]>>2)&7 > 3 {
					return bad()
				}
				gce = b[pos-2 : pos+6]
				pos += 6
			case 0xff:
				if pos >= len(b) || b[pos] != 11 || len(b)-pos < 12 {
					return bad()
				}
				app := b[pos+1 : pos+12]
				start := pos + 12
				end, ok := blocks(start)
				if !ok {
					return bad()
				}
				if string(app) == "NETSCAPE2.0" || string(app) == "ANIMEXTS1.0" {
					if loopSeen || end-start != 5 || b[start] != 3 || b[start+1] != 1 {
						return bad()
					}
					loopSeen = true
					n := u16(b[start+2:])
					if n == 0 {
						idx.plays = 0
					} else {
						idx.plays = uint64(n) + 1
					}
				}
				pos = end
			case 0x01:
				eagerSafe = false
				if pos >= len(b) || b[pos] != 12 || len(b)-pos < 13 {
					return bad()
				}
				end, ok := blocks(pos + 13)
				if !ok {
					return bad()
				}
				pos = end
				gce = nil
			default:
				if label != 0xfe {
					eagerSafe = false
				}
				end, ok := blocks(pos)
				if !ok {
					return bad()
				}
				pos = end
			}
		case 0x2c:
			start := pos
			if len(b)-pos < 10 {
				return bad()
			}
			x, y := u16(b[pos+1:]), u16(b[pos+3:])
			fw, fh := u16(b[pos+5:]), u16(b[pos+7:])
			flags := b[pos+9]
			pos += 10
			if fw == 0 || fh == 0 || x+fw > w || y+fh > h || flags&0x18 != 0 {
				return bad()
			}
			paletteSize := 0
			if flags&0x80 != 0 {
				paletteSize = 1 << ((flags & 7) + 1)
				n := 3 * paletteSize
				if n > len(b)-pos {
					return bad()
				}
				pos += n
			} else if packed&0x80 != 0 {
				paletteSize = 1 << ((packed & 7) + 1)
			} else {
				return bad()
			}
			if gce != nil && gce[3]&1 != 0 && int(gce[6]) >= paletteSize {
				return bad()
			}
			if pos >= len(b) || b[pos] < 2 || b[pos] > 8 {
				return bad()
			}
			pos++
			end, ok := blocks(pos)
			if !ok {
				return bad()
			}
			pos = end
			if len(idx.frames) >= l.MaxFrames || idx.retained > l.MaxMetadataBytes-512 {
				return nil, ErrLimit
			}
			var encoded time.Duration
			disp := keep
			if gce != nil {
				encoded = time.Duration(u16(gce[4:])) * 10 * time.Millisecond
				switch (gce[3] >> 2) & 7 {
				case 2:
					disp = background
				case 3:
					disp = previous
				}
				if len(idx.frames) == 0 && gce[3]&1 != 0 && gce[6] == b[11] {
					idx.background = color.NRGBA{}
				}
			}
			gc := gce
			data := b[start:end]
			rect := image.Rect(x, y, x+fw, y+fh)
			idx.frames = append(idx.frames, frame{rect: rect, encoded: encoded, delay: effectiveDelay(encoded, true, l), dispose: disp, decode: func(ctx context.Context) (image.Image, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				r := io.MultiReader(bytes.NewReader(header), bytes.NewReader(gc), bytes.NewReader(data), bytes.NewReader([]byte{0x3b}))
				im, err := gif.Decode(r)
				if err != nil {
					return nil, fmt.Errorf("%w: GIF frame: %v", ErrMalformed, err)
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return im, nil
			}})
			idx.retained += 512
			pixels := int64(fw) * int64(fh)
			if pixels > largestFrame {
				largestFrame = pixels
			}
			eagerBytes += pixels + 16<<10
			gce = nil
		default:
			return bad()
		}
	}
	return bad()
}

// gifContextReader lets the eager decoder observe cancellation while reading.
type gifContextReader struct {
	ctx context.Context
	r   *bytes.Reader
}

func (r *gifContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
func (r *gifContextReader) ReadByte() (byte, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.ReadByte()
}
