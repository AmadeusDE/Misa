package gui

import "github.com/AmadeusDE/Misa/internal/term"

// invalidate drops every rendering cache the live terminal can rebuild: the
// compositor's scratch, the snapshot's retained references, the frame the
// window last showed and the per-buffer frame histories, then the backend's
// buffers. The terminal's graphics store is the source and is untouched, so
// the whole frame is reconstructed from it.
//
// A window that cannot present keeps the redraw pending. Repeated events in
// that state touch the same empty caches, and reattachment presents the one
// frame they coalesced into.
func (a *app) invalidate() {
	a.rend.gfx = compositor{}
	a.snap = term.Snapshot{}
	a.shown = frame{}
	a.frames = nil
	a.shownW, a.shownH = 0, 0
	if a.be != nil {
		a.be.DropBuffers()
	}
	a.requestDraw()
}
