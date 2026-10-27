package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/client"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

type harness struct {
	dir string
	p   *fakellm.Provider
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir, err := os.MkdirTemp("", "ub")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("HOME", dir)
	return &harness{dir: dir, p: fakellm.Echo(0.01)}
}

func (h *harness) config(t *testing.T, daily float64) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(`
state_dir = "` + filepath.Join(h.dir, "state") + `"
config_dir = "` + filepath.Join(h.dir, "config") + `"
socket = "` + filepath.Join(h.dir, "u.sock") + `"
[bots.alpha]
model = "fake/m"
capabilities = { files = "read" }
workdir = "` + filepath.Join(h.dir, "work") + `"
spend = { daily = ` + strconv.FormatFloat(daily, 'f', -1, 64) + ` }
[bots.beta]
model = "fake/m"
capabilities = { files = "read" }
`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// run starts a daemon and returns a client plus a stop that waits for shutdown.
func (h *harness) run(t *testing.T, daily float64) (*Daemon, *client.Client, func()) {
	t.Helper()
	d, err := New(Options{Config: h.config(t, daily), Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() { errc <- d.Run(context.Background()) }()
	c, err := client.Dial(d.cfg.Socket)
	if err != nil {
		t.Fatal(err)
	}
	stop := func() {
		if err := c.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("run: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("daemon did not shut down")
		}
		c.Close()
	}
	return d, c, stop
}

func awaitType(t *testing.T, ch <-chan api.Event, typ string) api.Event {
	t.Helper()
	timeout := time.After(15 * time.Second)
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

func TestMilestoneSendResumeAndBudget(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. a message gets a streamed reply
	_, c, stop := h.run(t, 0)
	events, err := c.Subscribe(ctx, "alpha", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send(ctx, "alpha", "remember the word pineapple"); err != nil {
		t.Fatal(err)
	}
	text := awaitType(t, events, "text")
	if !strings.Contains(string(text.Data), "echo: remember the word pineapple") {
		t.Fatalf("streamed text = %s", text.Data)
	}
	awaitType(t, events, "turn_end")
	if _, err := c.Send(ctx, "nobody", "hi"); err == nil {
		t.Fatal("sending to an unknown bot should fail")
	}
	stop()

	// 2. a restart resumes the thread with its prior context
	d2, c, stop := h.run(t, 0)
	events, err = c.Subscribe(ctx, "alpha", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := d2.bots["alpha"].Resumed(); n == 0 {
		t.Fatal("restarted bot did not resume its session")
	}
	if _, err := c.Send(ctx, "alpha", "what word?"); err != nil {
		t.Fatal(err)
	}
	awaitType(t, events, "turn_end")
	if !strings.Contains(h.p.History(), "pineapple") {
		t.Fatalf("model lost the earlier context:\n%s", h.p.History())
	}
	stop()

	// 3. a tiny daily cap refuses the next turn, and cost shows per-bot spend
	_, c, stop = h.run(t, 0.015)
	defer stop()
	events, err = c.Subscribe(ctx, "alpha", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send(ctx, "alpha", "one more"); err != nil {
		t.Fatal(err)
	}
	refused := awaitType(t, events, "budget")
	if !strings.Contains(string(refused.Data), "budget exhausted") {
		t.Fatalf("budget event = %s", refused.Data)
	}
	rows, err := c.Cost(ctx, api.CostQuery{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("cost rows = %v err=%v", rows, err)
	}
	if rows[0].Bot != "alpha" || rows[0].USD < 0.0199 || rows[0].USD > 0.0201 {
		t.Fatalf("cost row = %+v, want alpha at $0.02", rows[0])
	}
	st, err := c.Status(ctx)
	if err != nil || len(st.Bots) != 2 {
		t.Fatalf("status = %+v err=%v", st, err)
	}
}

func TestSecondDaemonOnTheSameSocketIsRefused(t *testing.T) {
	h := newHarness(t)
	_, _, stop := h.run(t, 0)
	defer stop()
	if _, err := New(Options{Config: h.config(t, 0), Provider: h.p}); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("err = %v", err)
	}
}

func TestShutdownRequeuesClaimedMail(t *testing.T) {
	h := newHarness(t)
	d, err := New(Options{Config: h.config(t, 0), Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.st.Enqueue(store.Mail{To: "beta", From: "user", Body: "pending"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := d.st.Claim("beta"); !ok {
		t.Fatal("claim failed")
	}
	d.closeAll()
	d2, err := New(Options{Config: h.config(t, 0), Provider: h.p})
	if err != nil {
		t.Fatal(err)
	}
	defer d2.closeAll()
	if n, _ := d2.st.Pending("beta"); n != 1 {
		t.Fatalf("pending after shutdown = %d, want the claimed mail back", n)
	}
}
