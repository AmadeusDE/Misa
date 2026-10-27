package tui

import (
	"context"
	"errors"
	"net/http"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/webdevtui"
)

var errNoDeviceAPI = errors.New("this backend has no device administration")

// devicesScreen hosts the shared device component over the daemon API.
type devicesScreen struct {
	model *webdevtui.Model
}

func newDevicesScreen(ctx context.Context, be Backend, w, h int) *devicesScreen {
	var admin webdevtui.Admin = unavailable{}
	if db, ok := be.(DeviceBackend); ok {
		admin = deviceAdapter{ctx: ctx, be: db}
	}
	m := webdevtui.New(admin)
	m.Resize(w, h)
	return &devicesScreen{model: m}
}

// webDisabled reports the daemon's "web UI not enabled" answer.
func (d *devicesScreen) webDisabled() bool {
	var se *api.StatusError
	return errors.As(d.model.Err(), &se) && se.Code == http.StatusServiceUnavailable
}

func (d *devicesScreen) view() string {
	switch {
	case d.webDisabled():
		return titleStyle.Render("Web devices") + "\n\n" + warnStyle.Render("The web UI is not enabled on this daemon.") + "\n\n" +
			"Devices, sessions and the web event log exist only when the daemon serves the web UI. " +
			"Start it with `ughbot serve -web`, or set `enabled = true` under [web] in the config, " +
			"then reopen this screen. Setup and enrollment steps are in docs/ughbot/WEB.md."
	case errors.Is(d.model.Err(), errNoDeviceAPI):
		return titleStyle.Render("Web devices") + "\n\n" + errStyle.Render("This connection cannot manage devices.")
	}
	return d.model.View()
}

func (d *devicesScreen) hint() string {
	if d.webDisabled() || errors.Is(d.model.Err(), errNoDeviceAPI) {
		return ""
	}
	return d.model.Hint()
}

func (d *devicesScreen) update(msg tea.Msg) tea.Cmd { return d.model.Update(msg) }

type unavailable struct{}

func (unavailable) Devices() ([]webdevtui.Device, error)   { return nil, errNoDeviceAPI }
func (unavailable) Revoke(string) error                    { return errNoDeviceAPI }
func (unavailable) Rename(string, string) error            { return errNoDeviceAPI }
func (unavailable) Sessions() ([]webdevtui.Session, error) { return nil, errNoDeviceAPI }
func (unavailable) KillSession(string) error               { return errNoDeviceAPI }
func (unavailable) Events(int) ([]webdevtui.Event, error)  { return nil, errNoDeviceAPI }
func (unavailable) Notices() ([]webdevtui.Notice, error)   { return nil, errNoDeviceAPI }
func (unavailable) DismissNotice(string) error             { return errNoDeviceAPI }

type deviceAdapter struct {
	ctx context.Context
	be  DeviceBackend
}

func (a deviceAdapter) Revoke(id string) error          { return a.be.WebRevoke(a.ctx, id) }
func (a deviceAdapter) Rename(id, name string) error    { return a.be.WebRename(a.ctx, id, name) }
func (a deviceAdapter) KillSession(prefix string) error { return a.be.WebKillSession(a.ctx, prefix) }
func (a deviceAdapter) DismissNotice(id string) error   { return a.be.WebDismissNotice(a.ctx, id) }

func (a deviceAdapter) Devices() ([]webdevtui.Device, error) {
	rows, err := a.be.WebDevices(a.ctx)
	out := make([]webdevtui.Device, len(rows))
	for i, r := range rows {
		out[i] = webdevtui.Device{ID: r.ID, Name: r.Name, Created: r.Created, LastSeen: r.LastSeen, Revoked: r.Revoked}
	}
	return out, err
}

func (a deviceAdapter) Sessions() ([]webdevtui.Session, error) {
	rows, err := a.be.WebSessions(a.ctx)
	out := make([]webdevtui.Session, len(rows))
	for i, r := range rows {
		out[i] = webdevtui.Session{ID: r.ID, DeviceID: r.DeviceID, DeviceName: r.DeviceName, IP: r.IP, Created: r.Created, Expires: r.Expires}
	}
	return out, err
}

func (a deviceAdapter) Events(limit int) ([]webdevtui.Event, error) {
	rows, err := a.be.WebEvents(a.ctx, limit)
	out := make([]webdevtui.Event, len(rows))
	for i, r := range rows {
		out[i] = webdevtui.Event{At: r.At, Kind: r.Kind, DeviceID: r.DeviceID, Name: r.Name, IP: r.IP, Detail: r.Detail}
	}
	return out, err
}

func (a deviceAdapter) Notices() ([]webdevtui.Notice, error) {
	rows, err := a.be.WebNotices(a.ctx)
	out := make([]webdevtui.Notice, len(rows))
	for i, r := range rows {
		out[i] = webdevtui.Notice{ID: r.ID, Text: r.Text, At: r.At}
	}
	return out, err
}
