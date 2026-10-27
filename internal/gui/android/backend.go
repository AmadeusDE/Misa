package android

import (
	"image"
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/gui"
)

// Backend is the Android Backend: an event queue plus a Surface. The core
// draws RGBA into Buffer; Present copies damaged rows into the window at
// the inset origin. Only the active session has presenting set, and it is
// cleared while the surface is gone, so a destroyed surface does not end
// the terminal.
type Backend struct {
	mu         sync.Mutex
	events     chan gui.Event
	log        []gui.Event
	w, h       int
	scale      float64
	title      string
	minW       int
	minH       int
	presenting bool
	pix        []byte
	stride     int
	pw, ph     int
	bufID      uint64
	originX    int
	originY    int
	winW       int
	winH       int
	surface    Surface
	clip       gui.Clipboard
	err        error
	closed     bool
}

func newBackend(clip gui.Clipboard) *Backend {
	if clip == nil {
		clip = newMemClip()
	}
	return &Backend{
		events: make(chan gui.Event, 256),
		scale:  1,
		bufID:  1,
		clip:   clip,
	}
}

func (b *Backend) Size() (int, int, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	scale := b.scale
	if scale <= 0 {
		scale = 1
	}
	return b.w, b.h, scale
}

func (b *Backend) Format() gui.PixelFormat { return gui.FormatRGBA8888 }

func (b *Backend) CanPresent() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.presenting && b.surface != nil
}

func (b *Backend) Buffer(pw, ph int) *gui.Target {
	if pw <= 0 || ph <= 0 {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	stride := pw * 4
	if b.pw != pw || b.ph != ph || len(b.pix) != stride*ph {
		b.pix = make([]byte, stride*ph)
		b.stride = stride
		b.pw, b.ph = pw, ph
	}
	return &gui.Target{Pix: b.pix, Stride: b.stride, ID: b.bufID}
}

func (b *Backend) Present(pixels []byte, stride int, format gui.PixelFormat, damage []image.Rectangle) {
	b.mu.Lock()
	surf := b.surface
	on := b.presenting
	ox, oy := b.originX, b.originY
	ww, wh := b.winW, b.winH
	pw, ph := b.pw, b.ph
	b.mu.Unlock()
	if !on || surf == nil || pw <= 0 || ph <= 0 || ww <= 0 || wh <= 0 {
		return
	}
	bounds, ok := DamageBounds(damage, ox, oy, ww, wh)
	if !ok {
		return
	}
	surf.Present(Frame{
		Src: pixels, SrcStride: stride, SrcW: pw, SrcH: ph,
		OriginX: ox, OriginY: oy, WindowW: ww, WindowH: wh,
		Bounds: bounds, Swap: format != gui.FormatRGBA8888,
	})
}

func (b *Backend) Commit() {}

func (b *Backend) DropBuffers() {
	b.mu.Lock()
	b.pix = nil
	b.pw, b.ph = 0, 0
	b.bufID++
	b.mu.Unlock()
}

func (b *Backend) SetTitle(title string) {
	b.mu.Lock()
	b.title = title
	b.mu.Unlock()
}

func (b *Backend) SetMinSize(w, h int) {
	b.mu.Lock()
	b.minW, b.minH = w, h
	b.mu.Unlock()
}

func (b *Backend) Events() <-chan gui.Event { return b.events }

func (b *Backend) Ready() <-chan struct{} { return nil }

func (b *Backend) Dispatch(gui.Host) {}

func (b *Backend) DispatchRepeat(gui.Host) {}

func (b *Backend) Repeat() <-chan time.Time { return nil }

func (b *Backend) Clipboard() gui.Clipboard { return b.clip }

func (b *Backend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	b.presenting = false
	close(b.events)
}

func (b *Backend) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func (b *Backend) setPresenting(on bool) {
	b.mu.Lock()
	b.presenting = on
	b.mu.Unlock()
}

func (b *Backend) setSurface(s Surface) {
	b.mu.Lock()
	b.surface = s
	b.mu.Unlock()
}

func (b *Backend) pushGeom(g Geometry) {
	w, h, scale := g.Logical()
	b.mu.Lock()
	if w > 0 && h > 0 {
		b.w, b.h, b.scale = w, h, scale
	}
	b.originX = g.Insets.Left
	b.originY = g.Insets.Top
	b.winW, b.winH = g.Width, g.Height
	b.mu.Unlock()
	if w > 0 && h > 0 {
		b.emit(gui.Event{Kind: gui.EventResize, Width: w, Height: h, Scale: scale})
	}
}

func (b *Backend) emit(ev gui.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.note(ev)
	select {
	case b.events <- ev:
		return
	default:
	}
	if ev.Kind == gui.EventPointer || ev.Kind == gui.EventScroll {
		return
	}
	select {
	case <-b.events:
	default:
	}
	select {
	case b.events <- ev:
	default:
	}
}

func (b *Backend) note(ev gui.Event) {
	b.log = append(b.log, ev)
	if len(b.log) > 32 {
		b.log = append([]gui.Event(nil), b.log[len(b.log)-32:]...)
	}
}

func (b *Backend) eventsLogged() []gui.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]gui.Event(nil), b.log...)
}

func (b *Backend) bufferID() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bufID
}

var _ gui.Backend = (*Backend)(nil)
