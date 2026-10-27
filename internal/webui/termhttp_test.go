package webui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type sseEvent struct {
	Name string
	Data string
}

type streamView struct {
	t      *testing.T
	events chan sseEvent
	cancel context.CancelFunc
	lines  []string
	code   int
	exited bool
	seq    uint64
}

func (e *env) mountTerm(t *testing.T, argv ...string) *terminalHub {
	t.Helper()
	e.srv.MountTerminal("/term/", SpawnArgv(argv, t.TempDir(), nil))
	h := e.srv.terminals[len(e.srv.terminals)-1]
	h.grace, h.authCheck = 300*time.Millisecond, 20*time.Millisecond
	t.Cleanup(e.srv.CloseTerminals)
	return h
}

func (e *env) loggedIn(t *testing.T, name string) *Client {
	t.Helper()
	c := e.enrolled(t, name)
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}
	return c
}

func openStream(t *testing.T, e *env, c *Client, query string) (*streamView, *http.Response) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, "GET", e.ts.URL+"/term/stream?"+query, nil)
	res, err := c.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		cancel()
		return nil, res
	}
	v := &streamView{t: t, events: make(chan sseEvent, 256), cancel: cancel}
	go func() {
		defer close(v.events)
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(nil, 1<<22)
		var ev sseEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.Name = line[7:]
			case strings.HasPrefix(line, "data: "):
				ev.Data = line[6:]
			case line == "" && ev.Name != "":
				v.events <- ev
				ev = sseEvent{}
			}
		}
	}()
	t.Cleanup(func() { cancel(); res.Body.Close() })
	return v, res
}

func (v *streamView) apply(ev sseEvent) {
	switch ev.Name {
	case "exit":
		var x struct{ Code int }
		_ = json.Unmarshal([]byte(ev.Data), &x)
		v.exited, v.code = true, x.Code
	case "frame":
		var f Frame
		if err := json.Unmarshal([]byte(ev.Data), &f); err != nil {
			v.t.Fatal(err)
		}
		if f.Type == "full" {
			v.lines = make([]string, f.Rows)
			for y, r := range f.Lines {
				v.lines[y] = rowText(r)
			}
			v.seq = f.Seq
		}
		for _, d := range f.Dirty {
			v.lines[d.Y] = rowText(d.Runs)
		}
	}
}

func (v *streamView) text() string { return strings.Join(v.lines, "\n") }

func (v *streamView) waitFor(cond func() bool, what string) {
	v.t.Helper()
	deadline := time.After(5 * time.Second)
	for !cond() {
		select {
		case ev, ok := <-v.events:
			if !ok {
				v.t.Fatalf("stream ended waiting for %s; screen:\n%s", what, v.text())
			}
			v.apply(ev)
		case <-deadline:
			v.t.Fatalf("timeout waiting for %s; screen:\n%s", what, v.text())
		}
	}
}

func (v *streamView) waitText(sub string) {
	v.t.Helper()
	v.waitFor(func() bool { return strings.Contains(v.text(), sub) }, sub)
}

func postInput(t *testing.T, c *Client, e *env, seq uint64, events ...inputEvent) (*http.Response, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(inputBatch{Seq: seq, Events: events})
	req, _ := http.NewRequest("POST", e.ts.URL+"/term/input", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	return res, out
}

func typed(s string) []inputEvent {
	var evs []inputEvent
	for _, r := range s {
		evs = append(evs, inputEvent{Type: "key", Key: string(r)})
	}
	return evs
}

func TestTerminalKeysReachChild(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "cat", "-v")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=10&cols=40")
	v.waitFor(func() bool { return len(v.lines) == 10 }, "initial frame")
	if v.seq != 1 {
		t.Fatalf("initial seq = %d", v.seq)
	}
	evs := append(typed("hi"), inputEvent{Type: "key", Key: "a", Code: "KeyA", Ctrl: true},
		inputEvent{Type: "key", Key: "ArrowUp"}, inputEvent{Type: "key", Key: "Enter"})
	if res, out := postInput(t, c, e, 1, evs...); res.StatusCode != 200 || out["next"].(float64) != 2 {
		t.Fatalf("input: %d %v", res.StatusCode, out)
	}
	v.waitText("hi^A^[[A")
}

func TestTerminalPasteAndBracketedPaste(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "sh", "-c", `printf '\033[?2004h'; cat -v`)
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=10&cols=60")
	time.Sleep(200 * time.Millisecond)
	postInput(t, c, e, 1, inputEvent{Type: "paste", Text: "pasted"})
	v.waitText("^[[200~pasted^[[201~")
}

