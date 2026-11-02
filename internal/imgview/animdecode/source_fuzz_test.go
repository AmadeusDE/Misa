package animdecode

import (
	"context"
	"testing"
)

func FuzzSourceComposition(f *testing.F) {
	f.Add(gifFixture(f))
	f.Add(wpFixture(f))
	f.Fuzz(func(t *testing.T, data []byte) {
		const maxEncoded = 64 << 10
		if len(data) > maxEncoded {
			return
		}
		limits := Limits{MaxEncodedBytes: maxEncoded, MaxPixels: 4096, MaxFrames: 64, MaxMetadataBytes: 64 << 10, MaxWorkingBytes: 1 << 20}
		s, err := Open(context.Background(), data, limits)
		if err != nil {
			return
		}
		defer s.Close()
		_, _ = s.DecodeFrame(context.Background(), s.Metadata().Frames-1)
		_, _ = s.DecodeFrame(context.Background(), 0)
		_, _ = s.Poster(context.Background())
	})
}
