// Package gui is Misa's window frontend: an OS-neutral terminal session and
// software renderer, with one backend per platform. Linux (other than
// Android) presents through Wayland. Android and the other targets error
// cleanly until their backends exist.
package gui

import (
	"image"
	"time"

	tea "charm.land/bubbletea/v2"
)

// PixelFormat is the byte order of a presented frame. Both formats store
// premultiplied components. Wayland's ARGB8888 and XRGB8888, and a Win32
// top-down 32-bit DIB, are BGRA in memory. Android's WINDOW_FORMAT_RGBA_8888
// is RGBA. The core draws straight into Backend.Format so the hot path does
// not swizzle; Present swizzles only a buffer whose format differs.
type PixelFormat uint8

const (
	FormatBGRA8888 PixelFormat = iota // B, G, R, A
	FormatRGBA8888                    // R, G, B, A
)

// EventKind is the platform-neutral window event the core applies.
type EventKind uint8

const (
	EventResize EventKind = iota
	EventScale            // buffer scale changed; logical size stays
	EventKey
	EventText // IME commit or other text with no key code
	EventPointer
	EventScroll
	EventFocus
	EventClose
	EventFrame // a frame may be presented again (callback done, buffer free)
)

// PointerAction is a pointer event's phase. Coordinates are logical pixels,
// the same space Size reports.
type PointerAction uint8

const (
	PointerMove PointerAction = iota
	PointerDown
	PointerUp
	PointerEnter
)

// Event is one window event after the backend has translated it.
// Scroll Steps are wheel steps, positive down or to the right; a Wayland
// backend accumulates value120 and continuous axes before emitting it.
type Event struct {
	Kind EventKind

	Width, Height int
	Scale         float64

	Key         tea.Key
	Text        string
	DismissHold bool // a non-modifier key press; closes a held screen

	X, Y    float64
	Button  tea.MouseButton
	Action  PointerAction
	Axis    int
	Steps   float64
	Mod     tea.KeyMod
	Focused bool
}

// Clipboard is the window selection. Write publishes text. Read requests
// the current selection: Wayland returns no text and delivers the bytes
// later on Pastes, because the compositor writes them on a pipe. A backend
// that can read synchronously returns the text from Read.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
	Pastes() <-chan string
}

// Target is the pixel memory the core draws into. ID identifies the buffer
// for damage tracking when a backend rotates more than one. content, when
// set by a same-package backend, is that buffer's own last frame; a nil
// content uses the core's frame for ID. Nil content means the buffer must
// already hold the last presented pixels outside the new damage.
type Target struct {
	Pix     []byte
	Stride  int
	ID      uint64
	content *frame
}

// Backend is one platform window. The core calls it from a single
// goroutine, which is also the goroutine that dispatches platform events,
// so a Wayland Present can allocate objects and send requests inline.
//
// Ready and Dispatch are the Wayland shape from the Windows plan: Ready is
// readable when a protocol message is queued, and Dispatch runs on the app
// goroutine and calls Host before it returns, so a configure still draws
// before the next message. Events is the channel shape from the Android
// plan, used by the fake backend and by window threads (Win32, Android)
// that translate on their own. A backend sets one of the two and leaves
// the other nil.
type Backend interface {
	Size() (w, h int, scale float64)
	Format() PixelFormat
	CanPresent() bool
	// Buffer returns pw×ph pixels in Format, or nil when none is free.
	Buffer(pw, ph int) *Target
	// Present submits pixels. damage is in buffer pixels. When pixels is
	// the slice last returned by Buffer and format is Format, the Wayland
	// backend attaches that shared memory with no copy and no swizzle.
	Present(pixels []byte, stride int, format PixelFormat, damage []image.Rectangle)
	// Commit flushes a configure that produced no new frame.
	Commit()
	// DropBuffers retires buffers after a zoom or a scale change.
	DropBuffers()
	SetTitle(string)
	SetMinSize(w, h int)

	Events() <-chan Event
	Ready() <-chan struct{}
	Dispatch(h Host)
	// DispatchRepeat delivers one repeat of the held key.
	DispatchRepeat(h Host)
	// Repeat fires while a key is held. Nil when nothing is armed; Win32
	// leaves it nil and emits repeated keys as Events.
	Repeat() <-chan time.Time

	Clipboard() Clipboard
	Close()
	Err() error
}

// Host receives translated events on the app goroutine.
type Host interface {
	Handle(Event)
}
