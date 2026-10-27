//go:build unix && !android

package gui

import (
	"errors"
	"image"
	"strings"

	"github.com/AmadeusDE/Misa/internal/wayland"
)

var errMissingGlobals = errors.New("compositor lacks wl_compositor, wl_shm or xdg_wm_base")

// Opcodes of the requests and events used, from wayland.xml, xdg-shell.xml,
// xdg-decoration-unstable-v1.xml and cursor-shape-v1.xml.
const (
	compositorCreateSurface = 0
	shmCreatePool           = 0
	shmPoolCreateBuffer     = 0
	shmPoolDestroy          = 1
	bufferDestroy           = 0
	bufferEventRelease      = 0
	surfaceAttach           = 1
	surfaceDamage           = 2
	surfaceFrame            = 3
	surfaceCommit           = 6
	surfaceSetBufferScale   = 8
	surfaceDamageBuffer     = 9
	surfaceEventPrefScale   = 2
	callbackEventDone       = 0
	wmBasePong              = 3
	wmBaseGetXdgSurface     = 2
	wmBaseEventPing         = 0
	xdgSurfaceGetToplevel   = 1
	xdgSurfaceAckConfigure  = 4
	xdgSurfaceEventConfig   = 0
	toplevelSetTitle        = 2
	toplevelSetAppID        = 3
	toplevelSetMinSize      = 8
	toplevelEventConfigure  = 0
	toplevelEventClose      = 1
	decorationGetToplevel   = 1
	decorationSetMode       = 1
	decorationServerSide    = 2
	formatARGB8888          = 0
	formatXRGB8888          = 1
	maxBuffers              = 3
)

type buffer struct {
	id      uint32
	mem     *wayland.SharedMemory
	c       canvas
	busy    bool
	stale   bool
	content frame
}

// window is the toplevel surface and its buffers.
type window struct {
	conn          *wayland.Conn
	compositor    uint32
	compositorVer uint32
	shm           uint32
	format        uint32
	wmBase        uint32
	surface       uint32
	xdgSurface    uint32
	toplevel      uint32
	buffers       []*buffer
	frameCB       uint32

	width, height   int // logical size the compositor configured; 0 lets us pick
	pendingW        int
	pendingH        int
	scale           int
	configured      bool
	closeRequested  bool
	onConfigure     func()
	onScale         func()
	onFrame         func()
	onBufferRelease func()
	onClose         func()
}

// newWindow maps a toplevel; translucent windows get buffers with alpha.
func newWindow(conn *wayland.Conn, reg *wayland.Registry, title string, translucent bool) (*window, error) {
	w := &window{conn: conn, scale: 1, format: formatXRGB8888}
	if translucent {
		w.format = formatARGB8888
	}
	w.compositor, w.compositorVer = reg.Bind("wl_compositor", 6, nil)
	w.shm, _ = reg.Bind("wl_shm", 1, nil)
	w.wmBase, _ = reg.Bind("xdg_wm_base", 6, w.wmBaseEvent)
	if w.compositor == 0 || w.shm == 0 || w.wmBase == 0 {
		return nil, errMissingGlobals
	}
	w.surface = conn.NewObject(w.surfaceEvent)
	conn.Send(w.compositor, compositorCreateSurface, w.surface)
	w.xdgSurface = conn.NewObject(w.xdgSurfaceEvent)
	conn.Send(w.wmBase, wmBaseGetXdgSurface, w.xdgSurface, w.surface)
	w.toplevel = conn.NewObject(w.toplevelEvent)
	conn.Send(w.xdgSurface, xdgSurfaceGetToplevel, w.toplevel)
	w.setTitle(title)
	conn.Send(w.toplevel, toplevelSetAppID, "misa")
	if deco, _ := reg.Bind("zxdg_decoration_manager_v1", 1, nil); deco != 0 {
		td := conn.NewObject(nil)
		conn.Send(deco, decorationGetToplevel, td, w.toplevel)
		conn.Send(td, decorationSetMode, uint32(decorationServerSide))
	}
	conn.Send(w.surface, surfaceCommit)
	return w, nil
}

// maxTitle keeps set_title far below the 64 KiB Wayland message limit; a
// longer message desyncs the connection.
const maxTitle = 256

func (w *window) setTitle(title string) {
	if len(title) > maxTitle {
		title = strings.ToValidUTF8(title[:maxTitle], "")
	}
	w.conn.Send(w.toplevel, toplevelSetTitle, strings.ReplaceAll(title, "\x00", ""))
}

func (w *window) setMinSize(width, height int) {
	w.conn.Send(w.toplevel, toplevelSetMinSize, int32(width), int32(height))
}

func (w *window) wmBaseEvent(ev *wayland.Event) {
	if ev.Op == wmBaseEventPing {
		w.conn.Send(w.wmBase, wmBasePong, ev.Uint())
	}
}

