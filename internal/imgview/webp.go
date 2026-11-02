package imgview

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"io"

	"github.com/AmadeusDE/Misa/internal/imgview/webpcodec"
)

// VP8X can conceal the compressed image's real dimensions from DecodeConfig.
// Inspect the raw codec header separately before permitting any pixel decode.
func validateStaticWebP(b []byte, cfg image.Config) error {
	bad := errors.New("invalid static WebP container or dimensions")
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(b[4:]))+8 != uint64(len(b)) {
		return bad
	}
	found := false
	for p := 12; p < len(b); {
		if len(b)-p < 8 {
			return bad
		}
		n := uint64(binary.LittleEndian.Uint32(b[p+4:]))
		end := uint64(p) + 8 + n + (n & 1)
		if end > uint64(len(b)) || n&1 != 0 && b[p+8+int(n)] != 0 {
			return bad
		}
		switch string(b[p : p+4]) {
		case "VP8 ", "VP8L":
			if found {
				return bad
			}
			found = true
			hdr := make([]byte, 12)
			copy(hdr, "RIFF")
			binary.LittleEndian.PutUint32(hdr[4:], uint32(4+end-uint64(p)))
			copy(hdr[8:], "WEBP")
			raw, err := webpcodec.DecodeConfig(io.MultiReader(bytes.NewReader(hdr), bytes.NewReader(b[p:int(end)])))
			if err != nil || raw.Width != cfg.Width || raw.Height != cfg.Height {
				return bad
			}
		case "ANIM", "ANMF":
			return bad
		}
		p = int(end)
	}
	if !found {
		return bad
	}
	return nil
}
