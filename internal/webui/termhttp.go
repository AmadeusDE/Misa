package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/term"
)

const (
	maxInputBody   = 4 << 20
	keepAliveEvery = 15 * time.Second
)

var terminalTmpl = parsePage("terminal.html")

// MountTerminal serves a browser terminal under prefix (default "/term/"),
// login required: the page at prefix, the SSE screen stream at prefix+"stream"
// and ordered input at prefix+"input". Each device gets one terminal started
// by spawn; it outlives a dropped stream for a grace period and ends with the
// program, the login, the device or the server.
func (s *Server) MountTerminal(prefix string, spawn SpawnFunc) {
	if prefix == "" {
		prefix = "/term/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	grace := s.opts.TerminalGrace
	if grace <= 0 {
		grace = defaultTerminalGrace
	}
	h := &terminalHub{srv: s, spawn: spawn, grace: grace, authCheck: defaultAuthCheck,
		frameEvery: defaultFrameEvery, sessions: map[string]*termSession{}, activeCount: &s.termActive}
	s.mu.Lock()
	s.terminals = append(s.terminals, h)
	s.mu.Unlock()
	s.Mount("GET "+prefix+"{$}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Page(w, r, "Terminal", terminalTmpl, map[string]string{"Prefix": prefix})
	}))
	s.Mount("GET "+prefix+"stream", http.HandlerFunc(h.handleStream))
	s.Mount("POST "+prefix+"input", http.HandlerFunc(h.handleInput))
	s.AddNav("Terminal", prefix)
}

func (s *Server) maxTerminals() int {
	if s.opts.MaxTerminals > 0 {
		return s.opts.MaxTerminals
	}
	return defaultMaxTerminals
}

// CloseTerminals ends every browser terminal.
func (s *Server) CloseTerminals() {
	s.mu.Lock()
	hubs := append([]*terminalHub(nil), s.terminals...)
	s.mu.Unlock()
	for _, h := range hubs {
		h.closeAll()
	}
}

// SpawnArgv runs argv in dir on a PTY sized rows x cols (TERM=xterm-256color),
// with extraEnv ("K=V") added through env(1), plus WEBUI_DEVICE naming the device that opened it.
func SpawnArgv(argv []string, dir string, extraEnv []string) SpawnFunc {
	return func(ctx context.Context, rows, cols int, who SessionInfo) (*term.Terminal, error) {
		env := extraEnv
		if who.DeviceName != "" {
			env = append(slices.Clip(env), "WEBUI_DEVICE="+who.DeviceName)
		}
		args := argv
		if len(env) > 0 {
			args = append(append([]string{"env"}, env...), argv...)
		}
		t := term.New(rows, cols)
		if err := t.StartArgv(dir, args...); err != nil {
			return nil, err
		}
		t.StartReader()
		go func() {
			<-ctx.Done()
			t.Close()
		}()
		return t, nil
	}
}

func queryInt(r *http.Request, name string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil && n > 0 {
		return n
	}
	return def
}

func (h *terminalHub) handleStream(w http.ResponseWriter, r *http.Request) {
	who, _ := SessionFrom(r)
	rows, cols := clampSize(queryInt(r, "rows", defaultRows), queryInt(r, "cols", defaultCols))
	ts, status := h.session(r, who, rows, cols)
	if ts == nil {
		http.Error(w, http.StatusText(status), status)
		return
	}
	a := ts.attach(r)
	if a == nil {
		http.Error(w, "terminal closed", http.StatusGone)
		return
	}
	defer ts.detach(a)
	if status == http.StatusOK {
		var cur term.Snapshot
		if ts.t.Snapshot(&cur); cur.Rows != rows || cur.Cols != cols {
			ts.t.Resize(rows, cols)
		}
	}
	rc := http.NewResponseController(w)
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	st := &sseStream{w: w, rc: rc, ts: ts, enc: newFrameEncoder(), r: r}
	st.run(a, h.frameEvery)
}

type sseStream struct {
	w    http.ResponseWriter
	rc   *http.ResponseController
	ts   *termSession
	enc  *frameEncoder
	r    *http.Request
	snap term.Snapshot
	id   int
	err  error
}

func (s *sseStream) event(name string, v any) {
	if s.err != nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		s.err = err
		return
	}
	s.id++
	_, s.err = fmt.Fprintf(s.w, "id: %d\nevent: %s\ndata: %s\n\n", s.id, name, data)
	if s.err == nil {
		s.err = s.rc.Flush()
	}
}

func (s *sseStream) keepAlive() {
	if s.err == nil {
		_, s.err = fmt.Fprint(s.w, ": ping\n\n")
		if s.err == nil {
			s.err = s.rc.Flush()
		}
	}
}

func (s *sseStream) capture() (mouse bool, title string) {
	s.ts.t.Snapshot(&s.snap)
	return s.ts.t.Mouse().Tracking != 0, s.ts.currentTitle()
}

func (s *sseStream) sendFull() {
	mouse, title := s.capture()
	s.event("frame", s.enc.full(&s.snap, mouse, title, s.ts.expectedSeq()))
}

func (s *sseStream) sendDiff() {
	mouse, title := s.capture()
	if f, changed := s.enc.diff(&s.snap, mouse, title, s.ts.expectedSeq()); changed {
		s.event("frame", f)
	}
}

func (s *sseStream) run(a *attachment, every time.Duration) {
	t := s.ts.t
	s.sendFull()
	ping := time.NewTicker(keepAliveEvery)
	defer ping.Stop()
	last := time.Now()
	for s.err == nil {
		select {
		case <-s.r.Context().Done():
			return
		case <-a.kicked:
			s.event("replaced", map[string]string{"reason": "opened in another window"})
			return
		case <-s.ts.done:
			s.event("closed", map[string]string{"reason": s.ts.closeReason()})
			return
		case <-t.Exited():
			s.sendDiff()
			s.event("exit", map[string]int{"code": t.ExitCode()})
			s.ts.close("program exited")
			return
		case <-ping.C:
			s.keepAlive()
			continue
		case <-s.ts.wake:
		case <-t.Dirty():
		}
		if wait := every - time.Since(last); wait > 0 {
			select {
			case <-time.After(wait):
			case <-s.r.Context().Done():
				return
			case <-s.ts.done:
			case <-a.kicked:
			}
		}
		last = time.Now()
		s.sendDiff()
	}
}

func (h *terminalHub) handleInput(w http.ResponseWriter, r *http.Request) {
	who, _ := SessionFrom(r)
	ts := h.lookup(who.DeviceID)
	if ts == nil {
		http.Error(w, "no terminal", http.StatusGone)
		return
	}
	var b inputBatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxInputBody)).Decode(&b); err != nil || b.Seq == 0 || len(b.Events) > maxEventsPerBatch {
		http.Error(w, "bad input batch", http.StatusBadRequest)
		return
	}
	res, next := ts.apply(b)
	status := http.StatusOK
	switch res {
	case outOfOrder:
		status = http.StatusConflict
	case gone:
		status = http.StatusGone
	}
	writeJSON(w, status, map[string]any{"next": next, "duplicate": res == duplicate})
}
