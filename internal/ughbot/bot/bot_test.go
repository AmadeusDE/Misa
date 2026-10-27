package bot

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/ledger"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

type rig struct {
	cfg *config.Config
	st  *store.Store
	led *ledger.Ledger
}

// newRig configures bot "alpha" with the given capabilities (TOML inline-table body) plus any extra bots.
func newRig(t *testing.T, caps string, daily float64, extra ...string) rig {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	src := `
state_dir = "` + filepath.Join(dir, "state") + `"
config_dir = "` + filepath.Join(dir, "config") + `"
socket = "` + filepath.Join(dir, "s.sock") + `"
[bots.alpha]
model = "fake/m"
capabilities = {` + caps + `}
workdir = "` + filepath.Join(dir, "work") + `"
spend = { daily = ` + strconv.FormatFloat(daily, 'f', -1, 64) + ` }
`
	for _, e := range extra {
		src += e + "\n"
	}
	cfg, err := config.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return rig{cfg, st, ledger.New(st)}
}

func (r rig) bot(t *testing.T, p llm.Provider) *Bot { return r.named(t, "alpha", p, nil) }

func (r rig) named(t *testing.T, name string, p llm.Provider, dir Directory) *Bot {
	t.Helper()
	b, err := New(Deps{Config: r.cfg, Bot: r.cfg.Bots[name], Store: r.st, Ledger: r.led, Provider: p, Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func waitFor(t *testing.T, ch <-chan api.Event, typ string) api.Event {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("stream closed before %q", typ)
			}
			if ev.Type == typ {
				return ev
			}
		case <-timeout:
			t.Fatalf("no %q event", typ)
		}
	}
}

func TestMailStreamsAReplyAndIsAcked(t *testing.T) {
	r := newRig(t, `files="read"`, 0)
	b := r.bot(t, fakellm.Echo(0.01))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := b.Subscribe(ctx, 0)
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	if _, err := b.Send("hello"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, events, "user")
	text := waitFor(t, events, "text")
	if !strings.Contains(string(text.Data), "echo: hello") {
		t.Fatalf("text = %s", text.Data)
	}
	waitFor(t, events, "turn_end")
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, _ := r.st.Pending("alpha")
		if _, ok, _ := r.st.Claim("alpha"); n == 0 && !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mail not drained")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMailFromABotIsLabelled(t *testing.T) {
	r := newRig(t, `files="read"`, 0)
	p := fakellm.Echo(0)
	b := r.bot(t, p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := b.Subscribe(ctx, 0)
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	if _, err := b.Deliver("beta", "status?"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, events, "mail")
	waitFor(t, events, "turn_end")
	if got := p.History(); !strings.Contains(got, "[mail from beta]") {
		t.Fatalf("model saw %q", got)
	}
}

func TestStartTaskReturnsItsResultAsMail(t *testing.T) {
	r := newRig(t, `files="read", tasks=1`, 0)
	p := &fakellm.Provider{Script: func(req llm.Request, _ int) []llm.Event {
		last := fakellm.LastUser(req)
		switch {
		case strings.Contains(last, "[mail from task:"):
			return fakellm.Reply("got it: "+last, 0)
		case strings.Contains(last, "subtask"):
			return fakellm.Reply("dug", 0)
		case strings.Contains(last, "dig"):
			for _, m := range req.Messages {
				if m.Role == llm.RoleTool {
					return fakellm.Reply("started", 0)
				}
			}
			return fakellm.ToolCall("c1", "start_task", `{"prompt":"subtask: dig deeper"}`, 0)
		}
		return fakellm.Reply("?", 0)
	}}
	b := r.bot(t, p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := b.Subscribe(ctx, 0)
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	if _, err := b.Send("please dig"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, events, "mail")
	end := waitFor(t, events, "turn_end")
	_ = end
	if !strings.Contains(p.History(), "dug") {
		t.Fatalf("main thread never saw the task result:\n%s", p.History())
	}
}

func TestBudgetRefusalIsAnEventNotACrash(t *testing.T) {
	r := newRig(t, `files="read"`, 0.005)
	b := r.bot(t, fakellm.Echo(0.01))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := b.Subscribe(ctx, 0)
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	_, _ = b.Send("one")
	waitFor(t, events, "turn_end")
	_, _ = b.Send("two")
	ev := waitFor(t, events, "budget")
	if !strings.Contains(string(ev.Data), "budget exhausted") {
		t.Fatalf("budget event = %s", ev.Data)
	}
}
