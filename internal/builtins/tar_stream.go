// Tar gzip streaming informed by Toybox tar.c (0BSD).
package builtins

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

func tarGzipWriter(w io.Writer) (io.Writer, func() error) {
	gz := gzip.NewWriter(w)
	return gz, gz.Close
}

// tarInput detects gzip and, before extraction, validates the entire stream in
// a temporary file. gzip's checksum is only verified at the end of the stream.
func tarInput(ctx *Context, r io.Reader, extract, requireGzip bool) (io.Reader, func(), bool, error) {
	br := bufio.NewReader(ContextReader(ctx.CancellationContext(), r))
	magic, err := br.Peek(2)
	if err != nil && err != io.EOF {
		return nil, nil, false, err
	}
	compressed := len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b
	if !compressed {
		if requireGzip {
			return nil, nil, false, fmt.Errorf("not a gzip stream")
		}
		return br, func() {}, false, nil
	}
	gz, err := gzip.NewReader(br)
	if err != nil {
		return nil, nil, false, err
	}
	if !extract {
		return gz, func() { _ = gz.Close() }, true, nil
	}
	f, err := os.CreateTemp("", "misa-tar-*")
	if err != nil {
		gz.Close()
		return nil, nil, false, err
	}
	cleanup := func() { f.Close(); os.Remove(f.Name()) }
	_, err = CopyContext(ctx.CancellationContext(), f, gz)
	if e := gz.Close(); err == nil {
		err = e
	}
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		cleanup()
		return nil, nil, false, err
	}
	return f, cleanup, true, nil
}
