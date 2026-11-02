package animdecode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"time"

	"github.com/AmadeusDE/Misa/internal/imgview/webpcodec"
	"golang.org/x/image/webp"
)

func webp24(b []byte) int       { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
func webpPut24(b []byte, n int) { b[0] = byte(n); b[1] = byte(n >> 8); b[2] = byte(n >> 16) }

// webpChunk checks both the declared extent and RIFF's mandatory zero padding.
func webpChunk(b []byte, p int) (string, []byte, int, error) {
	if p < 0 || len(b)-p < 8 {
		return "", nil, 0, ErrMalformed
	}
	n := uint64(binary.LittleEndian.Uint32(b[p+4:]))
	end := uint64(p) + 8 + n
	padded := end + (n & 1)
	if padded > uint64(len(b)) || (n&1 != 0 && b[int(end)] != 0) {
		return "", nil, 0, ErrMalformed
	}
	return string(b[p : p+4]), b[p+8 : int(end)], int(padded), nil
}

func webpReader(payload []byte, w, h int, alpha bool) io.Reader {
	hdr := make([]byte, 12)
	copy(hdr, "RIFF")
	copy(hdr[8:], "WEBP")
	extra := 0
	if alpha {
		extra = 18
	}
	binary.LittleEndian.PutUint32(hdr[4:], uint32(4+extra+len(payload)))
	if !alpha {
		return io.MultiReader(bytes.NewReader(hdr), bytes.NewReader(payload))
	}
	x := make([]byte, 18)
	copy(x, "VP8X")
	binary.LittleEndian.PutUint32(x[4:], 10)
	x[8] = 0x10
	webpPut24(x[12:], w-1)
	webpPut24(x[15:], h-1)
	return io.MultiReader(bytes.NewReader(hdr), bytes.NewReader(x), bytes.NewReader(payload))
}

func webpFrameData(ctx context.Context, payload []byte, w, h int, budget func() int64) (func(context.Context) (image.Image, error), error) {
	if uint64(len(payload)) > uint64(^uint32(0))-22 {
		return nil, ErrLimit
	}
	alpha, found := false, false
	var codec []byte
	for p := 0; p < len(payload); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		typ, _, end, err := webpChunk(payload, p)
		if err != nil {
			return nil, err
		}
		switch typ {
		case "ALPH":
			if alpha || found {
				return nil, ErrMalformed
			}
			alpha = true
		case "VP8 ", "VP8L":
			if found || (alpha && typ == "VP8L") {
				return nil, ErrMalformed
			}
			found = true
			codec = payload[p:end]
		default:
			if !found {
				return nil, ErrMalformed
			}
		}
		p = end
	}
	if !found {
		return nil, ErrMalformed
	}
	// Parse the codec itself without VP8X; its canvas fields can conceal a mismatch.
	cfg, err := webp.DecodeConfig(webpReader(codec, w, h, false))
	if err != nil || cfg.Width != w || cfg.Height != h {
		return nil, ErrMalformed
	}
	// Unknown trailing subchunks are not needed by the decoder.
	end := len(payload)
	for p := 0; p < len(payload); {
		typ, _, next, _ := webpChunk(payload, p)
		if typ == "VP8 " || typ == "VP8L" {
			end = next
			break
		}
		p = next
	}
	payload = payload[:end]
	return func(ctx context.Context) (image.Image, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		im, err := webpcodec.Decode(webpReader(payload, w, h, alpha), budget())
		if errors.Is(err, webpcodec.ErrLimit) {
			return nil, ErrLimit
		}
		if err != nil {
			return nil, fmt.Errorf("%w: WebP frame: %v", ErrMalformed, err)
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if im.Bounds().Dx() != w || im.Bounds().Dy() != h {
			return nil, ErrMalformed
		}
		return im, nil
	}, nil
}

func indexWebP(ctx context.Context, b []byte, l Limits) (*index, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(b)) > l.MaxEncodedBytes {
		return nil, ErrLimit
	}
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(b[4:]))+8 != uint64(len(b)) {
		return nil, ErrMalformed
	}
	idx := &index{format: "webp"}
	anim := false
	seenFrame := false
	ended := false
	flags := byte(0)
	seenICC, seenEXIF, seenXMP := false, false, false
	for p := 12; p < len(b); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		typ, c, end, err := webpChunk(b, p)
		if err != nil {
			return nil, err
		}
		if p == 12 && typ != "VP8X" {
			return nil, ErrMalformed
		}
		switch typ {
		case "VP8X":
			if p != 12 || len(c) != 10 || c[0]&0xc1 != 0 || c[0]&2 == 0 || c[1] != 0 || c[2] != 0 || c[3] != 0 {
				return nil, ErrMalformed
			}
			flags = c[0]
			idx.size = image.Pt(webp24(c[4:])+1, webp24(c[7:])+1)
			if !validSize(idx.size.X, idx.size.Y, l) {
				return nil, ErrLimit
			}
		case "ICCP":
			if seenICC || anim || flags&0x20 == 0 {
				return nil, ErrMalformed
			}
			seenICC = true
		case "ANIM":
			if anim || len(c) != 6 || ended {
				return nil, ErrMalformed
			}
			anim = true
			idx.background = color.NRGBA{R: c[2], G: c[1], B: c[0], A: c[3]}
			idx.plays = uint64(binary.LittleEndian.Uint16(c[4:]))
		case "ANMF":
			if !anim || ended || len(c) < 16 || c[15]&0xfc != 0 {
				return nil, ErrMalformed
			}
			seenFrame = true
			if len(idx.frames) >= l.MaxFrames || idx.retained > l.MaxMetadataBytes-512 {
				return nil, ErrLimit
			}
			x, y := 2*webp24(c), 2*webp24(c[3:])
			w, h := webp24(c[6:])+1, webp24(c[9:])+1
			r := image.Rect(x, y, x+w, y+h)
			if !r.In(image.Rectangle{Max: idx.size}) {
				return nil, ErrMalformed
			}
			// VP8 may copy its compressed partitions; only one frame is active.
			if n := int64(len(c) - 16); n > idx.encodedWork {
				idx.encodedWork = n
			}
			dec, err := webpFrameData(ctx, c[16:], w, h, func() int64 { return codecWorkingBudget(idx, l) })
			if err != nil {
				return nil, err
			}
			d := time.Duration(webp24(c[12:])) * time.Millisecond
			f := frame{rect: r, encoded: d, delay: effectiveDelay(d, false, l), source: c[15]&2 != 0, decode: dec}
			if c[15]&1 != 0 {
				f.dispose = background
			}
			idx.frames = append(idx.frames, f)
			idx.retained += 512
		case "EXIF":
			if !seenFrame || seenEXIF || flags&8 == 0 || seenXMP {
				return nil, ErrMalformed
			}
			seenEXIF = true
			ended = true
		case "XMP ":
			if !seenFrame || seenXMP || flags&4 == 0 {
				return nil, ErrMalformed
			}
			seenXMP = true
			ended = true
		case "VP8 ", "VP8L", "ALPH":
			return nil, ErrMalformed
		}
		p = end
	}
	if !anim || !seenFrame || (flags&0x20 != 0) != seenICC || (flags&8 != 0) != seenEXIF || (flags&4 != 0) != seenXMP {
		return nil, ErrMalformed
	}
	if err := checkIndex(idx, l); err != nil {
		return nil, err
	}
	return idx, nil
}
