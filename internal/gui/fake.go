package gui

import (
	"image"
	"sync"
	"time"
)

// fakeBackend is a headless window for core tests. It delivers Events and
// records the frames the core presents.
type fakeBackend struct {
	w, h       int
	scale      float64
	format     PixelFormat
	title      string
	minW, minH int
	canPresent bool
	noBuffer   bool

	events chan Event
	clip   *memClipboard

	mu      sync.Mutex
	pix     []byte
	stride  int
	pw, ph  int
	content frame
	frames  []fakeFrame
	commits int
	drops   int
	err     error
}

type fakeFrame struct {
	pix    []byte
	stride int
	format PixelFormat
	damage []image.Rectangle
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		scale:      1,
		format:     FormatBGRA8888,
		canPresent: true,
		events:     make(chan Event, 32),
		clip:       newMemClipboard(),
	}
}

func (f *fakeBackend) Size() (int, int, float64) {
	scale := f.scale
	if scale == 0 {
		scale = 1
	}
	return f.w, f.h, scale
}

func (f *fakeBackend) Format() PixelFormat { return f.format }

func (f *fakeBackend) CanPresent() bool { return f.canPresent }

func (f *fakeBackend) Buffer(pw, ph int) *Target {
	if !f.canPresent || f.noBuffer || pw <= 0 || ph <= 0 {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	stride := pw * 4
	if f.pw != pw || f.ph != ph || len(f.pix) != stride*ph {
		f.pix = make([]byte, stride*ph)
		f.stride = stride
		f.pw, f.ph = pw, ph
		f.content = frame{}
	}
	return &Target{Pix: f.pix, Stride: f.stride, ID: 1, content: &f.content}
}

func (f *fakeBackend) Present(pixels []byte, stride int, format PixelFormat, damage []image.Rectangle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frames = append(f.frames, fakeFrame{
		pix:    append([]byte(nil), pixels...),
		stride: stride,
		format: format,
		damage: append([]image.Rectangle(nil), damage...),
	})
}

func (f *fakeBackend) Commit() { f.commits++ }
func (f *fakeBackend) DropBuffers() {
	f.drops++
	f.pix = nil
	f.pw, f.ph = 0, 0
	f.content = frame{}
}

func (f *fakeBackend) SetTitle(title string) { f.title = title }

func (f *fakeBackend) SetMinSize(w, h int) { f.minW, f.minH = w, h }

func (f *fakeBackend) Events() <-chan Event     { return f.events }
func (f *fakeBackend) Ready() <-chan struct{}   { return nil }
func (f *fakeBackend) Dispatch(Host)            {}
func (f *fakeBackend) DispatchRepeat(Host)      {}
func (f *fakeBackend) Repeat() <-chan time.Time { return nil }

func (f *fakeBackend) Clipboard() Clipboard { return f.clip }
func (f *fakeBackend) Close()               {}
func (f *fakeBackend) Err() error           { return f.err }

func (f *fakeBackend) push(ev Event) { f.events <- ev }

func (f *fakeBackend) frameCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.frames)
}

type memClipboard struct {
	mu     sync.Mutex
	text   string
	writes int
	pastes chan string
	async  bool
}

func newMemClipboard() *memClipboard {
	return &memClipboard{pastes: make(chan string, 4)}
}

func (c *memClipboard) Read() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.async {
		c.pastes <- c.text
		return "", nil
	}
	return c.text, nil
}

func (c *memClipboard) Write(text string) error {
	c.mu.Lock()
	c.text = text
	c.writes++
	c.mu.Unlock()
	return nil
}

func (c *memClipboard) Pastes() <-chan string { return c.pastes }

var _ Backend = (*fakeBackend)(nil)
