// Package imgview provides bounded, read-only image decoding and transforms.
package imgview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"runtime"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var ErrAnimationPending = errors.New("animated image support is pending")
var ErrLimit = errors.New("image exceeds configured limit")

// Limits bounds encoded input and decoded dimensions. Zero fields use defaults.
type Limits struct {
	MaxEncodedBytes int64
	MaxPixels       int64
}

func DefaultLimits() Limits { return Limits{64 << 20, 32_000_000} }
func (l Limits) normalized() Limits {
	d := DefaultLimits()
	if l.MaxEncodedBytes <= 0 {
		l.MaxEncodedBytes = d.MaxEncodedBytes
	}
	if l.MaxPixels <= 0 {
		l.MaxPixels = d.MaxPixels
	}
	return l
}

// Decoded is immutable after publication. Orientation has already been applied.
type Decoded struct {
	Pixels   *image.NRGBA
	Format   string
	Animated bool
}

// Decode reads a regular file with finite input and pixel limits. Cancellation
// cannot interrupt a codec invocation, but suppresses publication afterwards.
func Decode(path string, limits Limits) (Decoded, error) {
	return DecodeContext(context.Background(), path, limits)
}

func DecodeContext(ctx context.Context, path string, limits Limits) (Decoded, error) {
	l := limits.normalized()
	if err := ctx.Err(); err != nil {
		return Decoded{}, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return Decoded{}, err
	}
	if !st.Mode().IsRegular() {
		return Decoded{}, errors.New("not a regular file")
	}
	if st.Size() > l.MaxEncodedBytes {
		return Decoded{}, ErrLimit
	}
	f, err := os.Open(path)
	if err != nil {
		return Decoded{}, err
	}
	defer f.Close()
	fst, err := f.Stat()
	if err != nil {
		return Decoded{}, err
	}
	if !fst.Mode().IsRegular() || !os.SameFile(st, fst) {
		return Decoded{}, errors.New("file changed while opening")
	}
	// Avoid overflowing LimitReader's extra byte.
	if l.MaxEncodedBytes == int64(^uint64(0)>>1) {
		return Decoded{}, ErrLimit
	}
	b, err := io.ReadAll(io.LimitReader(f, l.MaxEncodedBytes+1))
	if err != nil {
		return Decoded{}, err
	}
	if int64(len(b)) > l.MaxEncodedBytes {
		return Decoded{}, ErrLimit
	}
	end, err := f.Stat()
	if err != nil {
		return Decoded{}, err
	}
	if fst.Size() != end.Size() || !fst.ModTime().Equal(end.ModTime()) {
		return Decoded{}, errors.New("file changed while reading")
	}
	if err = ctx.Err(); err != nil {
		return Decoded{}, err
	}
	animated, err := isAnimated(b)
	if err != nil {
		return Decoded{}, err
	}
	if animated && len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return decodeAnimatedWebP(ctx, b, l)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return Decoded{}, fmt.Errorf("image header: %w", err)
	}
	if !validDimensions(cfg.Width, cfg.Height, l) {
		return Decoded{}, ErrLimit
	}
	src, format2, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return Decoded{}, fmt.Errorf("image decode: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return Decoded{}, err
	}
	if format != format2 || (format != "gif" && (src.Bounds().Dx() != cfg.Width || src.Bounds().Dy() != cfg.Height)) || !src.Bounds().In(image.Rect(0, 0, cfg.Width, cfg.Height)) {
		return Decoded{}, errors.New("decoded dimensions differ from header")
	}
	dst := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	if format == "gif" {
		draw.Draw(dst, src.Bounds(), src, src.Bounds().Min, draw.Src)
	} else {
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	}
	orientation := Normal
	if format == "jpeg" {
		orientation = exifOrientation(b)
	} else if format == "tiff" {
		orientation = tiffOrientation(b)
	}
	if orientation != Normal {
		dst = Transform(dst, orientation)
	}
	if err = ctx.Err(); err != nil {
		return Decoded{}, err
	}
	return Decoded{Pixels: dst, Format: format, Animated: animated}, nil
}

// DefaultWorkers is deliberately small because codec calls cannot be interrupted.
func DefaultWorkers() int {
	if runtime.GOOS == "android" {
		return 1
	}
	return 2
}

func validDimensions(w, h int, l Limits) bool {
	return w > 0 && h > 0 && int64(w) <= l.MaxPixels/int64(h) && int64(w) <= int64(int(^uint(0)>>1))/4/int64(h)
}

// decodeAnimatedWebP reconstructs a static WebP from the first ANMF payload.
func decodeAnimatedWebP(ctx context.Context, b []byte, l Limits) (Decoded, error) {
	canvas, frame, bg, err := webpPoster(b)
	if err != nil {
		return Decoded{}, err
	}
	if !validDimensions(canvas.Dx(), canvas.Dy(), l) {
		return Decoded{}, ErrLimit
	}
	// Check the compressed frame dimensions independently of the VP8X header,
	// which would otherwise hide mismatches and permit an oversized allocation.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(frame.raw))
	if err != nil {
		return Decoded{}, fmt.Errorf("webp frame header: %w", err)
	}
	if cfg.Width != frame.rect.Dx() || cfg.Height != frame.rect.Dy() {
		return Decoded{}, errors.New("webp frame dimensions differ from header")
	}
	src, _, err := image.Decode(bytes.NewReader(frame.data))
	if err != nil {
		return Decoded{}, fmt.Errorf("webp frame decode: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Decoded{}, err
	}
	dst := image.NewNRGBA(canvas)
	draw.Draw(dst, canvas, &image.Uniform{C: color.NRGBA(bg)}, image.Point{}, draw.Src)
	op := draw.Over
	if frame.noBlend {
		op = draw.Src
	}
	draw.Draw(dst, frame.rect, src, src.Bounds().Min, op)
	if err := ctx.Err(); err != nil {
		return Decoded{}, err
	}
	return Decoded{Pixels: dst, Format: "webp", Animated: true}, nil
}
