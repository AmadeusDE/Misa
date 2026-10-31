package term

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	conKernel = windows.NewLazySystemDLL("kernel32.dll")
	conCreate = conKernel.NewProc("CreatePseudoConsole")
	conResize = conKernel.NewProc("ResizePseudoConsole")
	conClose  = conKernel.NewProc("ClosePseudoConsole")
)

func conAvailable() error {
	for _, p := range []*windows.LazyProc{conCreate, conResize, conClose} {
		if err := p.Find(); err != nil {
			return fmt.Errorf("%w: ConPTY: %v", ErrUnsupported, err)
		}
	}
	return nil
}

func conSize(rows, cols int) (uintptr, error) {
	if rows < 1 || rows > 32767 || cols < 1 || cols > 32767 {
		return 0, fmt.Errorf("invalid ConPTY size %dx%d", cols, rows)
	}
	return uintptr(uint32(cols) | uint32(rows)<<16), nil
}

func conResult(op string, result uintptr) error {
	if int32(result) < 0 {
		return fmt.Errorf("%s: HRESULT %#x", op, uint32(result))
	}
	return nil
}

func createConsole(rows, cols int, input, output windows.Handle) (windows.Handle, error) {
	size, err := conSize(rows, cols)
	if err != nil {
		return 0, err
	}
	var h windows.Handle
	r, _, _ := conCreate.Call(size, uintptr(input), uintptr(output), 0, uintptr(unsafe.Pointer(&h)))
	return h, conResult("CreatePseudoConsole", r)
}

// closeBound limits each wait in Close: a ConPTY host that never answers must
// not hang the UI, and the goroutine finishing the close is not leaked handles.
const closeBound = 5 * time.Second

// windowsConn owns both pipe ends, the pseudoconsole and the job.
//
// Output ends only when the pseudoconsole is closed, and ClosePseudoConsole may
// block while its output is unread. So the console is closed from its own
// goroutine while something always reads: the Terminal's reader after a
// natural exit (final output is delivered in order, then EOF), or Close's
// drain when the reader is absent or stuck.
type windowsConn struct {
	in, out     *winPipe
	mu          sync.Mutex
	console     windows.Handle
	consoleDone chan struct{}
	jobMu       sync.Mutex
	job         windows.Handle
	closeOnce   sync.Once
	closed      chan struct{}
}

func newWindowsConn(in, out windows.Handle, console, job windows.Handle) *windowsConn {
	return &windowsConn{in: newWinPipe(in), out: newWinPipe(out), console: console, job: job,
		consoleDone: make(chan struct{}), closed: make(chan struct{})}
}

func (c *windowsConn) Read(b []byte) (int, error)  { return c.out.Read(b) }
func (c *windowsConn) Write(b []byte) (int, error) { return c.in.Write(b) }
func (c *windowsConn) Resize(rows, cols int) error {
	size, err := conSize(rows, cols)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.console == 0 {
		return os.ErrClosed
	}
	r, _, _ := conResize.Call(uintptr(c.console), size)
	return conResult("ResizePseudoConsole", r)
}

// closeConsole closes the pseudoconsole once; the lock is not held across the
// call, which can block until the output is read.
func (c *windowsConn) closeConsole() {
	c.mu.Lock()
	h := c.console
	c.console = 0
	c.mu.Unlock()
	if h != 0 {
		conClose.Call(uintptr(h))
		close(c.consoleDone)
	}
}

// kill ends every process in the job, including descendants that outlived the
// leader. The job stays open until Close.
func (c *windowsConn) kill() {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	if c.job != 0 {
		_ = windows.TerminateJobObject(c.job, 1)
	}
}

func (c *windowsConn) releaseJob() {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	if c.job != 0 {
		_ = windows.TerminateJobObject(c.job, 1)
		windows.CloseHandle(c.job)
		c.job = 0
	}
}

// Close is idempotent; every caller returns after teardown has finished. Order:
// kill the tree (nothing writes any more), drop input (cancels a blocked
// writer), close the console while draining output until EOF, then cancel and
// close the output pipe whatever happened.
func (c *windowsConn) Close() error {
	c.closeOnce.Do(func() {
		defer close(c.closed)
		c.releaseJob()
		c.in.close()
		go c.closeConsole()
		drained := make(chan struct{})
		go func() {
			defer close(drained)
			buf := make([]byte, 32*1024)
			for {
				if _, err := c.out.Read(buf); err != nil {
					return
				}
			}
		}()
		bound := time.NewTimer(closeBound)
		defer bound.Stop()
		select {
		case <-drained:
		case <-bound.C:
		}
		c.out.close()
		<-drained
		select {
		case <-c.consoleDone:
		case <-time.After(closeBound):
		}
	})
	<-c.closed
	return nil
}
