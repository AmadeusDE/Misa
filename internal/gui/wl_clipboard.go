//go:build unix && !android

package gui

import (
	"io"
	"os"
	"slices"

	"github.com/AmadeusDE/Misa/internal/wayland"
)

const (
	dataManagerCreateSource  = 0
	dataManagerGetDevice     = 1
	dataSourceOffer          = 0
	dataSourceDestroy        = 1
	dataSourceEventSend      = 1
	dataSourceEventCancelled = 2
	dataDeviceSetSelection   = 1
	dataDeviceEventDataOffer = 0
	dataDeviceEventSelection = 5
	dataOfferReceive         = 1
	dataOfferDestroy         = 2
	dataOfferEventOffer      = 0
	maxPaste                 = 16 << 20
)

var textMimes = []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "TEXT", "STRING"}

// wlClipboard is the Wayland selection: pasting from it and owning it for
// text programs copy with OSC 52.
type wlClipboard struct {
	be              *wlBackend
	manager, device uint32
	offers          map[uint32][]string
	selection       uint32
	source          uint32
	pasted          chan string
}

func (b *wlBackend) bindClipboard(reg *wayland.Registry) {
	c := &b.clip
	c.be = b
	c.offers = map[uint32][]string{}
	c.pasted = make(chan string, 1)
	c.manager, _ = reg.Bind("wl_data_device_manager", 3, nil)
	if c.manager == 0 || b.seat.id == 0 {
		return
	}
	c.device = b.conn.NewObject(func(ev *wayland.Event) { c.deviceEvent(b.conn, ev) })
	b.conn.Send(c.manager, dataManagerGetDevice, c.device, b.seat.id)
}

func (c *wlClipboard) deviceEvent(conn *wayland.Conn, ev *wayland.Event) {
	switch ev.Op {
	case dataDeviceEventDataOffer:
		id := ev.Uint()
		c.offers[id] = nil
		conn.SetHandler(id, func(ev *wayland.Event) {
			if ev.Op == dataOfferEventOffer {
				c.offers[id] = append(c.offers[id], ev.Text())
			}
		})
	case dataDeviceEventSelection:
		c.selection = ev.Object()
		for id := range c.offers {
			if id != c.selection {
				conn.Send(id, dataOfferDestroy)
				conn.SetHandler(id, nil)
				delete(c.offers, id)
			}
		}
	}
}

func textMime(offered []string) (string, bool) {
	for _, m := range textMimes {
		if slices.Contains(offered, m) {
			return m, true
		}
	}
	return "", false
}

func (c *wlClipboard) Read() (string, error) {
	c.paste()
	return "", nil
}

func (c *wlClipboard) Write(text string) error {
	c.copy(text)
	return nil
}

func (c *wlClipboard) Pastes() <-chan string { return c.pasted }

// paste reads the selection on a goroutine; the text arrives on Pastes.
func (c *wlClipboard) paste() {
	mime, ok := textMime(c.offers[c.selection])
	if c.selection == 0 || !ok || c.be == nil {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	c.be.conn.Send(c.selection, dataOfferReceive, mime, wayland.FD(w.Fd()))
	w.Close()
	go func() {
		defer r.Close()
		data, _ := io.ReadAll(io.LimitReader(r, maxPaste))
		c.pasted <- string(data)
	}()
}

// copy offers text as the selection until another client takes it.
func (c *wlClipboard) copy(text string) {
	if c.manager == 0 || c.device == 0 || c.be == nil {
		return
	}
	conn := c.be.conn
	var src uint32
	src = conn.NewObject(func(ev *wayland.Event) {
		switch ev.Op {
		case dataSourceEventSend:
			ev.Text()
			if fd := ev.FD(); fd >= 0 {
				go writeAndClose(os.NewFile(uintptr(fd), "selection"), text)
			}
		case dataSourceEventCancelled:
			conn.Send(src, dataSourceDestroy)
			if c.source == src {
				c.source = 0
			}
		}
	})
	conn.Send(c.manager, dataManagerCreateSource, src)
	for _, m := range textMimes {
		conn.Send(src, dataSourceOffer, m)
	}
	conn.Send(c.device, dataDeviceSetSelection, src, c.be.seat.serial)
	c.source = src
}

func writeAndClose(f *os.File, text string) {
	io.WriteString(f, text)
	f.Close()
}
