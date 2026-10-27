package webadmin

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/webdevtui"
	"github.com/AmadeusDE/Misa/internal/webui"
)

// TUI adapts the store-backed admin to the device management screen.
func TUI(a *webui.Admin) webdevtui.Admin { return tuiAdmin{a} }

// RunTUI runs the device management screen full-screen on the terminal.
func RunTUI(a *webui.Admin) error {
	_, err := tea.NewProgram(webdevtui.NewProgram(TUI(a))).Run()
	return err
}

type tuiAdmin struct{ a *webui.Admin }

func (t tuiAdmin) Devices() ([]webdevtui.Device, error) {
	ds, err := t.a.Devices()
	out := make([]webdevtui.Device, len(ds))
	for i, d := range ds {
		out[i] = webdevtui.Device{ID: d.ID, Name: d.Name, Created: d.Created, LastSeen: d.LastSeen, Revoked: d.Revoked}
	}
	return out, err
}

func (t tuiAdmin) Revoke(id string) error          { return t.a.RevokeDevice(id) }
func (t tuiAdmin) Rename(id, name string) error    { return t.a.RenameDevice(id, name) }
func (t tuiAdmin) KillSession(prefix string) error { return t.a.KillSession(prefix) }
func (t tuiAdmin) DismissNotice(id string) error   { return t.a.DismissNotice(id) }

func (t tuiAdmin) Sessions() ([]webdevtui.Session, error) {
	rows, err := t.a.Sessions()
	out := make([]webdevtui.Session, len(rows))
	for i, r := range rows {
		out[i] = webdevtui.Session{ID: r.ID, DeviceID: r.DeviceID, DeviceName: r.DeviceName, IP: r.IP, Created: r.Created, Expires: r.Expires}
	}
	return out, err
}

func (t tuiAdmin) Events(limit int) ([]webdevtui.Event, error) {
	evs, err := t.a.Events(limit)
	out := make([]webdevtui.Event, len(evs))
	for i, e := range evs {
		out[i] = webdevtui.Event{At: e.At, Kind: e.Kind, DeviceID: e.DeviceID, Name: e.Name, IP: e.IP, Detail: e.Detail}
	}
	return out, err
}

func (t tuiAdmin) Notices() ([]webdevtui.Notice, error) {
	ns, err := t.a.Notices()
	out := make([]webdevtui.Notice, len(ns))
	for i, n := range ns {
		out[i] = webdevtui.Notice{ID: n.ID, Text: n.Text, At: n.At}
	}
	return out, err
}