func TestTerminalMouseReachesChildOnlyWhenRequested(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "sh", "-c", `printf '\033[?1000h\033[?1006h'; cat -v`)
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=10&cols=60")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	time.Sleep(200 * time.Millisecond)
	postInput(t, c, e, 1,
		inputEvent{Type: "mouse", Action: "press", Button: "left", X: 4, Y: 2},
		inputEvent{Type: "mouse", Action: "release", Button: "left", X: 4, Y: 2},
		inputEvent{Type: "mouse", Action: "press", Button: "wheelup", X: 0, Y: 0, Mods: modCtrl})
	v.waitText("^[[<0;5;3M^[[<0;5;3m^[[<80;1;1M")
}

func TestTerminalMouseIgnoredWithoutTracking(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "cat", "-v")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	postInput(t, c, e, 1, inputEvent{Type: "mouse", Action: "press", Button: "left", X: 1, Y: 1},
		inputEvent{Type: "key", Key: "z"}, inputEvent{Type: "key", Key: "Enter"})
	v.waitText("z")
	if strings.Contains(v.text(), "[<") {
		t.Fatalf("mouse report leaked:\n%s", v.text())
	}
}

func TestTerminalResizeEvent(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "sh", "-c", `trap 'stty size' WINCH; while :; do sleep 0.05; done`)
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=10&cols=40")
	v.waitFor(func() bool { return len(v.lines) == 10 }, "frame")
	time.Sleep(200 * time.Millisecond)
	postInput(t, c, e, 1, inputEvent{Type: "resize", Rows: 12, Cols: 50})
	v.waitFor(func() bool { return len(v.lines) == 12 && strings.Contains(v.text(), "12 50") }, "resized screen with stty size")
}

func TestTerminalInputOrdering(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "cat", "-v")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	if res, out := postInput(t, c, e, 2, typed("B")...); res.StatusCode != http.StatusConflict || out["next"].(float64) != 1 {
		t.Fatalf("future batch: %d %v", res.StatusCode, out)
	}
	if res, _ := postInput(t, c, e, 1, typed("A")...); res.StatusCode != 200 {
		t.Fatalf("batch 1: %d", res.StatusCode)
	}
	if res, out := postInput(t, c, e, 1, typed("A")...); res.StatusCode != 200 || out["duplicate"] != true {
		t.Fatalf("retry: %d %v", res.StatusCode, out)
	}
	if res, _ := postInput(t, c, e, 2, typed("B")...); res.StatusCode != 200 {
		t.Fatalf("batch 2: %d", res.StatusCode)
	}
	postInput(t, c, e, 3, inputEvent{Type: "key", Key: "Enter"})
	v.waitText("AB")
	if strings.Contains(v.text(), "AAB") {
		t.Fatalf("retried batch ran twice:\n%s", v.text())
	}
	if res, _ := postInput(t, c, e, 0); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("seq 0: %d", res.StatusCode)
	}
}

func TestTerminalInputRequiresCSRF(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "cat", "-v")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	req, _ := http.NewRequest("POST", e.ts.URL+"/term/input", strings.NewReader(`{"seq":1,"events":[]}`))
	res, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("no csrf header: %d", res.StatusCode)
	}
}

func TestTerminalRejectsAnonymous(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "cat")
	for _, m := range []string{"GET /term/stream", "POST /term/input", "GET /term/"} {
		verb, path, _ := strings.Cut(m, " ")
		req, _ := http.NewRequest(verb, e.ts.URL+path, strings.NewReader("{}"))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
			t.Errorf("%s anonymous: %d", m, res.StatusCode)
		}
	}
}

