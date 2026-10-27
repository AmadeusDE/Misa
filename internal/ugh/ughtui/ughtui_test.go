package ughtui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// fakeHost records what ugh sends and drives the model the way Misa does:
// runs commands, expands batches, feeds results back on one goroutine.
type fakeHost struct {
	m     *Model
	msgs  chan tea.Msg
	stop  chan struct{}
	mu    sync.Mutex
	warns []string
	sent  int
}

func newFakeHost() *fakeHost {
	return &fakeHost{msgs: make(chan tea.Msg, 4096), stop: make(chan struct{})}
}

func (h *fakeHost) Send(msg tea.Msg) {
	h.mu.Lock()
	h.sent++
	h.mu.Unlock()
	h.put(msg)
}

func (h *fakeHost) Warn(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.warns = append(h.warns, text)
}

func (h *fakeHost) put(msg tea.Msg) {
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			h.run(c)
		}
		return
	}
	if msg == nil {
		return
	}
	select {
	case h.msgs <- msg:
	case <-h.stop:
	}
}

func (h *fakeHost) run(cmd tea.Cmd) {
	if cmd != nil {
		go func() { h.put(cmd()) }()
	}
}

// until feeds messages through Update until cond holds after one of them;
// it returns that message.
func (h *fakeHost) until(t *testing.T, cond func(tea.Msg) bool) tea.Msg {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case msg := <-h.msgs:
			_, cmd := h.m.Update(msg)
			h.run(cmd)
			if cond(msg) {
				return msg
			}
		case <-deadline:
			t.Fatalf("host: condition never met; view:\n%s", h.m.View().Content)
		}
	}
}

func (h *fakeHost) typeLine(text string) {
	for _, r := range text {
		h.put(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	h.put(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// fixtureServer streams a recorded Chat Completions SSE fixture; hold, if
// set, keeps the stream open until the request is cancelled.
func fixtureServer(t *testing.T, fixture string, hold bool) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "llm", "chat", "testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if hold {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// isolated points every ugh path at temp dirs and configures one custom
// provider, "fixture", at srv; nothing touches the user's files, keyring or
// the network.
func isolated(t *testing.T, srv *httptest.Server) {
	t.Helper()
	home := t.TempDir()
	for _, v := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(v, filepath.Join(home, v))
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(home, "no-bus"))
	t.Setenv("UGH_FIXTURE_KEY", "fixture-key")
	dir := filepath.Join(home, "XDG_CONFIG_HOME", "ugh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`[auth]
store = "file"
[permissions]
mode = "auto"
[context]
ladder = "off"
digest = "off"
[providers.fixture]
base_url = %q
key_env = "UGH_FIXTURE_KEY"
usage = "none"
`, srv.URL+"/v1")
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

// captureTerminal swaps stdout and stderr for a pipe and returns what was
// written. It changes process globals: these tests do not run in parallel.
func captureTerminal(t *testing.T) func() string {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = w, w
	got := make(chan string, 1)
	go func() { b, _ := io.ReadAll(r); got <- string(b) }()
	return func() string {
		os.Stdout, os.Stderr = oldOut, oldErr
		w.Close()
		return <-got
	}
}

func start(t *testing.T, srv *httptest.Server) *fakeHost {
	t.Helper()
	isolated(t, srv)
	h := newFakeHost()
	m, err := New(Options{Model: "fixture/mimo-v2.6-pro", Thinking: "off", Cwd: t.TempDir(), Host: h})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m
	h.run(m.Init())
	h.put(tea.WindowSizeMsg{Width: 100, Height: 30})
	t.Cleanup(func() { m.Close(); close(h.stop) })
	return h
}

func (h *fakeHost) runToEnd(t *testing.T, prompt string) {
	t.Helper()
	h.typeLine(prompt)
	h.until(t, func(tea.Msg) bool { return h.m.Status().Running })
	h.until(t, func(tea.Msg) bool { return !h.m.Status().Running })
}

// A prompt streams its reply through Host.Send into the view, and nothing
// is written to the terminal.
func TestContractStreamsThroughHostAndNeverTheTerminal(t *testing.T) {
	restore := captureTerminal(t)
	h := start(t, fixtureServer(t, "text.sse", false))
	h.runToEnd(t, "hi")
	view := h.m.View().Content
	h.m.Close()
	if out := restore(); out != "" {
		t.Fatalf("wrote to the terminal: %q", out)
	}
	h.mu.Lock()
	sent := h.sent
	h.mu.Unlock()
	if sent == 0 {
		t.Fatal("nothing arrived through Host.Send")
	}
	if st := h.m.Status(); st.Model != "fixture/mimo-v2.6-pro" || st.Usage.Out == 0 {
		t.Errorf("status = %+v", st)
	}
	if !strings.Contains(view, "pong") {
		t.Errorf("streamed reply (\"pong\") missing from view:\n%s", view)
	}
}

// /quit gives ExitMsg, never tea.Quit.
func TestContractQuitGivesExitMsg(t *testing.T) {
	h := start(t, fixtureServer(t, "text.sse", false))
	h.typeLine("/quit")
	h.until(t, func(msg tea.Msg) bool {
		if _, quit := msg.(tea.QuitMsg); quit {
			t.Fatal("ugh returned tea.Quit")
		}
		_, ok := msg.(ExitMsg)
		return ok
	})
}

// /copy is a command for the host (OSC 52), not a terminal write.
func TestContractCopyIsAHostCommand(t *testing.T) {
	restore := captureTerminal(t)
	h := start(t, fixtureServer(t, "text.sse", false))
	h.runToEnd(t, "hi")
	h.typeLine("/copy")
	h.until(t, func(msg tea.Msg) bool { return strings.Contains(fmt.Sprintf("%T", msg), "Clipboard") })
	h.m.Close()
	if out := restore(); out != "" {
		t.Fatalf("wrote to the terminal: %q", out)
	}
}

// Close during a run that is waiting on the provider leaves no goroutines
// behind.
func TestContractCloseMidRunLeaksNothing(t *testing.T) {
	srv := fixtureServer(t, "text.sse", true)
	isolated(t, srv)
	baseline := runtime.NumGoroutine()
	h := newFakeHost()
	m, err := New(Options{Model: "fixture/mimo-v2.6-pro", Thinking: "off", Cwd: t.TempDir(), Host: h})
	if err != nil {
		t.Fatal(err)
	}
	h.m = m
	h.run(m.Init())
	h.typeLine("hi")
	h.until(t, func(tea.Msg) bool { return m.Status().Running })
	m.Close()
	close(h.stop)
	srv.CloseClientConnections()
	for deadline := time.Now().Add(5 * time.Second); runtime.NumGoroutine() > baseline+2; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			t.Fatalf("goroutines %d > baseline %d:\n%s", runtime.NumGoroutine(), baseline, buf[:runtime.Stack(buf, true)])
		}
	}
}
