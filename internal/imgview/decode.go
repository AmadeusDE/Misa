// Package imgview provides bounded, read-only image decoding and transforms.
package imgview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"runtime"

	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
	"github.com/AmadeusDE/Misa/internal/imgview/webpcodec"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var ErrLimit = errors.New("image exceeds configured limit")

// Limits bounds encoded input and decoded dimensions. Zero fields use defaults.
type Limits struct {
	MaxEncodedBytes int64
	MaxPixels       int64
	Animation       animdecode.Limits
	workCache       *Cache
}

func DefaultLimits() Limits { return Limits{MaxEncodedBytes: 64 << 20, MaxPixels: 32_000_000} }
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
	b, err := readEncoded(ctx, path, l)
	if err != nil {
		return Decoded{}, err
	}
	animated, err := isAnimated(b)
	if err != nil {
		return Decoded{}, err
	}
	if animated {
		return decodeAnimationPoster(ctx, b, l)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return Decoded{}, fmt.Errorf("image header: %w", err)
	}
	if !validDimensions(cfg.Width, cfg.Height, l) {
		return Decoded{}, ErrLimit
	}
	var src image.Image
	format2 := format
	if format == "webp" {
		if err := validateStaticWebP(b, cfg); err != nil {
			return Decoded{}, err
		}
		release, err := reserveAnimationWork(l)
		if err != nil {
			return Decoded{}, err
		}
		defer release()
		budget := animationWorkLimit(l)
		// Keep the returned NRGBA conversion within the same working budget.
		outputBytes := int64(cfg.Width) * int64(cfg.Height) * 4
		if outputBytes >= budget {
			return Decoded{}, ErrLimit
		}
		src, err = webpcodec.Decode(bytes.NewReader(b), budget-outputBytes)
		if errors.Is(err, webpcodec.ErrLimit) {
			return Decoded{}, ErrLimit
		}
	} else {
		src, format2, err = image.Decode(bytes.NewReader(b))
	}
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
