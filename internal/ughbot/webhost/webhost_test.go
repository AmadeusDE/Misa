package webhost

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/daemon"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/webui"
)

var ughbotBin string

// TestMain builds the ughbot binary the browser terminal spawns.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "webhost-ughbot")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ughbotBin = filepath.Join(dir, "ughbot")
	build := exec.Command("go", "build", "-o", ughbotBin, "./cmd/ughbot")
	build.Dir = "../../.."
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building ughbot: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func startStack(t *testing.T) (base string, ck *clock, admin *webui.Admin, secret string) {
	t.Helper()
	dir := t.TempDir()
	cfgText := `
state_dir = "` + filepath.Join(dir, "state") + `"
config_dir = "` + filepath.Join(dir, "config") + `"
socket = "` + filepath.Join(dir, "u.sock") + `"
[web]
enabled = true
listen_addr = "127.0.0.1:0"
[bots.a]
model = "fake/m"
capabilities = { files = "read" }
workdir = "` + filepath.Join(dir, "work") + `"
`
	cfgPath := filepath.Join(dir, "ughbot.toml")
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]byte(cfgText))
	if err != nil {
		t.Fatal(err)
	}
	d, err := daemon.New(daemon.Options{Config: cfg, Provider: fakellm.Echo(0.01)})
	if err != nil {
		t.Fatal(err)
	}
	ck = &clock{t: time.Now().UTC()}
	h, err := New(Options{Config: cfg, ConfigPath: cfgPath, Executable: ughbotBin, Logf: func(string, ...any) {}, Now: ck.Now})
	if err != nil {
		t.Fatal(err)
	}
	admin = webui.NewAdmin(h.Store, ck.Now)
	if secret, _, err = admin.TOTPInit("ughbot", "me"); err != nil {
		t.Fatal(err)
	}
	ck.Advance(time.Minute)
	ln, err := h.Listen()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 2)
	go func() { done <- d.Run(ctx) }()
	go func() { done <- h.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		for range 2 {
			select {
			case <-done:
			case <-time.After(20 * time.Second):
				t.Error("shutdown timed out")
			}
		}
	})
	return "http://" + ln.Addr().String(), ck, admin, secret
}

type screen struct {
	events chan string
	lines  []string
}

func openTerminal(t *testing.T, c *webui.Client, base string) *screen {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/term/stream?rows=24&cols=100", nil)
	res, err := c.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("terminal stream: %d", res.StatusCode)
	}
	t.Cleanup(func() { cancel(); res.Body.Close() })
	s := &screen{events: make(chan string, 256)}
	go func() {
		defer close(s.events)
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(nil, 1<<22)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				s.events <- data
			}
		}
	}()
	return s
}

func (s *screen) apply(data string) {
	var f struct {
		Type  string `json:"type"`
		Rows  int    `json:"rows"`
		Lines [][]struct{ T string }
		Dirty []struct {
			Y int
			R []struct{ T string }
		}
	}
	if json.Unmarshal([]byte(data), &f) != nil {
		return
	}
	text := func(runs []struct{ T string }) string {
		var sb strings.Builder
		for _, r := range runs {
			sb.WriteString(r.T)
		}
		return sb.String()
	}
	if f.Type == "full" {
		s.lines = make([]string, f.Rows)
		for y, r := range f.Lines {
			s.lines[y] = text(r)
		}
	}
	for _, d := range f.Dirty {
		if d.Y < len(s.lines) {
			s.lines[d.Y] = text(d.R)
		}
	}
}

func (s *screen) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for !strings.Contains(strings.Join(s.lines, "\n"), want) {
		select {
		case data, ok := <-s.events:
			if !ok {
				t.Fatalf("stream ended waiting for %q:\n%s", want, strings.Join(s.lines, "\n"))
			}
			s.apply(data)
		case <-deadline:
			t.Fatalf("timeout waiting for %q:\n%s", want, strings.Join(s.lines, "\n"))
		}
	}
}

func sendKeys(t *testing.T, c *webui.Client, base string, seq int, keys ...string) {
	t.Helper()
	var evs []map[string]any
	for _, k := range keys {
		evs = append(evs, map[string]any{"type": "key", "key": k})
	}
	body, _ := json.Marshal(map[string]any{"seq": seq, "events": evs})
	req, _ := http.NewRequest(http.MethodPost, base+"/term/input", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("terminal input: %d", res.StatusCode)
	}
}

func TestWebEndToEnd(t *testing.T) {
	base, ck, _, secret := startStack(t)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for _, path := range []string{"/term/", "/term/stream"} {
		req, _ := http.NewRequest(http.MethodGet, base+path, nil)
		req.Header.Set("Accept", "text/html")
		res, err := noRedirect.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode == http.StatusOK || (res.StatusCode == http.StatusSeeOther && !strings.HasPrefix(res.Header.Get("Location"), "/login")) {
			t.Fatalf("anonymous %s: %d -> %q", path, res.StatusCode, res.Header.Get("Location"))
		}
	}
	res, err := noRedirect.Post(base+"/term/input", "application/json", strings.NewReader(`{"seq":1,"events":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
		t.Fatalf("anonymous input: %d", res.StatusCode)
	}

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	c, err := webui.NewClient(base, priv)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := webui.TOTPCode(secret, ck.Now())
	if err := c.Enroll(t.Context(), code, "test-device"); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(t.Context()); err != nil {
		t.Fatal(err)
	}

	root, _ := http.NewRequest(http.MethodGet, base+"/", nil)
	res, err = c.Do(root)
	if err != nil || res.Request.URL.Path != "/term/" {
		t.Fatalf("root did not land on the terminal: %v %v", err, res)
	}
	res.Body.Close()

	post, _ := http.NewRequest(http.MethodPost, base+"/term/input", strings.NewReader(`{"seq":1,"events":[]}`))
	post.Header.Set("Content-Type", "application/json")
	res, err = c.HTTP.Do(post) // session cookie, no CSRF header
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("input without CSRF: %d", res.StatusCode)
	}

	scr := openTerminal(t, c, base)
	scr.waitFor(t, "enter open")
	scr.waitFor(t, "fake/m")
	sendKeys(t, c, base, 1, "Enter")
	scr.waitFor(t, "enter send")
	scr.waitFor(t, "attached to a")
	sendKeys(t, c, base, 2, "h", "i", "Enter")
	scr.waitFor(t, "echo: hi")
}

func TestTerminalArgv(t *testing.T) {
	cfg, err := config.Parse([]byte("socket = \"/s/u.sock\"\n[bots.a]\nmodel='m'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(TerminalArgv(cfg, "/bin/ughbot", "/c/u.toml"), " "); got != "/bin/ughbot tui -c /c/u.toml -s /s/u.sock" {
		t.Fatalf("default argv = %q", got)
	}
	cfg.Web.Command = []string{"misa", "--x"}
	if got := strings.Join(TerminalArgv(cfg, "/bin/ughbot", "/c/u.toml"), " "); got != "misa --x" {
		t.Fatalf("override argv = %q", got)
	}
}

func TestTOTPConfigured(t *testing.T) {
	store, err := webui.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if TOTPConfigured(store) {
		t.Fatal("fresh store reports TOTP")
	}
	if _, _, err := webui.NewAdmin(store, nil).TOTPInit("a", "b"); err != nil {
		t.Fatal(err)
	}
	if !TOTPConfigured(store) {
		t.Fatal("TOTP not reported after init")
	}
}
