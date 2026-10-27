package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type fakeBackend struct {
	mu        sync.Mutex
	sent      []string
	stopped   bool
	history   []api.Event
	live      chan api.Event
	lastAfter uint64
}

func (f *fakeBackend) ListBots(context.Context) ([]api.BotInfo, error) {
	return []api.BotInfo{{Name: "alpha", State: "idle"}}, nil
}
func (f *fakeBackend) Status(context.Context) (api.Status, error) {
	return api.Status{Version: "t", Bots: []api.BotInfo{{Name: "alpha"}}}, nil
}
func (f *fakeBackend) Cost(_ context.Context, q api.CostQuery) ([]api.CostRow, error) {
	return []api.CostRow{{Bot: q.Bot, Day: q.From, USD: 1.5}}, nil
}
func (f *fakeBackend) Send(_ context.Context, bot, text string) (string, error) {
	if bot != "alpha" {
		return "", api.ErrUnknownBot
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return "t-1", nil
}
func (f *fakeBackend) Stop(context.Context) error {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
	return nil
}
func (f *fakeBackend) Subscribe(ctx context.Context, bot string, after uint64) (<-chan api.Event, error) {
	if bot != "alpha" {
		return nil, api.ErrUnknownBot
	}
	f.mu.Lock()
	f.lastAfter = after
	var replay []api.Event
	for _, e := range f.history {
		if e.Seq > after {
			replay = append(replay, e)
		}
	}
	f.mu.Unlock()
	out := make(chan api.Event)
	go func() {
		defer close(out)
		for _, e := range replay {
			select {
			case out <- e:
			case <-ctx.Done():
				return
			}
		}
		for {
			select {
			case e := <-f.live:
				select {
				case out <- e:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

type harness struct {
	fb     *fakeBackend
	srv    *api.Server
	c      *Client
	cancel context.CancelFunc
	done   chan error
}

func start(t *testing.T) *harness {
	t.Helper()
	fb := &fakeBackend{live: make(chan api.Event, 16)}
	for i := uint64(1); i <= 3; i++ {
		fb.history = append(fb.history, api.Event{Seq: i, Type: "text", Data: json.RawMessage(`"m"`)})
	}
	path := filepath.Join(t.TempDir(), "u.sock")
	ln, err := api.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := api.NewServer(fb)
	srv.Heartbeat = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{fb: fb, srv: srv, c: c, cancel: cancel, done: done}
	t.Cleanup(func() { cancel(); c.Close() })
	return h
}

func recv(t *testing.T, ch <-chan api.Event) api.Event {
	t.Helper()
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatal("stream closed")
		}
		return e
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for event")
	}
	return api.Event{}
}

func TestDialFailsWithoutDaemon(t *testing.T) {
	if _, err := Dial(filepath.Join(t.TempDir(), "none.sock")); err == nil {
		t.Fatal("expected error")
	}
}

func TestEndpoints(t *testing.T) {
	h := start(t)
	ctx := context.Background()
	bots, err := h.c.ListBots(ctx)
	if err != nil || len(bots) != 1 || bots[0].Name != "alpha" {
		t.Fatalf("bots %v %v", bots, err)
	}
	st, err := h.c.Status(ctx)
	if err != nil || st.Version != "t" {
		t.Fatalf("status %v %v", st, err)
	}
	rows, err := h.c.Cost(ctx, api.CostQuery{Bot: "alpha", From: "2026-10-01"})
	if err != nil || len(rows) != 1 || rows[0].Bot != "alpha" || rows[0].Day != "2026-10-01" || rows[0].USD != 1.5 {
		t.Fatalf("cost %v %v", rows, err)
	}
	ticket, err := h.c.Send(ctx, "alpha", "hi")
	if err != nil || ticket != "t-1" || h.fb.sent[0] != "hi" {
		t.Fatalf("send %q %v", ticket, err)
	}
	if err := h.c.Stop(ctx); err != nil || !h.fb.stopped {
		t.Fatalf("stop %v", err)
	}
}

func TestErrors(t *testing.T) {
	h := start(t)
	ctx := context.Background()
	var se *api.StatusError
	if _, err := h.c.Send(ctx, "nobody", "x"); !errors.As(err, &se) || se.Code != http.StatusNotFound {
		t.Fatalf("unknown bot: %v", err)
	}
	if _, err := h.c.Send(ctx, "alpha", ""); !errors.As(err, &se) || se.Code != http.StatusBadRequest {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := h.c.Subscribe(ctx, "nobody", 0); !errors.As(err, &se) || se.Code != http.StatusNotFound {
		t.Fatalf("subscribe unknown: %v", err)
	}
}

func TestSubscribeReplayLiveAndResume(t *testing.T) {
	h := start(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := h.c.Subscribe(ctx, "alpha", 0)
	if err != nil {
		t.Fatal(err)
	}
	for want := uint64(1); want <= 3; want++ {
		if e := recv(t, ch); e.Seq != want || e.Type != "text" {
			t.Fatalf("got %+v want seq %d", e, want)
		}
	}
	h.fb.live <- api.Event{Seq: 4, Type: "done"}
	if e := recv(t, ch); e.Seq != 4 || e.Type != "done" {
		t.Fatalf("live %+v", e)
	}
	cancel()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	ch2, err := h.c.Subscribe(ctx2, "alpha", 2)
	if err != nil {
		t.Fatal(err)
	}
	if e := recv(t, ch2); e.Seq != 3 {
		t.Fatalf("resume got %+v", e)
	}
	h.fb.mu.Lock()
	got := h.fb.lastAfter
	h.fb.mu.Unlock()
	if got != 2 {
		t.Fatalf("backend saw after=%d", got)
	}
}

func TestQueryAfterFallback(t *testing.T) {
	h := start(t)
	resp, err := h.c.http.Get("http://ughbot/v1/bots/alpha/events?after=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	h.fb.mu.Lock()
	defer h.fb.mu.Unlock()
	if h.fb.lastAfter != 1 {
		t.Fatalf("after=%d", h.fb.lastAfter)
	}
}

func TestSlowSubscriberDoesNotBlockServer(t *testing.T) {
	h := start(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := h.c.Subscribe(ctx, "alpha", 0); err != nil {
		t.Fatal(err)
	}
	for i := uint64(4); i < 14; i++ {
		h.fb.live <- api.Event{Seq: i, Type: "text"}
	}
	rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rcancel()
	if _, err := h.c.ListBots(rctx); err != nil {
		t.Fatalf("server blocked by idle subscriber: %v", err)
	}
	if _, err := h.c.Send(rctx, "alpha", "still works"); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownClosesStreams(t *testing.T) {
	h := start(t)
	ch, err := h.c.Subscribe(context.Background(), "alpha", 3)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return")
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unexpected event")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream not closed")
	}
}

func TestHeartbeatKeepsStreamOpen(t *testing.T) {
	h := start(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, _ := h.c.Subscribe(ctx, "alpha", 3)
	time.Sleep(100 * time.Millisecond)
	h.fb.live <- api.Event{Seq: 4, Type: "text"}
	if e := recv(t, ch); e.Seq != 4 {
		t.Fatalf("%+v", e)
	}
}
