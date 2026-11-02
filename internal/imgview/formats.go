package imgview

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"strings"
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

// imageExtensions are the extensions the registered decoders can read.
var imageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff"}

func isImageName(name string) bool {
	for _, ext := range imageExtensions {
		if strings.EqualFold(filepath.Ext(name), ext) {
			return true
		}
	}
	return false
}
