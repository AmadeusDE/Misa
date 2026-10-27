package webhost

import (
	"context"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/webui"
)

// APIAdmin exposes the device store through the daemon's socket API.
func APIAdmin(a *webui.Admin) api.WebAdmin { return apiAdmin{a} }

type apiAdmin struct{ a *webui.Admin }

func (x apiAdmin) WebDevices(context.Context) ([]api.WebDevice, error) {
	ds, err := x.a.Devices()
	out := make([]api.WebDevice, len(ds))
	for i, d := range ds {
		out[i] = api.WebDevice{ID: d.ID, Name: d.Name, Created: d.Created, LastSeen: d.LastSeen, Revoked: d.Revoked}
	}
	return out, err
}

func (x apiAdmin) WebRevoke(_ context.Context, id string) error { return x.a.RevokeDevice(id) }

func (x apiAdmin) WebRename(_ context.Context, id, name string) error {
	return x.a.RenameDevice(id, name)
}

func (x apiAdmin) WebSessions(context.Context) ([]api.WebSession, error) {
	rows, err := x.a.Sessions()
	out := make([]api.WebSession, len(rows))
	for i, r := range rows {
		out[i] = api.WebSession{ID: r.ID, DeviceID: r.DeviceID, DeviceName: r.DeviceName, IP: r.IP, Created: r.Created, Expires: r.Expires}
	}
	return out, err
}

func (x apiAdmin) WebKillSession(_ context.Context, prefix string) error {
	return x.a.KillSession(prefix)
}

func (x apiAdmin) WebEvents(_ context.Context, limit int) ([]api.WebEvent, error) {
	evs, err := x.a.Events(limit)
	out := make([]api.WebEvent, len(evs))
	for i, e := range evs {
		out[i] = api.WebEvent{At: e.At, Kind: e.Kind, DeviceID: e.DeviceID, Name: e.Name, IP: e.IP, Detail: e.Detail}
	}
	return out, err
}

func (x apiAdmin) WebNotices(context.Context) ([]api.WebNotice, error) {
	ns, err := x.a.Notices()
	out := make([]api.WebNotice, len(ns))
	for i, n := range ns {
		out[i] = api.WebNotice{ID: n.ID, Text: n.Text, At: n.At}
	}
	return out, err
}

func (x apiAdmin) WebDismissNotice(_ context.Context, id string) error {
	return x.a.DismissNotice(id)
}
