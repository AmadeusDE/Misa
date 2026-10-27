package webui

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	srv    *Server
	ts     *httptest.Server
	clock  *fakeClock
	admin  *Admin
	secret string
	store  *FileStore
}

func newEnv(t *testing.T, mod func(*Options)) *env {
	t.Helper()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clock := &fakeClock{t: time.Now().UTC()}
	o := Options{Store: store, Title: "Test", ListenAddr: "127.0.0.1:0", Now: clock.Now, Logf: func(string, ...any) {}}
	if mod != nil {
		mod(&o)
	}
	srv, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	srv.Mount("/echo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok " + r.Method))
	}))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	admin := NewAdmin(store, clock.Now)
	secret, _, err := admin.TOTPInit("Test", "me")
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Minute)
	return &env{srv: srv, ts: ts, clock: clock, admin: admin, secret: secret, store: store}
}

func (e *env) code(t *testing.T) string {
	t.Helper()
	c, err := TOTPCode(e.secret, e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (e *env) client(t *testing.T) *Client {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, err := NewClient(e.ts.URL, priv)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (e *env) enrolled(t *testing.T, name string) *Client {
	t.Helper()
	c := e.client(t)
	if err := c.Enroll(t.Context(), e.code(t), name); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(time.Duration(totpPeriod) * time.Second)
	return c
}

func readAll(res *http.Response) string {
	defer res.Body.Close()
	var sb strings.Builder
	_, _ = io.Copy(&sb, res.Body)
	return sb.String()
}
