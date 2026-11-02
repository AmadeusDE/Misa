package animdecode

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
)

func apngDecoder(b []byte, ihdr [13]byte, palette, trans []byte, im apngImage) func(context.Context) (image.Image, error) {
	return func(ctx context.Context) (image.Image, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h := ihdr
		binary.BigEndian.PutUint32(h[:], uint32(im.rect.Dx()))
		binary.BigEndian.PutUint32(h[4:], uint32(im.rect.Dy()))
		stream := &apngReader{ctx: ctx, data: b, ihdr: apngChunk("IHDR", h[:]), palette: palette, trans: trans, spans: im.spans}
		decoded, err := png.Decode(stream)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return &apngOffset{Image: decoded, rect: im.rect}, nil
	}
}
func apngChunk(typ string, data []byte) []byte {
	b := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(b, uint32(len(data)))
	copy(b[4:], typ)
	copy(b[8:], data)
	binary.BigEndian.PutUint32(b[len(b)-4:], crc32.ChecksumIEEE(b[4:len(b)-4]))
	return b
}

type apngOffset struct {
	image.Image
	rect image.Rectangle
}

func (im *apngOffset) Bounds() image.Rectangle { return im.rect }
func (im *apngOffset) At(x, y int) color.Color { return im.Image.At(x-im.rect.Min.X, y-im.rect.Min.Y) }

// apngReader presents the synthesized container a piece at a time, without
// allocating a reader or reconstructed IDAT chunk for every input span.
type apngReader struct {
	ctx                        context.Context
	data, ihdr, palette, trans []byte
	spans                      []apngSpan
	stage, span                int
	pending                    []byte
	header                     [8]byte
	tail                       [4]byte
}

func (r *apngReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	for len(r.pending) == 0 {
		switch r.stage {
		case 0:
			r.pending = []byte("\x89PNG\r\n\x1a\n")
			r.stage++
		case 1:
			r.pending = r.ihdr
			r.stage++
		case 2:
			r.pending = r.palette
			r.stage++
		case 3:
			r.pending = r.trans
			r.stage++
		case 4:
			if r.span == len(r.spans) {
				r.stage = 7
				continue
			}
			s := r.spans[r.span]
			data := r.data[s.start:s.end]
			binary.BigEndian.PutUint32(r.header[:], uint32(len(data)))
			copy(r.header[4:], "IDAT")
			crc := crc32.Update(0, crc32.IEEETable, r.header[4:])
			crc = crc32.Update(crc, crc32.IEEETable, data)
			binary.BigEndian.PutUint32(r.tail[:], crc)
			r.pending = r.header[:]
			r.stage++
		case 5:
			s := r.spans[r.span]
			r.pending = r.data[s.start:s.end]
			r.stage++
		case 6:
			r.pending = r.tail[:]
			r.span++
			r.stage = 4
		case 7:
			r.pending = []byte("\x00\x00\x00\x00IEND\xaeB`\x82")
			r.stage++
		default:
			return 0, io.EOF
		}
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
