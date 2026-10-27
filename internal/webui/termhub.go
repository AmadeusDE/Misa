package webui

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmadeusDE/Misa/internal/term"
)

const (
	defaultTerminalGrace = 60 * time.Second
	defaultMaxTerminals  = 8
	defaultAuthCheck     = 2 * time.Second
	defaultFrameEvery    = 33 * time.Millisecond
	defaultRows          = 24
	defaultCols          = 80
	maxTitle             = 256
)

// SpawnFunc starts the program a browser terminal shows. ctx ends when the
// terminal is closed; the returned terminal must already be reading (StartReader).
type SpawnFunc func(ctx context.Context, rows, cols int, who SessionInfo) (*term.Terminal, error)

type terminalHub struct {
	srv         *Server
	spawn       SpawnFunc
	grace       time.Duration
	authCheck   time.Duration
	frameEvery  time.Duration
	mu          sync.Mutex
	sessions    map[string]*termSession
	activeCount *atomic.Int32
}

func (h *terminalHub) closeAll() {
	h.mu.Lock()
	all := make([]*termSession, 0, len(h.sessions))
	for _, ts := range h.sessions {
		all = append(all, ts)
	}
	h.mu.Unlock()
	for _, ts := range all {
		ts.close("server shutdown")
	}
}

func (h *terminalHub) lookup(deviceID string) *termSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[deviceID]
}

// session returns the device's terminal, starting one sized rows x cols when
// it has none. created reports a fresh start.
func (h *terminalHub) session(r *http.Request, who SessionInfo, rows, cols int) (ts *termSession, status int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ts = h.sessions[who.DeviceID]; ts != nil {
		return ts, http.StatusOK
	}
	if int(h.activeCount.Load()) >= h.srv.maxTerminals() {
		return nil, http.StatusServiceUnavailable
	}
	ctx, cancel := context.WithCancel(context.Background())
	t, err := h.spawn(ctx, rows, cols, who)
	if err != nil || t == nil {
		cancel()
		h.srv.opts.Logf("webui: terminal spawn for %s: %v", who.DeviceName, err)
		return nil, http.StatusInternalServerError
	}
	ts = &termSession{hub: h, key: who.DeviceID, t: t, cancel: cancel, nextSeq: 1,
		done: make(chan struct{}), wake: make(chan struct{}, 1), authReq: cookieOnly(r)}
	t.SetOSCHook(ts.oscTitle)
	h.sessions[who.DeviceID] = ts
	h.activeCount.Add(1)
	ts.startGrace()
	go ts.watchAuth()
	return ts, http.StatusCreated
}

func cookieOnly(r *http.Request) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	if c, err := r.Cookie(cookieName); err == nil {
		req.AddCookie(c)
	}
	return req
}

type attachment struct{ kicked chan struct{} }

type termSession struct {
	hub     *terminalHub
	key     string
	t       *term.Terminal
	cancel  context.CancelFunc
	authReq *http.Request // guarded by mu
	done    chan struct{}
	wake    chan struct{}

	inputMu  sync.Mutex
	mu       sync.Mutex
	nextSeq  uint64
	attached *attachment
	graceGen int
	closed   bool
	reason   string
	title    string
}

func (ts *termSession) oscTitle(osc string) {
	kind, text, ok := strings.Cut(osc, ";")
	if !ok || kind != "0" && kind != "2" {
		return
	}
	if len(text) > maxTitle {
		text = text[:maxTitle]
	}
	ts.mu.Lock()
	ts.title = strings.ToValidUTF8(text, "")
	ts.mu.Unlock()
}

func (ts *termSession) currentTitle() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.title
}

func (ts *termSession) closeReason() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.reason
}

// attach makes the caller the one live stream, replacing any other. Its
// login becomes the one the terminal lives and dies with.
func (ts *termSession) attach(r *http.Request) *attachment {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return nil
	}
	ts.authReq = cookieOnly(r)
	if ts.attached != nil {
		close(ts.attached.kicked)
	}
	a := &attachment{kicked: make(chan struct{})}
	ts.attached = a
	ts.graceGen++
	return a
}

func (ts *termSession) detach(a *attachment) {
	ts.mu.Lock()
	if ts.attached != a {
		ts.mu.Unlock()
		return
	}
	ts.attached = nil
	ts.mu.Unlock()
	ts.startGrace()
}

// startGrace closes the terminal unless a stream attaches within the grace period.
func (ts *termSession) startGrace() {
	ts.mu.Lock()
	ts.graceGen++
	gen := ts.graceGen
	ts.mu.Unlock()
	time.AfterFunc(ts.hub.grace, func() {
		ts.mu.Lock()
		expired := ts.graceGen == gen && ts.attached == nil
		ts.mu.Unlock()
		if expired {
			ts.close("no client reconnected")
		}
	})
}

// watchAuth closes the terminal once the login that last attached a stream
// is gone: logout, expiry, or a revoked device.
func (ts *termSession) watchAuth() {
	tick := time.NewTicker(ts.hub.authCheck)
	defer tick.Stop()
	for {
		select {
		case <-ts.done:
			return
		case <-tick.C:
			ts.mu.Lock()
			req := ts.authReq
			ts.mu.Unlock()
			if _, ok := ts.hub.srv.authenticate(req); !ok {
				ts.close("signed out")
				return
			}
		}
	}
}

func (ts *termSession) close(reason string) {
	ts.mu.Lock()
	if ts.closed {
		ts.mu.Unlock()
		return
	}
	ts.closed, ts.reason = true, reason
	ts.mu.Unlock()
	close(ts.done)
	ts.cancel()
	ts.t.Close()
	h := ts.hub
	h.mu.Lock()
	if h.sessions[ts.key] == ts {
		delete(h.sessions, ts.key)
	}
	h.mu.Unlock()
	h.activeCount.Add(-1)
}

type applyResult int

const (
	applied applyResult = iota
	duplicate
	outOfOrder
	gone
)

// apply runs a batch if it is the next one in sequence. A batch already
// applied (a retry) is acknowledged without running again; one from the
// future is refused so the client resends from next. Batches run one at a
// time under inputMu; ts.mu is never held while calling into the terminal,
// whose reader takes ts.mu from the title hook.
func (ts *termSession) apply(b inputBatch) (res applyResult, next uint64) {
	ts.inputMu.Lock()
	defer ts.inputMu.Unlock()
	if res, next := ts.checkSeq(b.Seq); res != applied {
		return res, next
	}
	for _, ev := range b.Events {
		applyEvent(ts.t, ev)
	}
	ts.mu.Lock()
	ts.nextSeq++
	next = ts.nextSeq
	ts.mu.Unlock()
	select {
	case ts.wake <- struct{}{}:
	default:
	}
	return applied, next
}

func (ts *termSession) checkSeq(seq uint64) (applyResult, uint64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	switch {
	case ts.closed:
		return gone, ts.nextSeq
	case seq < ts.nextSeq:
		return duplicate, ts.nextSeq
	case seq > ts.nextSeq:
		return outOfOrder, ts.nextSeq
	}
	return applied, ts.nextSeq
}

func (ts *termSession) expectedSeq() uint64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.nextSeq
}
