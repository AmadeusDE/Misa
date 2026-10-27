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

// clipboard is the Wayland selection: pasting from it and owning it for
// text programs copy with OSC 52.
type clipboard struct {
	manager, device uint32
	offers          map[uint32][]string
	selection       uint32
	source          uint32
	pasted          chan string
}

func (a *app) bindClipboard(reg *wayland.Registry) {
	c := &a.clip
	c.offers = map[uint32][]string{}
	c.pasted = make(chan string, 1)
	c.manager, _ = reg.Bind("wl_data_device_manager", 3, nil)
	if c.manager == 0 || a.seat.id == 0 {
		return
	}
	c.device = a.conn.NewObject(func(ev *wayland.Event) { c.deviceEvent(a.conn, ev) })
	a.conn.Send(c.manager, dataManagerGetDevice, c.device, a.seat.id)
}

func (c *clipboard) deviceEvent(conn *wayland.Conn, ev *wayland.Event) {
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

// paste reads the selection on a goroutine; the text arrives on c.pasted.
func (c *clipboard) paste(a *app) {
	mime, ok := textMime(c.offers[c.selection])
	if c.selection == 0 || !ok {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	a.conn.Send(c.selection, dataOfferReceive, mime, wayland.FD(w.Fd()))
	w.Close()
	go func() {
		defer r.Close()
		data, _ := io.ReadAll(io.LimitReader(r, maxPaste))
		c.pasted <- string(data)
	}()
}

// copy offers text as the selection until another client takes it.
func (c *clipboard) copy(a *app, text string) {
	if c.manager == 0 || c.device == 0 {
		return
	}
	var src uint32
	src = a.conn.NewObject(func(ev *wayland.Event) {
		switch ev.Op {
		case dataSourceEventSend:
			ev.Text()
			if fd := ev.FD(); fd >= 0 {
				go writeAndClose(os.NewFile(uintptr(fd), "selection"), text)
			}
		case dataSourceEventCancelled:
			a.conn.Send(src, dataSourceDestroy)
			if c.source == src {
				c.source = 0
			}
		}
	})
	a.conn.Send(c.manager, dataManagerCreateSource, src)
	for _, m := range textMimes {
		a.conn.Send(src, dataSourceOffer, m)
	}
	a.conn.Send(c.device, dataDeviceSetSelection, src, a.seat.serial)
	c.source = src
}

func writeAndClose(f *os.File, text string) {
	io.WriteString(f, text)
	f.Close()
}
