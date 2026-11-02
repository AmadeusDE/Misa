package animdecode

import (
	"context"
	"fmt"
	"image"
	"image/draw"

	"github.com/AmadeusDE/Misa/internal/imageframe"
)

func imageBounds(idx *index) image.Rectangle { return image.Rectangle{Max: idx.size} }
func (s *Source) reset() {
	s.canvas = image.NewNRGBA(imageBounds(s.idx))
	draw.Draw(s.canvas, s.canvas.Bounds(), image.NewUniform(s.idx.background), image.Point{}, draw.Src)
	s.next = 0
}
func (s *Source) DecodeFrame(ctx context.Context, n int) (imageframe.DecodedFrame, error) {
	if s.idx == nil {
		return imageframe.DecodedFrame{}, ErrClosed
	}
	if n < 0 || n >= len(s.idx.frames) {
		return imageframe.DecodedFrame{}, fmt.Errorf("frame %d out of range", n)
	}
	if err := ctx.Err(); err != nil {
		return imageframe.DecodedFrame{}, err
	}
	if s.canvas == nil || n < s.next {
		s.reset()
	}
	for s.next <= n {
		if err := ctx.Err(); err != nil {
			s.canvas = nil
			return imageframe.DecodedFrame{}, err
		}
		f := s.idx.frames[s.next]
		var saved *image.NRGBA
		if f.dispose == previous {
			saved = image.NewNRGBA(f.rect)
			draw.Draw(saved, f.rect, s.canvas, f.rect.Min, draw.Src)
		}
		im, err := f.decode(ctx)
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			s.canvas = nil
			return imageframe.DecodedFrame{}, err
		}
		if im == nil || im.Bounds().Dx() != f.rect.Dx() || im.Bounds().Dy() != f.rect.Dy() {
			s.canvas = nil
			return imageframe.DecodedFrame{}, ErrMalformed
		}
		op := draw.Over
		if f.source {
			op = draw.Src
		}
		draw.Draw(s.canvas, f.rect, im, im.Bounds().Min, op)
		var out *image.NRGBA
		if s.next == n {
			out = image.NewNRGBA(s.canvas.Bounds())
			draw.Draw(out, out.Bounds(), s.canvas, image.Point{}, draw.Src)
		}
		switch f.dispose {
		case background:
			draw.Draw(s.canvas, f.rect, image.NewUniform(s.idx.background), image.Point{}, draw.Src)
		case previous:
			draw.Draw(s.canvas, f.rect, saved, f.rect.Min, draw.Src)
		}
		s.next++
		if err = ctx.Err(); err != nil {
			s.canvas = nil
			return imageframe.DecodedFrame{}, err
		}
		if out != nil {
			return imageframe.DecodedFrame{Index: n, Pixels: out, Duration: f.delay}, nil
		}
	}
	panic("unreachable")
}
func (s *Source) Poster(ctx context.Context) (*image.NRGBA, error) {
	if s.idx == nil {
		return nil, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.idx.poster == nil {
		f, err := s.DecodeFrame(ctx, 0)
		return f.Pixels, err
	}
	im, err := s.idx.poster(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if im == nil || im.Bounds().Size() != s.idx.size {
		return nil, ErrMalformed
	}
	out := image.NewNRGBA(imageBounds(s.idx))
	draw.Draw(out, out.Bounds(), im, im.Bounds().Min, draw.Src)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
