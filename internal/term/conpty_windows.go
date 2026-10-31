package term

import (
	"fmt"
	"io"
	"os"
	"sync"
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

type windowsConn struct {
	in, out *os.File
	mu      sync.Mutex
	console windows.Handle
	jobMu   sync.Mutex
	job     windows.Handle
	once    sync.Once
}

func (c *windowsConn) Read(b []byte) (int, error) {
	n, err := c.out.Read(b)
	if err == windows.ERROR_BROKEN_PIPE {
		err = io.EOF
	}
	return n, err
}
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
func (c *windowsConn) closeConsole() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.console != 0 {
		conClose.Call(uintptr(c.console))
		c.console = 0
	}
}

func (c *windowsConn) kill() {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	if c.job != 0 {
		_ = windows.TerminateJobObject(c.job, 1)
	}
}

func (c *windowsConn) Close() error {
	c.once.Do(func() {
		c.jobMu.Lock()
		if c.job != 0 {
			_ = windows.TerminateJobObject(c.job, 1)
			windows.CloseHandle(c.job)
			c.job = 0
		}
		c.jobMu.Unlock()
		// TODO(P03): prove bounded close and full output draining with ConPTY.
		c.in.Close()
		c.out.Close()
		c.closeConsole()
	})
	return nil
}
