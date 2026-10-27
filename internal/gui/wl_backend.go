//go:build unix && !android

package gui

import (
	"fmt"
	"image"
	"os"
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/wayland"
)

// wlBackend is the Wayland window. Protocol objects are dispatched on the
// app goroutine; pump only queues raw events so a configure still draws
// before the next message is dispatched.
type wlBackend struct {
	conn *wayland.Conn
	win  *window
	seat seat
	clip wlClipboard
	host Host

	mu     sync.Mutex
	queue  []wayland.Event
	ready  chan struct{}
	err    error
	closed bool

	pending *buffer
}

func Run(dir string, argv []string, title string) int {
	code, err := runWL(dir, argv, title)
	if err != nil {
		fmt.Fprintln(os.Stderr, "misa: gui:", err)
		return 1
	}
	return code
}

func runWL(dir string, argv []string, title string) (int, error) {
	conn, err := wayland.Connect()
	if err != nil {
		return 1, err
	}
	be := &wlBackend{conn: conn, ready: make(chan struct{}, 256)}
	defer be.Close()
	reg := conn.GetRegistry()
	if err := conn.Roundtrip(); err != nil {
		return 1, err
	}
	cfg := config.LoadGUI()
	a := newApp(be, cfg, dir, argv)
	if be.win, err = newWindow(conn, reg, title, a.rend.pal.translucent()); err != nil {
		return 1, err
	}
	be.wire()
	be.bindSeat(reg)
	be.bindClipboard(reg)
	if err := a.loadFonts(); err != nil {
		return 1, err
	}
	go be.pump()
	return a.loop()
}

func (b *wlBackend) wire() {
	w := b.win
	w.onConfigure = func() {
		b.emit(Event{Kind: EventResize, Width: w.width, Height: w.height, Scale: float64(w.scale)})
	}
	w.onScale = func() {
		b.emit(Event{Kind: EventScale, Scale: float64(w.scale)})
	}
	w.onFrame = func() { b.emit(Event{Kind: EventFrame}) }
	w.onBufferRelease = func() { b.emit(Event{Kind: EventFrame}) }
	w.onClose = func() { b.emit(Event{Kind: EventClose}) }
}

func (b *wlBackend) pump() {
	for ev := range b.conn.Events() {
		b.mu.Lock()
		b.queue = append(b.queue, ev)
		b.mu.Unlock()
		b.ready <- struct{}{}
	}
	b.mu.Lock()
	if b.err == nil {
		b.err = b.conn.Err()
	}
	b.closed = true
	b.mu.Unlock()
	close(b.ready)
}

func (b *wlBackend) emit(ev Event) {
	if b.host != nil {
		b.host.Handle(ev)
	}
}

func (b *wlBackend) Size() (int, int, float64) {
	if b.win == nil {
		return 0, 0, 1
	}
	return b.win.width, b.win.height, float64(b.win.scale)
}

func (b *wlBackend) Format() PixelFormat { return FormatBGRA8888 }

func (b *wlBackend) CanPresent() bool {
	return b.win != nil && b.win.configured && b.win.frameCB == 0
}

func (b *wlBackend) Buffer(pw, ph int) *Target {
	if b.win == nil {
		return nil
	}
	buf := b.win.bufferFor(pw, ph)
	if buf == nil {
		return nil
	}
	b.pending = buf
	return &Target{Pix: buf.c.pix, Stride: buf.c.stride, ID: uint64(buf.id), content: &buf.content}
}

func (b *wlBackend) Present(pixels []byte, stride int, format PixelFormat, damage []image.Rectangle) {
	buf := b.pending
	if buf == nil {
		return
	}
	if !samePixels(pixels, buf.c.pix) || stride != buf.c.stride || format != FormatBGRA8888 {
		blit(buf.c.pix, buf.c.stride, FormatBGRA8888, pixels, stride, format, buf.c.w, buf.c.h)
	}
	b.win.present(buf, damage)
	b.pending = nil
}

func (b *wlBackend) Commit() {
	if b.win != nil {
		b.win.commit()
	}
}

func (b *wlBackend) DropBuffers() {
	if b.win != nil {
		b.win.dropBuffers()
	}
	b.pending = nil
}

func (b *wlBackend) SetTitle(title string) {
	if b.win != nil {
		b.win.setTitle(title)
	}
}

func (b *wlBackend) SetMinSize(w, h int) {
	if b.win != nil {
		b.win.setMinSize(w, h)
	}
}

func (b *wlBackend) Events() <-chan Event { return nil }

func (b *wlBackend) Ready() <-chan struct{} { return b.ready }

func (b *wlBackend) Dispatch(h Host) {
	b.host = h
	b.mu.Lock()
	if len(b.queue) == 0 {
		b.mu.Unlock()
		return
	}
	ev := b.queue[0]
	b.queue = b.queue[1:]
	b.mu.Unlock()
	b.conn.Dispatch(ev)
}

func (b *wlBackend) DispatchRepeat(h Host) {
	b.host = h
	b.repeatKey()
}

func (b *wlBackend) Repeat() <-chan time.Time { return b.seat.repeatC() }

func (b *wlBackend) Clipboard() Clipboard { return &b.clip }

func (b *wlBackend) Close() {
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
}

func (b *wlBackend) Err() error {
	if b.conn != nil {
		if err := b.conn.Err(); err != nil {
			return err
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

var _ Backend = (*wlBackend)(nil)
