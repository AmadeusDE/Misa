package term

import (
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// winPipe is one end of an anonymous pipe with blocking I/O. Unlike os.File it
// can always be closed: close cancels the reads and writes other goroutines are
// blocked in, waits for them to leave, and only then closes the handle.
type winPipe struct {
	mu      sync.Mutex
	h       windows.Handle
	busy    int
	closing bool
	once    sync.Once
}

func newWinPipe(h windows.Handle) *winPipe { return &winPipe{h: h} }

func (p *winPipe) enter() (windows.Handle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return 0, os.ErrClosed
	}
	p.busy++
	return p.h, nil
}

func (p *winPipe) leave() (closing bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.busy--
	return p.closing
}

func (p *winPipe) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	h, err := p.enter()
	if err != nil {
		return 0, err
	}
	var n uint32
	err = windows.ReadFile(h, b, &n, nil)
	if p.leave() && err != nil {
		err = os.ErrClosed
	}
	if err == windows.ERROR_BROKEN_PIPE || err == windows.ERROR_HANDLE_EOF {
		err = io.EOF
	}
	return int(n), err
}

func (p *winPipe) Write(b []byte) (int, error) {
	total := 0
	for len(b) > 0 {
		h, err := p.enter()
		if err != nil {
			return total, err
		}
		var n uint32
		err = windows.WriteFile(h, b, &n, nil)
		if p.leave() && err != nil {
			err = os.ErrClosed
		}
		total += int(n)
		if err != nil {
			return total, err
		}
		b = b[n:]
	}
	return total, nil
}

// close is safe to call repeatedly and concurrently. The cancel is retried
// because an operation that passed enter may not have reached the kernel yet.
func (p *winPipe) close() {
	p.once.Do(func() {
		p.mu.Lock()
		p.closing = true
		h := p.h
		p.mu.Unlock()
		for {
			_ = windows.CancelIoEx(h, nil)
			p.mu.Lock()
			idle := p.busy == 0
			p.mu.Unlock()
			if idle {
				break
			}
			time.Sleep(time.Millisecond)
		}
		windows.CloseHandle(h)
	})
}
