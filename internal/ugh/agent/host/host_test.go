package host

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
)

type scripted struct{ reply string }

func (s scripted) Stream(_ context.Context, _ llm.Request) (<-chan llm.Event, error) {
	ch := make(chan llm.Event, 3)
	ch <- llm.Event{Type: llm.EventTextDelta, TextDelta: s.reply}
	ch <- llm.Event{Type: llm.EventUsage, Usage: &llm.Usage{In: 10, Out: 2, Cost: 0.01}}
	ch <- llm.Event{Type: llm.EventDone, Stop: "stop"}
	close(ch)
	return ch, nil
}

type agentConfig = agent.Config

type allowAll struct{}

func (allowAll) Check(context.Context, perm.Call) perm.Outcome { return perm.Outcome{Action: "allow"} }

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	return home
}

func botRoots(t *testing.T) roots.Roots {
	t.Helper()
	base := t.TempDir()
	r := roots.Roots{Config: filepath.Join(base, "config"), State: filepath.Join(base, "state")}
	if err := os.MkdirAll(r.Config, 0o700); err != nil {
		t.Fatal(err)
	}
	return r
}

func newHost(t *testing.T, r roots.Roots, cwd, reply string, extra func(*Options)) *Runner {
	t.Helper()
	o := Options{Cwd: cwd, Model: "fake/m", Roots: r, Gate: allowAll{}, Provider: scripted{reply}}
	if extra != nil {
		extra(&o)
	}
	run, err := New(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.Close() })
	return run
}

func filesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestHostsWithDifferentRootsNeverShareState(t *testing.T) {
	home := isolatedHome(t)
	cwd := t.TempDir()
	a, b := botRoots(t), botRoots(t)

	ha := newHost(t, a, cwd, "from a", nil)
	hb := newHost(t, b, cwd, "from b", nil)
	if _, err := ha.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := hb.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}

	pa, err := session.LatestIn(a, cwd)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := session.LatestIn(b, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if pa == pb || !strings.HasPrefix(pa, a.State) || !strings.HasPrefix(pb, b.State) {
		t.Fatalf("sessions leaked: %s %s", pa, pb)
	}
	for _, r := range []roots.Roots{a, b} {
		routes := filepath.Join(r.State, "routes.jsonl")
		if _, err := os.Stat(routes); err != nil {
			t.Fatalf("no route log under %s: %v", r.State, err)
		}
	}
	if left := filesUnder(t, home); len(left) != 0 {
		t.Fatalf("host touched the user's home: %v", left)
	}
}

func TestResumeStaysInsideItsRoots(t *testing.T) {
	isolatedHome(t)
	cwd := t.TempDir()
	a, b := botRoots(t), botRoots(t)
	first := newHost(t, a, cwd, "one", nil)
	if _, err := first.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}

	resumed := newHost(t, a, cwd, "two", func(o *Options) { o.Resume = true })
	if resumed.Resumed == 0 || resumed.Session.Path() != first.Session.Path() {
		t.Fatalf("did not resume own session: %d %s", resumed.Resumed, resumed.Session.Path())
	}
	_, err := New(context.Background(), Options{Cwd: cwd, Model: "fake/m", Roots: b, Gate: allowAll{}, Provider: scripted{"x"}, Resume: true})
	if err == nil {
		t.Fatal("other roots resumed a session they do not own")
	}
}

func TestConfigComesFromRoots(t *testing.T) {
	isolatedHome(t)
	r := botRoots(t)
	if err := os.WriteFile(filepath.Join(r.Config, "config.toml"), []byte("[agent]\nmax_turns_root = 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := newHost(t, r, t.TempDir(), "ok", nil)
	if got := run.Runtime.Config.Agent.MaxTurnsRoot; got != 7 {
		t.Fatalf("max_turns_root = %d, want 7 from the bot's config", got)
	}
}

func TestGateLogsDecisionsUnderItsRoots(t *testing.T) {
	home := isolatedHome(t)
	r := botRoots(t)
	if err := os.WriteFile(filepath.Join(r.Config, "config.toml"), []byte("[permissions]\nmode = \"manual\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	broker := perm.NewBroker(strings.NewReader(""), io.Discard, false, "")
	run, err := New(context.Background(), Options{Cwd: t.TempDir(), Model: "fake/m", Roots: r, Prompter: broker, Provider: scripted{"ok"}})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	if _, err := os.Stat(filepath.Join(r.State, "decisions.jsonl")); err != nil {
		t.Fatalf("decision log not under the bot's state root: %v", err)
	}
	if left := filesUnder(t, home); len(left) != 0 {
		t.Fatalf("gate touched the user's home: %v", left)
	}
}

func TestNeedsPrompterOrGate(t *testing.T) {
	if _, err := New(context.Background(), Options{Cwd: t.TempDir(), Model: "fake/m", Provider: scripted{""}}); err == nil {
		t.Fatal("host without prompter or gate accepted")
	}
}

func TestConfigureSeesAgentConfig(t *testing.T) {
	isolatedHome(t)
	var seen string
	newHost(t, botRoots(t), t.TempDir(), "ok", func(o *Options) {
		o.Configure = func(c *agentConfig) { seen = c.Model }
	})
	if seen != "m" {
		t.Fatalf("Configure saw model %q", seen)
	}
}

func TestResumeKeepsRecordedModelUnlessConfigOverrides(t *testing.T) {
	isolatedHome(t)
	cwd := t.TempDir()
	r := botRoots(t)
	if _, err := newHost(t, r, cwd, "one", nil).Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	kept := newHost(t, r, cwd, "two", func(o *Options) { o.Resume = true; o.Model = "fake/new"; o.System = "new system" })
	if kept.Model != "fake/m" {
		t.Fatalf("default resume must keep the session's model, got %q", kept.Model)
	}
	over := newHost(t, r, cwd, "three", func(o *Options) {
		o.Resume = true
		o.Model = "fake/new"
		o.System = "new system"
		o.OverrideResumed = true
	})
	if over.Model != "fake/new" || over.System != "new system" {
		t.Fatalf("override ignored: model=%q system=%q", over.Model, over.System)
	}
	if over.Resumed == 0 {
		t.Fatal("override must still resume history")
	}
}
