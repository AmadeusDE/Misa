package term

import (
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func screenText(tm *Terminal) string { return strings.Join(tm.PlainLines(), "\n") }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// writeConn adapts a write-only file to conn for tests that only watch replies.
type writeConn struct{ io.WriteCloser }

func (writeConn) Read([]byte) (int, error) { select {} }
func (writeConn) Resize(int, int) error    { return nil }

// fakeConn is a transport with no PTY and no process: the test plays the
// program by feeding output and reading what the core wrote.
type fakeConn struct {
	out    chan []byte
	closed chan struct{}
	once   sync.Once

	mu      sync.Mutex
	written []byte
	rows    int
	cols    int
}

func newFakeConn() *fakeConn {
	return &fakeConn{out: make(chan []byte, 16), closed: make(chan struct{})}
}

func (c *fakeConn) Read(p []byte) (int, error) {
	select {
	case b := <-c.out:
		return copy(p, b), nil
	case <-c.closed:
		return 0, io.EOF
	}
}

func (c *fakeConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.written = append(c.written, p...)
	c.mu.Unlock()
	return len(p), nil
}

func (c *fakeConn) Close() error { c.once.Do(func() { close(c.closed) }); return nil }

func (c *fakeConn) Resize(rows, cols int) error {
	c.mu.Lock()
	c.rows, c.cols = rows, cols
	c.mu.Unlock()
	return nil
}

func (c *fakeConn) wrote() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return string(c.written)
}

type fakeProc struct {
	status chan int
	kills  atomic.Int32
}

func (p *fakeProc) ID() int            { return 1 }
func (p *fakeProc) Status() <-chan int { return p.status }
func (p *fakeProc) Kill() {
	p.kills.Add(1)
	select { // a killed process reports a signal status, once
	case p.status <- 137:
	default:
	}
}

func fakeTerminal(rows, cols int) (*Terminal, *fakeConn, *fakeProc) {
	tm := New(rows, cols)
	c, p := newFakeConn(), &fakeProc{status: make(chan int, 1)}
	tm.conn, tm.proc = c, p
	return tm, c, p
}

func TestFakeTransportFinalOutputPrecedesExit(t *testing.T) {
	tm, c, p := fakeTerminal(3, 20)
	tm.StartReader()
	c.out <- []byte("last words")
	p.status <- 3
	// the transport reports EOF only after the program is gone
	waitFor(t, "output", func() bool { return strings.Contains(screenText(tm), "last words") })
	c.Close()
	waitExited(t, tm)
	if !tm.Done() || tm.ExitCode() != 3 {
		t.Errorf("done=%v code=%d", tm.Done(), tm.ExitCode())
	}
	if !strings.Contains(screenText(tm), "last words") {
		t.Error("exit reported before final output was applied")
	}
}

func TestFakeTransportInputAndRepliesAreOrdered(t *testing.T) {
	tm, c, _ := fakeTerminal(3, 20)
	tm.StartReader()
	defer tm.Close()
	tm.SendBytes([]byte("a"))
	tm.SendBytes([]byte("b"))
	tm.SendBytes([]byte("c"))
	waitFor(t, "writes", func() bool { return c.wrote() == "abc" })
}

func TestFakeTransportResizeAndCloseKillOnce(t *testing.T) {
	tm, c, p := fakeTerminal(3, 20)
	tm.StartReader()
	tm.Resize(9, 41)
	c.mu.Lock()
	rows, cols := c.rows, c.cols
	c.mu.Unlock()
	if rows != 9 || cols != 41 {
		t.Errorf("transport size %dx%d, want 9x41", rows, cols)
	}
	p.status <- 0
	tm.Close()
	tm.Close()
	waitExited(t, tm)
	if p.kills.Load() == 0 {
		t.Error("Close did not kill the process tree")
	}
	select {
	case <-c.closed:
	case <-time.After(time.Second):
		t.Error("Close did not close the transport")
	}
}

func TestFakeTransportHasNoJobControl(t *testing.T) {
	tm, _, _ := fakeTerminal(3, 20)
	if tm.Stopped() {
		t.Error("stopped without a job-control transport")
	}
	if err := tm.Continue(); err == nil {
		t.Error("Continue succeeded on a transport without job control")
	}
}
