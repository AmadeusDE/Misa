package client

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type webBackend struct {
	*fakeBackend
	calls []string
	fail  error
}

func (w *webBackend) WebDevices(context.Context) ([]api.WebDevice, error) {
	return []api.WebDevice{{ID: "d1", Name: "phone", Revoked: true}}, w.fail
}
func (w *webBackend) WebSessions(context.Context) ([]api.WebSession, error) {
	return []api.WebSession{{ID: "abcdef123456", DeviceName: "phone"}}, nil
}
func (w *webBackend) WebEvents(_ context.Context, limit int) ([]api.WebEvent, error) {
	return []api.WebEvent{{Kind: "limit", Detail: string(rune('0' + limit))}}, nil
}
func (w *webBackend) WebNotices(context.Context) ([]api.WebNotice, error) {
	return []api.WebNotice{{ID: "n1", Text: "hi"}}, nil
}
func (w *webBackend) WebRevoke(_ context.Context, id string) error {
	w.calls = append(w.calls, "revoke "+id)
	return w.fail
}
func (w *webBackend) WebRename(_ context.Context, id, name string) error {
	w.calls = append(w.calls, "rename "+id+" "+name)
	return nil
}
func (w *webBackend) WebKillSession(_ context.Context, p string) error {
	w.calls = append(w.calls, "kill "+p)
	return nil
}
func (w *webBackend) WebDismissNotice(_ context.Context, id string) error {
	w.calls = append(w.calls, "dismiss "+id)
	return nil
}

func startBackend(t *testing.T, b api.Backend) *Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "u.sock")
	ln, err := api.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go api.NewServer(b).Serve(ctx, ln)
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); c.Close() })
	return c
}

func TestWebAdminEndpointsRoundTrip(t *testing.T) {
	wb := &webBackend{fakeBackend: &fakeBackend{live: make(chan api.Event)}}
	c := startBackend(t, wb)
	ctx := context.Background()
	if ds, err := c.WebDevices(ctx); err != nil || len(ds) != 1 || ds[0].Name != "phone" || !ds[0].Revoked {
		t.Fatalf("devices %+v %v", ds, err)
	}
	if ss, err := c.WebSessions(ctx); err != nil || len(ss) != 1 || ss[0].ID != "abcdef123456" {
		t.Fatalf("sessions %+v %v", ss, err)
	}
	if es, err := c.WebEvents(ctx, 7); err != nil || len(es) != 1 || es[0].Detail != "7" {
		t.Fatalf("events %+v %v", es, err)
	}
	if ns, err := c.WebNotices(ctx); err != nil || len(ns) != 1 || ns[0].ID != "n1" {
		t.Fatalf("notices %+v %v", ns, err)
	}
	for _, err := range []error{
		c.WebRevoke(ctx, "d1"), c.WebRename(ctx, "d1", "tablet"), c.WebKillSession(ctx, "abcdef"), c.WebDismissNotice(ctx, "n1"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := "revoke d1,rename d1 tablet,kill abcdef,dismiss n1"
	if got := strings.Join(wb.calls, ","); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
}

func TestWebAdminErrorsPropagate(t *testing.T) {
	wb := &webBackend{fakeBackend: &fakeBackend{live: make(chan api.Event)}, fail: errors.New("no device \"x\"")}
	c := startBackend(t, wb)
	err := c.WebRevoke(context.Background(), "x")
	var se *api.StatusError
	if !errors.As(err, &se) || se.Code != http.StatusInternalServerError || !strings.Contains(se.Message, "no device") {
		t.Fatalf("err = %v", err)
	}
}

func TestWebAdminDisabledIsClear(t *testing.T) {
	c := startBackend(t, &fakeBackend{live: make(chan api.Event)})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var se *api.StatusError
	if _, err := c.WebDevices(ctx); !errors.As(err, &se) || se.Code != http.StatusServiceUnavailable || !strings.Contains(se.Message, "web UI not enabled") {
		t.Fatalf("list: %v", err)
	}
	if err := c.WebRevoke(ctx, "x"); !errors.As(err, &se) || se.Code != http.StatusServiceUnavailable {
		t.Fatalf("action: %v", err)
	}
	if err := c.WebRevoke(ctx, ""); err == nil {
		t.Fatal("empty id accepted")
	}
}
