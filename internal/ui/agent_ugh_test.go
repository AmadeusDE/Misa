//go:build !nough

package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const fixtureReply = `data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"fixture-pong"}}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"finish_reason":"stop","delta":{}}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}

data: [DONE]

`

// isolatedUgh points ugh's config, state and keyring at temp dirs, with one
// provider, "fixture", that streams fixtureReply from a local server.
func isolatedUgh(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, fixtureReply)
	}))
	t.Cleanup(srv.Close)
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

func typeKeys(m *Model, text string) tea.Cmd {
	var cmds []tea.Cmd
	for _, r := range text {
		_, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		cmds = append(cmds, cmd)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return tea.Batch(append(cmds, cmd)...)
}

func TestUghStreamsInPaneAndQuitsToPrompt(t *testing.T) {
	isolatedUgh(t)
	m := fullShellModel(t)
	start := typeCommand(m, "ugh -m fixture/m -thinking off")
	if m.session == nil {
		t.Fatalf("ugh did not start: %q", m.shellBuffer)
	}
	p := newPump(t, m)
	p.run(tea.Batch(start, typeKeys(m, "hi")))
	p.until("the streamed reply", func() bool {
		return strings.Contains(ansi.Strip(m.render()), "fixture-pong") && !m.session.a.(ughAgent).Status().Running
	})
	p.run(typeKeys(m, "/quit"))
	p.until("quit", func() bool { return m.session == nil })
	if strings.Contains(ansi.Strip(m.render()), "fixture-pong") {
		t.Error("pane still shows ugh after /quit")
	}
}
