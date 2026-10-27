package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ErrWebDisabled is returned by the web admin endpoints when the daemon runs without the web UI.
var ErrWebDisabled = errors.New("web UI not enabled: start the daemon with `ughbot serve -web` or set [web] enabled = true")

type WebDevice struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"last_seen"`
	Revoked  bool      `json:"revoked"`
}

type WebSession struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	IP         string    `json:"ip"`
	Created    time.Time `json:"created"`
	Expires    time.Time `json:"expires"`
}

type WebEvent struct {
	At       time.Time `json:"at"`
	Kind     string    `json:"kind"`
	DeviceID string    `json:"device_id,omitempty"`
	Name     string    `json:"name,omitempty"`
	IP       string    `json:"ip,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

type WebNotice struct {
	ID   string    `json:"id"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// WebTarget names a device, session prefix or notice; NewName is used by rename.
type WebTarget struct {
	ID      string `json:"id"`
	NewName string `json:"new_name,omitempty"`
}

// WebAdmin is the device management surface over the unix socket. A Backend
// that also implements it gets the /v1/web/* endpoints.
type WebAdmin interface {
	WebDevices(ctx context.Context) ([]WebDevice, error)
	WebRevoke(ctx context.Context, idOrName string) error
	WebRename(ctx context.Context, idOrName, newName string) error
	WebSessions(ctx context.Context) ([]WebSession, error)
	WebKillSession(ctx context.Context, prefix string) error
	WebEvents(ctx context.Context, limit int) ([]WebEvent, error)
	WebNotices(ctx context.Context) ([]WebNotice, error)
	WebDismissNotice(ctx context.Context, id string) error
}

func (s *Server) registerWebAdmin(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/web/devices", s.web(func(ctx context.Context, wa WebAdmin, r *http.Request) (any, error) { return wa.WebDevices(ctx) }))
	mux.HandleFunc("GET /v1/web/sessions", s.web(func(ctx context.Context, wa WebAdmin, r *http.Request) (any, error) { return wa.WebSessions(ctx) }))
	mux.HandleFunc("GET /v1/web/notices", s.web(func(ctx context.Context, wa WebAdmin, r *http.Request) (any, error) { return wa.WebNotices(ctx) }))
	mux.HandleFunc("GET /v1/web/events", s.web(func(ctx context.Context, wa WebAdmin, r *http.Request) (any, error) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		return wa.WebEvents(ctx, limit)
	}))
	mux.HandleFunc("POST /v1/web/devices/revoke", s.webAction(func(ctx context.Context, wa WebAdmin, t WebTarget) error { return wa.WebRevoke(ctx, t.ID) }))
	mux.HandleFunc("POST /v1/web/devices/rename", s.webAction(func(ctx context.Context, wa WebAdmin, t WebTarget) error { return wa.WebRename(ctx, t.ID, t.NewName) }))
	mux.HandleFunc("POST /v1/web/sessions/kill", s.webAction(func(ctx context.Context, wa WebAdmin, t WebTarget) error { return wa.WebKillSession(ctx, t.ID) }))
	mux.HandleFunc("POST /v1/web/notices/dismiss", s.webAction(func(ctx context.Context, wa WebAdmin, t WebTarget) error { return wa.WebDismissNotice(ctx, t.ID) }))
}

func (s *Server) webAdmin() (WebAdmin, error) {
	if s.Web != nil {
		return s.Web, nil
	}
	if wa, ok := s.backend.(WebAdmin); ok {
		return wa, nil
	}
	return nil, ErrWebDisabled
}

func (s *Server) web(list func(context.Context, WebAdmin, *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wa, err := s.webAdmin()
		var rows any
		if err == nil {
			rows, err = list(r.Context(), wa, r)
		}
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func (s *Server) webAction(act func(context.Context, WebAdmin, WebTarget) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wa, err := s.webAdmin()
		if err != nil {
			writeError(w, err)
			return
		}
		var t WebTarget
		if !decodeBody(w, r, &t) {
			return
		}
		if t.ID == "" {
			writeJSON(w, http.StatusBadRequest, ErrorBody{Error: "id is required"})
			return
		}
		if err := act(r.Context(), wa, t); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
