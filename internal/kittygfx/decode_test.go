package kittygfx

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"math"
	"strconv"
	"testing"
)

func encodePixels(raw []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(raw))
}

func TestDecodeRGB(t *testing.T) {
	raw := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 255}
	cmd := Command{Action: ActionTransmit, Transmission: 'd', Format: 24, Width: 2, Height: 2}
	raster, err := Decode(cmd, encodePixels(raw), DefaultLimits())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := raster.Bounds(); got != (image.Rect(0, 0, 2, 2)) {
		t.Fatalf("Bounds = %v", got)
	}
	if raster.Format() != 24 {
		t.Fatalf("Format = %d", raster.Format())
	}
	want := []color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}
	for i, w := range want {
		x, y := i%2, i/2
		got, ok := raster.At(x, y).(color.NRGBA)
		if !ok || got != w {
			t.Fatalf("At(%d,%d) = %v, want %v", x, y, raster.At(x, y), w)
		}
	}
	dst := make([]byte, len(raw))
	if err := raster.CopyPixels(dst); err != nil {
		t.Fatalf("CopyPixels: %v", err)
	}
	if !bytes.Equal(dst, raw) {
		t.Fatalf("CopyPixels = %v, want %v", dst, raw)
	}
}

func TestDecodeRGBA(t *testing.T) {
	raw := []byte{10, 20, 30, 40}
	cmd := Command{Action: ActionTransmitPlace, Transmission: 'd', Format: 32, Width: 1, Height: 1}
	raster, err := Decode(cmd, encodePixels(raw), DefaultLimits())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if raster.Format() != 32 || raster.Bounds() != (image.Rect(0, 0, 1, 1)) {
		t.Fatalf("raster = %+v", raster)
	}
	if got, ok := raster.At(0, 0).(color.NRGBA); !ok || got != (color.NRGBA{10, 20, 30, 40}) {
		t.Fatalf("At = %v", raster.At(0, 0))
	}
	if raster.ColorModel() != color.NRGBAModel {
		t.Fatalf("ColorModel = %v", raster.ColorModel())
	}
}

func TestRasterIsImmutable(t *testing.T) {
	raw := []byte{1, 2, 3}
	raster, err := Decode(Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1}, encodePixels(raw), DefaultLimits())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	dst := make([]byte, 3)
	if err := raster.CopyPixels(dst); err != nil {
		t.Fatalf("CopyPixels: %v", err)
	}
	dst[0] = 99
	if got, ok := raster.At(0, 0).(color.NRGBA); !ok || got.R != 1 {
		t.Fatalf("CopyPixels exposed the raster: %v", raster.At(0, 0))
	}
	if err := raster.CopyPixels(make([]byte, 2)); err == nil {
		t.Fatal("CopyPixels accepted a short destination")
	}
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {1, 0}, {0, 1}, {100, 100}} {
		if got := raster.At(p[0], p[1]); got != (color.NRGBA{}) {
			t.Fatalf("At(%d,%d) outside Bounds = %v", p[0], p[1], got)
		}
	}
}

func TestDecodeExactPayloadLength(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		encoded string
		want    error
	}{
		{"decodes one byte too many", Command{Action: ActionTransmit, Format: 32, Width: 1, Height: 1}, "AAAAAAA=", ErrTooLarge},
		{"decodes one byte too few", Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1}, "AA==", ErrMalformed},
		{"incomplete quantum", Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1}, "AAAAA", ErrMalformed},
		{"full quantum for three bytes", Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1}, "AAAA", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.cmd, []byte(tt.encoded), Limits{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("Decode(%q) = %v, want %v", tt.encoded, err, tt.want)
			}
		})
	}
}

func TestDecodeRejectsImageLargerThanInt(t *testing.T) {
	if strconv.IntSize > 32 {
		t.Skip("a 64-bit int can address every configured image size")
	}
	cmd := Command{Action: ActionTransmit, Format: 32, Width: 1 << 16, Height: 1 << 16}
	lim := Limits{MaxWidth: 1 << 16, MaxHeight: 1 << 16, MaxImageBytes: math.MaxInt64, MaxTerminalBytes: math.MaxInt64}
	if _, err := Decode(cmd, []byte("AAAA"), lim); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Decode = %v, want ErrTooLarge", err)
	}
}

