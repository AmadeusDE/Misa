package imgview

import (
	"context"
	"errors"
	"io"
	"os"
)

// readEncoded takes a finite snapshot and detects replacement or mutation during
// the read. Decoders never retain offsets into a subsequently mutable file.
func readEncoded(ctx context.Context, path string, l Limits) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if st.Size() > l.MaxEncodedBytes {
		return nil, ErrLimit
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fst, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fst.Mode().IsRegular() || !os.SameFile(st, fst) {
		return nil, errors.New("file changed while opening")
	}
	// Avoid overflowing LimitReader's extra byte.
	if l.MaxEncodedBytes == int64(^uint64(0)>>1) {
		return nil, ErrLimit
	}
	b, err := io.ReadAll(io.LimitReader(f, l.MaxEncodedBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > l.MaxEncodedBytes {
		return nil, ErrLimit
	}
	end, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fst.Size() != end.Size() || !fst.ModTime().Equal(end.ModTime()) {
		return nil, errors.New("file changed while reading")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return b, nil
}
