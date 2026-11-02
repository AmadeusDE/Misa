package imgview

import (
	"context"
	"errors"

	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

// OpenAnimation returns a bounded, on-demand frame source backed by an immutable
// file snapshot. The caller owns the source and must close it. Static images
// other than GIF are not sequences and return an unsupported-container error.
func OpenAnimation(ctx context.Context, path string, limits Limits) (*animdecode.Source, error) {
	l := limits.normalized()
	b, err := readEncoded(ctx, path, l)
	if err != nil {
		return nil, err
	}
	return openAnimation(ctx, b, l)
}

func openAnimation(ctx context.Context, b []byte, l Limits) (*animdecode.Source, error) {
	a := l.Animation
	// Outer image limits always constrain nested animation limits.
	if a.MaxEncodedBytes <= 0 || a.MaxEncodedBytes > l.MaxEncodedBytes {
		a.MaxEncodedBytes = l.MaxEncodedBytes
	}
	if a.MaxPixels <= 0 || a.MaxPixels > l.MaxPixels {
		a.MaxPixels = l.MaxPixels
	}
	s, err := animdecode.Open(ctx, b, a)
	if errors.Is(err, animdecode.ErrLimit) {
		return nil, errors.Join(ErrLimit, err)
	}
	return s, err
}

func decodeAnimationPoster(ctx context.Context, b []byte, l Limits) (Decoded, error) {
	release, err := reserveAnimationWork(l)
	if err != nil {
		return Decoded{}, err
	}
	defer release()
	s, err := openAnimation(ctx, b, l)
	if err != nil {
		return Decoded{}, err
	}
	defer s.Close()
	pixels, err := s.Poster(ctx)
	if errors.Is(err, animdecode.ErrLimit) {
		err = errors.Join(ErrLimit, err)
	}
	if err != nil {
		return Decoded{}, err
	}
	format := s.Format()
	if format == "apng" {
		format = "png"
	}
	return Decoded{Pixels: pixels, Format: format, Animated: true}, nil
}

func animationWorkLimit(l Limits) int64 {
	if l.Animation.MaxWorkingBytes > 0 {
		return l.Animation.MaxWorkingBytes
	}
	return animdecode.DefaultLimits().MaxWorkingBytes
}

func reserveAnimationWork(l Limits) (func(), error) {
	if l.workCache == nil {
		return func() {}, nil
	}
	return l.workCache.reserve(animationWorkLimit(l))
}