func TestDecodeDefaultCaps(t *testing.T) {
	full := Command{Action: ActionTransmit, Format: 32, Width: 16384, Height: 16384}
	if _, err := Decode(full, []byte("AAAA"), Limits{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("16384x16384 RGBA = %v, want ErrTooLarge from the byte cap", err)
	}
	noByteCap := Limits{MaxImageBytes: math.MaxInt64, MaxTerminalBytes: math.MaxInt64}
	if _, err := Decode(full, []byte("AAAA"), noByteCap); !errors.Is(err, ErrMalformed) {
		t.Fatalf("without a byte cap the short payload should be malformed, got %v", err)
	}
	for _, cmd := range []Command{
		{Action: ActionTransmit, Format: 32, Width: 16385, Height: 1},
		{Action: ActionTransmit, Format: 32, Width: 1, Height: 16385},
	} {
		if _, err := Decode(cmd, []byte("AAAA"), Limits{}); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("%dx%d = %v, want ErrTooLarge from the dimension cap", cmd.Width, cmd.Height, err)
		}
	}
}

func TestDecodeRejects(t *testing.T) {
	one := encodePixels([]byte{1, 2, 3})
	tests := []struct {
		name    string
		cmd     Command
		encoded []byte
		lim     Limits
		want    error
	}{
		{"place action", Command{Action: ActionPlace, Format: 24, Width: 1, Height: 1}, one, Limits{}, ErrUnsupported},
		{"delete action", Command{Action: ActionDelete, Format: 24, Width: 1, Height: 1}, one, Limits{}, ErrUnsupported},
		{"query action", Command{Action: ActionQuery, Format: 24, Width: 1, Height: 1}, one, Limits{}, ErrUnsupported},
		{"file transmission", Command{Action: ActionTransmit, Transmission: 'f', Format: 24, Width: 1, Height: 1}, one, Limits{}, ErrUnsupported},
		{"format", Command{Action: ActionTransmit, Format: 16, Width: 1, Height: 1}, one, Limits{}, ErrUnsupported},
		{"png format", Command{Action: ActionTransmit, Format: 100, Width: 1, Height: 1}, one, Limits{}, ErrUnsupported},
		{"missing format", Command{Action: ActionTransmit, Width: 1, Height: 1}, one, Limits{}, ErrMalformed},
		{"zero width", Command{Action: ActionTransmit, Format: 24, Height: 1}, one, Limits{}, ErrMalformed},
		{"zero height", Command{Action: ActionTransmit, Format: 24, Width: 1}, one, Limits{}, ErrMalformed},
		{"negative width", Command{Action: ActionTransmit, Format: 24, Width: -1, Height: 1}, one, Limits{}, ErrMalformed},
		{"width limit", Command{Action: ActionTransmit, Format: 24, Width: 16385, Height: 1}, one, Limits{}, ErrTooLarge},
		{"height limit", Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 16385}, one, Limits{}, ErrTooLarge},
		{"image bytes", Command{Action: ActionTransmit, Format: 24, Width: 2, Height: 2}, encodePixels(make([]byte, 12)), Limits{MaxImageBytes: 8}, ErrTooLarge},
		{"terminal bytes", Command{Action: ActionTransmit, Format: 24, Width: 2, Height: 2}, encodePixels(make([]byte, 12)), Limits{MaxTerminalBytes: 8}, ErrTooLarge},
		{"overflow", Command{Action: ActionTransmit, Format: 32, Width: math.MaxInt32, Height: math.MaxInt32}, one,
			Limits{MaxWidth: math.MaxInt32, MaxHeight: math.MaxInt32, MaxImageBytes: math.MaxInt64, MaxTerminalBytes: math.MaxInt64}, ErrTooLarge},
		{"short payload", Command{Action: ActionTransmit, Format: 24, Width: 2, Height: 2}, one, Limits{}, ErrMalformed},
		{"long payload", Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1}, encodePixels([]byte{1, 2, 3, 4, 5, 6}), Limits{}, ErrTooLarge},
		{"bad base64", Command{Action: ActionTransmit, Format: 24, Width: 1, Height: 1}, []byte("AA*A"), Limits{}, ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.cmd, tt.encoded, tt.lim); !errors.Is(err, tt.want) {
				t.Fatalf("Decode(%+v, %q) = %v, want %v", tt.cmd, tt.encoded, err, tt.want)
			}
		})
	}
}
