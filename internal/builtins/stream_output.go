package builtins

import "io"

// trackedOutput remembers a write failure even when formatting helpers ignore it.
type trackedOutput struct {
	writer io.Writer
	err    error
}

func (w *trackedOutput) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}