func TestTerminalPageRendersInLayout(t *testing.T) {
	e := newEnv(t, nil)
	e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	req, _ := http.NewRequest("GET", e.ts.URL+"/term/", nil)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := readAll(res)
	for _, want := range []string{`id="term"`, `data-prefix="/term/"`, "/static/term.js", "/static/termkeys.js", "/static/termsend.js", `name="csrf-token"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	for _, p := range []string{"/static/term.js", "/static/termkeys.js", "/static/termsend.js", "/static/term.css"} {
		r, err := http.Get(e.ts.URL + p)
		if err != nil || r.StatusCode != 200 {
			t.Errorf("%s not served", p)
		}
		r.Body.Close()
	}
}

func TestTerminalExitSendsExitEventAndCloses(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "sh", "-c", "echo bye; exit 3")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return v.exited }, "exit event")
	if v.code != 3 || !strings.Contains(v.text(), "bye") {
		t.Fatalf("code %d screen:\n%s", v.code, v.text())
	}
	waitUntil(t, func() bool { return h.lookup(sessionDevice(t, e, c)) == nil }, "terminal removed after exit")
}

func sessionDevice(t *testing.T, e *env, c *Client) string {
	t.Helper()
	return c.DeviceID
}

func waitUntil(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTerminalSurvivesReconnectWithinGrace(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat", "-v")
	h.grace = 2 * time.Second
	c := e.loggedIn(t, "phone")
	v, res := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	postInput(t, c, e, 1, append(typed("kept"), inputEvent{Type: "key", Key: "Enter"})...)
	v.waitText("kept")
	v.cancel()
	res.Body.Close()
	time.Sleep(150 * time.Millisecond)
	if h.lookup(c.DeviceID) == nil {
		t.Fatal("terminal closed right after the stream dropped")
	}
	v2, _ := openStream(t, e, c, "rows=5&cols=30&full=1")
	v2.waitText("kept")
	if v2.seq != 2 {
		t.Fatalf("seq in reconnect frame = %d, want 2", v2.seq)
	}
}

func TestTerminalClosesAfterGrace(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	v, res := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	ts := h.lookup(c.DeviceID)
	v.cancel()
	res.Body.Close()
	waitUntil(t, func() bool { return h.lookup(c.DeviceID) == nil }, "terminal closed after grace")
	select {
	case <-ts.t.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("child still running")
	}
}

func TestTerminalSecondStreamReplacesFirst(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	v1, _ := openStream(t, e, c, "rows=5&cols=30")
	v1.waitFor(func() bool { return len(v1.lines) > 0 }, "frame")
	v2, _ := openStream(t, e, c, "rows=5&cols=30")
	v2.waitFor(func() bool { return len(v2.lines) > 0 }, "frame")
	seen := false
	for ev := range v1.events {
		if ev.Name == "replaced" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("first stream was not told it was replaced")
	}
	if h.lookup(c.DeviceID) == nil {
		t.Fatal("replacing a stream closed the terminal")
	}
}

func TestTerminalClosedOnLogout(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	ts := h.lookup(c.DeviceID)
	req, _ := http.NewRequest("POST", e.ts.URL+"/auth/logout", strings.NewReader("{}"))
	if res, err := c.Do(req); err != nil || res.StatusCode != 200 {
		t.Fatalf("logout: %v", err)
	}
	waitUntil(t, func() bool { return h.lookup(c.DeviceID) == nil }, "terminal closed on logout")
	<-ts.t.Exited()
}

func TestTerminalClosedOnSessionExpiry(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.SessionTTL = time.Minute })
	h := e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	e.clock.Advance(2 * time.Minute)
	waitUntil(t, func() bool { return h.lookup(c.DeviceID) == nil }, "terminal closed on expiry")
}

func TestTerminalClosedOnDeviceRevoke(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	ts := h.lookup(c.DeviceID)
	if err := e.admin.RevokeDevice("phone"); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, func() bool { return h.lookup(c.DeviceID) == nil }, "terminal closed on revoke")
	select {
	case <-ts.t.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("child still running after revoke")
	}
	if res, _ := postInput(t, c, e, 2, typed("x")...); res.StatusCode == 200 {
		t.Fatal("input accepted after revoke")
	}
}

func TestTerminalClosedOnServerShutdown(t *testing.T) {
	e := newEnv(t, nil)
	h := e.mountTerm(t, "cat")
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	ts := h.lookup(c.DeviceID)
	e.srv.CloseTerminals()
	select {
	case <-ts.t.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("child still running")
	}
	closed := false
	for ev := range v.events {
		closed = closed || ev.Name == "closed"
	}
	if !closed {
		t.Fatal("stream got no closed event")
	}
}

func TestTerminalLimit(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.MaxTerminals = 1 })
	e.mountTerm(t, "cat")
	a := e.loggedIn(t, "phone")
	b := e.loggedIn(t, "laptop")
	v, _ := openStream(t, e, a, "rows=5&cols=30")
	v.waitFor(func() bool { return len(v.lines) > 0 }, "frame")
	if _, res := openStream(t, e, b, "rows=5&cols=30"); res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("second terminal: %d", res.StatusCode)
	}
}

func TestSpawnArgvEnv(t *testing.T) {
	e := newEnv(t, nil)
	e.srv.MountTerminal("/term/", SpawnArgv([]string{"sh", "-c", `echo "$TERM:$FOO"`}, t.TempDir(), []string{"FOO=bar"}))
	t.Cleanup(e.srv.CloseTerminals)
	c := e.loggedIn(t, "phone")
	v, _ := openStream(t, e, c, "rows=5&cols=30")
	v.waitText("xterm-256color:bar")
}
