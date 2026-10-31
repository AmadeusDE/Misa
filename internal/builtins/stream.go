package builtins

import (
	"context"
	"io"
	"os"
)

const streamChunk = 32 * 1024

type streamReadResult struct {
	data []byte
	err  error
}
type contextReader struct {
	ctx    context.Context
	source io.Reader
}

// ContextReader checks cancellation between chunks and while waiting for Read.
// Sequential reads use at most one worker, which may remain in an uninterruptible
// source Read until the source unblocks/closes. It never closes a caller-owned
// source or gives the worker the caller's buffer. Workers exit after each Read;
// no idle goroutines are retained for previously opened files.
func ContextReader(ctx context.Context, source io.Reader) io.Reader {
	if ctx == nil || ctx.Done() == nil {
		return source
	}
	return &contextReader{ctx: ctx, source: source}
}
func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	result := make(chan streamReadResult, 1)
	size := min(len(p), streamChunk)
	go func() {
		if err := r.ctx.Err(); err != nil {
			result <- streamReadResult{err: err}
			return
		}
		buffer := make([]byte, size)
		n, err := r.source.Read(buffer)
		result <- streamReadResult{buffer[:n], err}
	}()
	select {
	case read := <-result:
		return copy(p, read.data), read.err
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}

type streamWriteResult struct {
	n   int
	err error
}
type contextWriter struct {
	ctx  context.Context
	sink io.Writer
}

// ContextWriter is the write counterpart of ContextReader. A blocked sink must
// eventually unblock/close to release its worker. Writes use owned copies so
// cancellation never leaves a worker accessing the command's buffers.
func ContextWriter(ctx context.Context, sink io.Writer) io.Writer {
	if ctx == nil || ctx.Done() == nil {
		return sink
	}
	return &contextWriter{ctx: ctx, sink: sink}
}
func (w *contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	total := 0
	for len(p) > 0 {
		size := min(len(p), streamChunk)
		data := append([]byte(nil), p[:size]...)
		result := make(chan streamWriteResult, 1)
		go func() {
			if err := w.ctx.Err(); err != nil {
				result <- streamWriteResult{err: err}
				return
			}
			n, err := w.sink.Write(data)
			result <- streamWriteResult{n, err}
		}()
		select {
		case write := <-result:
			total += write.n
			if write.err != nil {
				return total, write.err
			}
			if write.n != size {
				return total, io.ErrShortWrite
			}
		case <-w.ctx.Done():
			return total, w.ctx.Err()
		}
		p = p[size:]
	}
	return total, nil
}

// CopyContext deliberately bypasses ReaderFrom/WriterTo fast paths so every
// chunk observes cancellation and the first read/write error ends the copy.
func CopyContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return io.CopyBuffer(struct{ io.Writer }{ContextWriter(ctx, dst)}, struct{ io.Reader }{ContextReader(ctx, src)}, make([]byte, streamChunk))
}

// ReadFileContext closes its owned file even when cancellation interrupts a read.
func ReadFileContext(ctx context.Context, path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return io.ReadAll(ContextReader(ctx, file))
}

func streamContext(c *Context) (*Context, func()) {
	copy := *c
	cancelCtx, cancel := context.WithCancel(c.CancellationContext())
	copy.Ctx = cancelCtx
	if c.Stdin != nil {
		copy.Stdin = ContextReader(copy.Ctx, c.Stdin)
	}
	if c.Stdout != nil {
		copy.Stdout = ContextWriter(copy.Ctx, c.Stdout)
	}
	if c.Stderr != nil {
		copy.Stderr = ContextWriter(copy.Ctx, c.Stderr)
	}
	return &copy, cancel
}
func streamResult(c *Context, code *int) {
	if err := c.CancellationContext().Err(); err != nil {
		*code = ExitStatus(err)
	}
}
