package daemon

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
)

type stubWeb struct{ revoked []string }

func (s *stubWeb) WebDevices(context.Context) ([]api.WebDevice, error) {
	return []api.WebDevice{{ID: "d1", Name: "phone"}}, nil
}
func (s *stubWeb) WebRevoke(_ context.Context, id string) error {
	s.revoked = append(s.revoked, id)
	return nil
}
func (s *stubWeb) WebRename(context.Context, string, string) error        { return nil }
func (s *stubWeb) WebSessions(context.Context) ([]api.WebSession, error)  { return nil, nil }
func (s *stubWeb) WebKillSession(context.Context, string) error           { return nil }
func (s *stubWeb) WebEvents(context.Context, int) ([]api.WebEvent, error) { return nil, nil }
func (s *stubWeb) WebNotices(context.Context) ([]api.WebNotice, error)    { return nil, nil }
func (s *stubWeb) WebDismissNotice(context.Context, string) error         { return nil }

func TestWebEndpointsReportDisabledWithoutWebUI(t *testing.T) {
	h := newHarness(t)
	_, c, stop := h.run(t, 0)
	defer stop()
	var se *api.StatusError
	_, err := c.WebDevices(context.Background())
	if !errors.As(err, &se) || se.Code != http.StatusServiceUnavailable || !strings.Contains(se.Message, "web UI not enabled") {
		t.Fatalf("err = %v", err)
	}
}

func TestWebEndpointsUseTheInstalledAdmin(t *testing.T) {
	h := newHarness(t)
	d, err := New(Options{Config: h.config(t, 0), Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubWeb{}
	d.SetWebAdmin(stub)
	errc := make(chan error, 1)
	go func() { errc <- d.Run(context.Background()) }()
	c, err := client.Dial(d.cfg.Socket)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if ds, err := c.WebDevices(context.Background()); err != nil || len(ds) != 1 || ds[0].Name != "phone" {
		t.Fatalf("devices %+v %v", ds, err)
	}
	if err := c.WebRevoke(context.Background(), "d1"); err != nil || len(stub.revoked) != 1 {
		t.Fatalf("revoke %v %v", err, stub.revoked)
	}
	_ = c.Stop(context.Background())
	select {
	case <-errc:
	case <-time.After(15 * time.Second):
		t.Fatal("no shutdown")
	}
}