func (w *window) surfaceEvent(ev *wayland.Event) {
	if ev.Op != surfaceEventPrefScale {
		return
	}
	if s := int(ev.Int()); s >= 1 && s != w.scale {
		w.scale = s
		w.conn.Send(w.surface, surfaceSetBufferScale, int32(s))
		w.dropBuffers()
		if w.onScale != nil {
			w.onScale()
		}
	}
}

func (w *window) toplevelEvent(ev *wayland.Event) {
	switch ev.Op {
	case toplevelEventConfigure:
		w.pendingW, w.pendingH = int(ev.Int()), int(ev.Int())
	case toplevelEventClose:
		w.closeRequested = true
		if w.onClose != nil {
			w.onClose()
		}
	}
}

func (w *window) xdgSurfaceEvent(ev *wayland.Event) {
	if ev.Op != xdgSurfaceEventConfig {
		return
	}
	w.conn.Send(w.xdgSurface, xdgSurfaceAckConfigure, ev.Uint())
	w.width, w.height = w.pendingW, w.pendingH
	w.configured = true
	if w.onConfigure != nil {
		w.onConfigure()
	}
}

// bufferFor returns a free buffer of the given pixel size, allocating one if
// all are in use; nil means wait for the compositor to release one.
func (w *window) bufferFor(pw, ph int) *buffer {
	live := 0
	for _, b := range w.buffers {
		if b.stale {
			continue
		}
		if b.c.w != pw || b.c.h != ph {
			w.retire(b)
			continue
		}
		live++
		if !b.busy {
			return b
		}
	}
	w.prune()
	if live >= maxBuffers {
		return nil
	}
	b, err := w.newBuffer(pw, ph)
	if err != nil {
		return nil
	}
	w.buffers = append(w.buffers, b)
	return b
}

func (w *window) newBuffer(pw, ph int) (*buffer, error) {
	stride := pw * 4
	mem, err := wayland.NewSharedMemory(stride * ph)
	if err != nil {
		return nil, err
	}
	b := &buffer{mem: mem, c: canvas{pix: mem.Data, w: pw, h: ph, stride: stride}}
	pool := w.conn.NewObject(nil)
	w.conn.Send(w.shm, shmCreatePool, pool, wayland.FD(mem.File.Fd()), int32(stride*ph))
	b.id = w.conn.NewObject(func(ev *wayland.Event) {
		if ev.Op != bufferEventRelease {
			return
		}
		b.busy = false
		if b.stale {
			w.destroy(b)
		} else if w.onBufferRelease != nil {
			w.onBufferRelease()
		}
	})
	w.conn.Send(pool, shmPoolCreateBuffer, b.id, int32(0), int32(pw), int32(ph), int32(stride), w.format)
	w.conn.Send(pool, shmPoolDestroy)
	return b, nil
}

// retire marks b for destruction once the compositor is done with it.
func (w *window) retire(b *buffer) {
	b.stale = true
	if !b.busy {
		w.destroy(b)
	}
}

func (w *window) destroy(b *buffer) {
	if b.mem == nil {
		return
	}
	w.conn.Send(b.id, bufferDestroy)
	b.mem.Close()
	b.mem = nil
}

func (w *window) prune() {
	kept := w.buffers[:0]
	for _, b := range w.buffers {
		if b.mem != nil {
			kept = append(kept, b)
		}
	}
	w.buffers = kept
}

func (w *window) dropBuffers() {
	for _, b := range w.buffers {
		w.retire(b)
	}
	w.prune()
}

// present attaches b with damage in buffer pixels and asks for a frame
// callback.
func (w *window) present(b *buffer, damage []image.Rectangle) {
	w.conn.Send(w.surface, surfaceAttach, b.id, int32(0), int32(0))
	for _, d := range damage {
		if w.compositorVer >= 4 {
			w.conn.Send(w.surface, surfaceDamageBuffer, int32(d.Min.X), int32(d.Min.Y), int32(d.Dx()), int32(d.Dy()))
		} else {
			s := w.scale
			w.conn.Send(w.surface, surfaceDamage, int32(d.Min.X/s), int32(d.Min.Y/s), int32((d.Dx()+s-1)/s), int32((d.Dy()+s-1)/s))
		}
	}
	w.frameCB = w.conn.NewObject(func(ev *wayland.Event) {
		if ev.Op == callbackEventDone {
			w.frameCB = 0
			if w.onFrame != nil {
				w.onFrame()
			}
		}
	})
	w.conn.Send(w.surface, surfaceFrame, w.frameCB)
	w.conn.Send(w.surface, surfaceCommit)
	b.busy = true
}

func (w *window) commit() { w.conn.Send(w.surface, surfaceCommit) }
