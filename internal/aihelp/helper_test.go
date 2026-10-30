package aihelp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ai/aitest"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// fake answers each Stream call from a script of events or errors and records the requests.
type fake struct {
	steps []step
	reqs  []llm.Request
}

type step struct {
	events []llm.Event
	err    error
}

func (f *fake) Stream(_ context.Context, req llm.Request) (<-chan llm.Event, error) {
	f.reqs = append(f.reqs, req)
	i := len(f.reqs) - 1
	if i >= len(f.steps) {
		return nil, errors.New("script exhausted")
	}
	if f.steps[i].err != nil {
		return nil, f.steps[i].err
	}
	ch := make(chan llm.Event, len(f.steps[i].events))
	for _, ev := range f.steps[i].events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func says(s string) step  { return step{events: []llm.Event{{Type: llm.EventTextDelta, TextDelta: s}}} }
func fails(code int) step { return step{err: &llm.HTTPError{StatusCode: code, Body: "no"}} }

func calls(name, args string) step {
	return step{events: []llm.Event{{Type: llm.EventToolCall, Call: &llm.ToolCall{ID: "c1", Name: name, Args: args}}}}
}

func helper(t *testing.T, models map[string]*fake, order ...string) (*Helper, *[]time.Duration) {
	t.Helper()
	var slept []time.Duration
	h := &Helper{
		Models: order,
		Open: func(ref string) (*ai.Client, error) {
			p, ok := models[ref]
			if !ok {
				return nil, errors.New("no such model " + ref)
			}
			return &ai.Client{Ref: ref, Model: ref, P: p}, nil
		},
		Cache: &Cache{Dir: t.TempDir()},
		Delay: time.Second,
		Sleep: func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil },
	}
	return h, &slept
}

func TestAskStreamsAndCaches(t *testing.T) {
	p := &fake{steps: []step{{events: []llm.Event{
		{Type: llm.EventTextDelta, TextDelta: "Use "}, {Type: llm.EventTextDelta, TextDelta: "`-S`."},
	}}}}
	h, _ := helper(t, map[string]*fake{"a/m": p}, "a/m")
	var streamed []string
	got, err := h.Ask(context.Background(), Query{Command: "pacman", Question: "install"}, func(s string) { streamed = append(streamed, s) })
	if err != nil || got != "Use `-S`." || strings.Join(streamed, "|") != "Use |`-S`." {
		t.Fatalf("Ask = %q, %v, streamed %q", got, err, streamed)
	}
	if !strings.Contains(p.reqs[0].Messages[0].Text, "About the command 'pacman': install") || !strings.Contains(p.reqs[0].System, "terminal command help assistant") {
		t.Errorf("request = %+v", p.reqs[0])
	}
	if len(p.reqs[0].Tools) != 0 {
		t.Errorf("tools %v sent although none were given", p.reqs[0].Tools)
	}

	streamed = nil
	got, err = h.Ask(context.Background(), Query{Command: "pacman", Question: "install"}, func(s string) { streamed = append(streamed, s) })
	if err != nil || got != "Use `-S`." || len(streamed) != 1 || len(p.reqs) != 1 {
		t.Errorf("cached Ask = %q, %v, streamed %q, %d provider calls", got, err, streamed, len(p.reqs))
	}

	p.steps = append(p.steps, says("fresh"))
	got, _ = h.Ask(context.Background(), Query{Command: "pacman", Question: "install", NoCache: true}, nil)
	if got != "fresh" || len(p.reqs) != 2 {
		t.Errorf("NoCache Ask = %q after %d calls", got, len(p.reqs))
	}
	if cached, _ := h.Cache.Get("a/m", "About the command 'pacman': install"); cached != "fresh" {
		t.Errorf("NoCache did not refresh the cache: %q", cached)
	}
}

func TestPromptShapes(t *testing.T) {
	for _, c := range []struct {
		q    Query
		want string
	}{
		{Query{Command: "ls"}, "Describe the command 'ls'."},
		{Query{Question: "how do I untar"}, "how do I untar"},
		{Query{Command: "tar", Question: "x", Context: "PAGE"}, "About the command 'tar': x\n\nThe user is reading this page:\n\nPAGE"},
	} {
		if got := c.q.prompt(); got != c.want {
			t.Errorf("prompt = %q, want %q", got, c.want)
		}
	}
	h, _ := helper(t, nil)
	if _, err := h.Ask(context.Background(), Query{}, nil); err == nil {
		t.Error("an empty question was asked")
	}
}

func TestToolsAreWiredThroughRunTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tldr := filepath.Join(home, ".local", "share", "misa", "tldr", "en", "common")
	os.MkdirAll(tldr, 0o755)
	os.WriteFile(filepath.Join(tldr, "frob.md"), []byte("# frob\n\n> Frobnicate things.\n"), 0o644)

	srv, _ := wikiServer(t)
	p := &fake{steps: []step{
		calls("tldr_page", `{"name":"frob"}`),
		calls("man_page", `{"name":"no-such-command-xyz"}`),
		calls("arch_wiki", `{"query":"systemd timers"}`),
		says("Frobnicate."),
	}}
	h, _ := helper(t, map[string]*fake{"a/m": p}, "a/m")
	h.Tools = Tools(Wiki{BaseURL: srv.URL})
	var notices strings.Builder
	h.Notice = &notices
	got, err := h.Ask(context.Background(), Query{Command: "frob"}, nil)
	if err != nil || got != "Frobnicate." {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	var names []string
	for _, s := range p.reqs[0].Tools {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "man_page,tldr_page,arch_wiki" {
		t.Errorf("tools offered: %v", names)
	}
	results := map[string]string{}
	for _, m := range p.reqs[3].Messages {
		if m.Role == llm.RoleTool {
			results[m.ToolName] = m.Text
		}
	}
	if !strings.Contains(results["tldr_page"], "Frobnicate things.") {
		t.Errorf("tldr result %q", results["tldr_page"])
	}
	if !strings.HasPrefix(results["man_page"], "error: ") || !strings.Contains(results["man_page"], "not found") {
		t.Errorf("man result %q", results["man_page"])
	}
	if !strings.Contains(results["arch_wiki"], "# Arch Wiki: Systemd") || !strings.Contains(results["arch_wiki"], "Other matches: Systemd, Systemd/Timers") {
		t.Errorf("wiki result %q", results["arch_wiki"])
	}
	if !strings.Contains(notices.String(), "[tldr_page") {
		t.Errorf("tool use not announced: %q", notices.String())
	}
}

func TestToolArgumentErrors(t *testing.T) {
	tools := Tools(Wiki{})
	for _, c := range []struct{ tool, args string }{
		{"man_page", `{}`}, {"man_page", `{"name":"  "}`}, {"man_page", `[`},
		{"tldr_page", `{}`}, {"arch_wiki", `{}`}, {"arch_wiki", `nope`},
	} {
		for _, tool := range tools {
			if tool.Spec.Name != c.tool {
				continue
			}
			if out, err := tool.Run(context.Background(), []byte(c.args)); err == nil {
				t.Errorf("%s(%s) = %q, want an error", c.tool, c.args, out)
			}
		}
	}
}

func TestToolOutputIsClipped(t *testing.T) {
	big := strings.Repeat("é", maxToolBytes)
	got := clip(big)
	if len(got) > maxToolBytes+20 || !strings.HasSuffix(got, "[truncated]") || strings.ContainsRune(got, '�') {
		t.Errorf("clip gave %d bytes, valid utf8 %v", len(got), !strings.ContainsRune(got, '�'))
	}
	if clip("short") != "short" {
		t.Error("short text changed")
	}
}

func TestBackoffDoublesThenSucceeds(t *testing.T) {
	p := &fake{steps: []step{fails(429), fails(503), says("ok")}}
	h, slept := helper(t, map[string]*fake{"a/m": p}, "a/m")
	var notices strings.Builder
	h.Notice = &notices
	got, err := h.Ask(context.Background(), Query{Question: "q"}, nil)
	if err != nil || got != "ok" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if len(*slept) != 2 || (*slept)[0] != time.Second || (*slept)[1] != 2*time.Second {
		t.Errorf("slept %v, want 1s then 2s", *slept)
	}
	if strings.Count(notices.String(), "Retrying in") != 2 {
		t.Errorf("notices %q", notices.String())
	}
}

func TestFallbackModelAfterRetriesRunOut(t *testing.T) {
	primary := &fake{steps: []step{fails(429), fails(429), fails(429)}}
	backup := &fake{steps: []step{says("from backup")}}
	h, slept := helper(t, map[string]*fake{"a/main": primary, "b/backup": backup}, "a/main", "b/backup")
	var notices strings.Builder
	h.Notice = &notices
	got, err := h.Ask(context.Background(), Query{Question: "q"}, nil)
	if err != nil || got != "from backup" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if len(primary.reqs) != attempts || len(*slept) != attempts-1 {
		t.Errorf("primary tried %d times, slept %v", len(primary.reqs), *slept)
	}
	if !strings.Contains(notices.String(), "Trying b/backup") {
		t.Errorf("notices %q", notices.String())
	}
	if cached, ok := h.Cache.Get("a/main", "q"); !ok || cached != "from backup" {
		t.Errorf("answer not cached under the requested model: %q %v", cached, ok)
	}
}

func TestOtherErrorsSkipTheRetriesButNotTheFallback(t *testing.T) {
	primary := &fake{steps: []step{fails(400)}}
	backup := &fake{steps: []step{says("b")}}
	h, slept := helper(t, map[string]*fake{"a/main": primary, "b/backup": backup}, "a/main", "b/backup")
	got, err := h.Ask(context.Background(), Query{Question: "q"}, nil)
	if err != nil || got != "b" || len(primary.reqs) != 1 || len(*slept) != 0 {
		t.Errorf("Ask = %q, %v; primary calls %d, slept %v", got, err, len(primary.reqs), *slept)
	}
}

func TestAllModelsFailing(t *testing.T) {
	h, _ := helper(t, map[string]*fake{"a/m": {steps: []step{fails(500), fails(500), fails(500)}}}, "a/m")
	if _, err := h.Ask(context.Background(), Query{Question: "q"}, nil); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v", err)
	}
	h, _ = helper(t, nil)
	if _, err := h.Ask(context.Background(), Query{Question: "q"}, nil); err == nil || !strings.Contains(err.Error(), "aiconfig") {
		t.Errorf("no models: err = %v", err)
	}
}

func TestNoRetryOnceTextWasShown(t *testing.T) {
	broken := step{events: []llm.Event{{Type: llm.EventTextDelta, TextDelta: "half"}, {Type: llm.EventError, Err: &llm.HTTPError{StatusCode: 503}}}}
	p := &fake{steps: []step{broken, says("whole")}}
	h, slept := helper(t, map[string]*fake{"a/m": p}, "a/m")
	if _, err := h.Ask(context.Background(), Query{Question: "q"}, nil); err == nil || len(p.reqs) != 1 || len(*slept) != 0 {
		t.Errorf("err = %v after %d calls, slept %v", err, len(p.reqs), *slept)
	}
}

func TestCancelledContextStopsTheFallbackChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &fake{steps: []step{fails(429)}}
	backup := &fake{steps: []step{says("b")}}
	h, _ := helper(t, map[string]*fake{"a/m": p, "b/m": backup}, "a/m", "b/m")
	h.Sleep = func(ctx context.Context, d time.Duration) error { cancel(); return ctx.Err() }
	if _, err := h.Ask(ctx, Query{Question: "q"}, nil); !errors.Is(err, context.Canceled) || len(backup.reqs) != 0 {
		t.Errorf("err = %v, backup called %d times", err, len(backup.reqs))
	}
}

func TestFixAnswersWithoutCacheOrPreamble(t *testing.T) {
	p := &fake{steps: []step{
		{events: []llm.Event{{Type: llm.EventTextDelta, TextDelta: "Let me check."}, {Type: llm.EventToolCall, Call: &llm.ToolCall{ID: "c", Name: "tldr_page", Args: `{"name":"zzz-none"}`}}}},
		says("pacman -S firefox\n\n-s is not an operation."),
	}}
	h, _ := helper(t, map[string]*fake{"a/m": p}, "a/m")
	h.Tools = Tools(Wiki{})
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".local/share/misa/tldr/en"), 0o755)
	cmd, why, err := h.Fix(context.Background(), "pacman -s firefox", "error: invalid option -- 's'", 1)
	if err != nil || cmd != "pacman -S firefox" || why != "-s is not an operation." {
		t.Fatalf("Fix = %q, %q, %v", cmd, why, err)
	}
	if !strings.Contains(p.reqs[0].Messages[0].Text, "Exit status: 1") || !strings.Contains(p.reqs[0].System, "Line 1: The corrected command") {
		t.Errorf("request = %+v", p.reqs[0])
	}
	if entries, _ := os.ReadDir(h.Cache.Dir); len(entries) != 0 {
		t.Errorf("fix was cached: %v", entries)
	}
	if _, _, err := h.Fix(context.Background(), "  ", "", 1); err == nil {
		t.Error("blank command accepted")
	}
}

func TestRetryable(t *testing.T) {
	for err, want := range map[error]bool{
		&llm.HTTPError{StatusCode: 429}:             true,
		&llm.HTTPError{StatusCode: 502}:             true,
		&llm.HTTPError{StatusCode: 401}:             false,
		errors.New("provider: HTTP 429: slow down"): true,
		errors.New("RESOURCE_EXHAUSTED"):            true,
		errors.New("dial tcp: refused"):             false,
	} {
		if Retryable(err) != want {
			t.Errorf("Retryable(%v) = %v", err, !want)
		}
	}
}

func TestNewUsesTheHelpModelAndFallback(t *testing.T) {
	aitest.Isolate(t)
	ai.SettingsPath = filepath.Join(t.TempDir(), "ai.yaml")
	t.Cleanup(func() { ai.SettingsPath = "" })
	if _, err := New(""); err == nil || !strings.Contains(err.Error(), "aiconfig") {
		t.Fatalf("no model: err = %v", err)
	}
	if Available() {
		t.Error("Available with no model")
	}
	if err := ai.SaveSettings(ai.Settings{Models: map[string]string{"help": "a/one", FallbackKey: "b/two"}}); err != nil {
		t.Fatal(err)
	}
	h, err := New("")
	if err != nil || strings.Join(h.Models, ",") != "a/one,b/two" || !Available() {
		t.Fatalf("New = %+v, %v", h, err)
	}
	h, _ = New("c/three")
	if strings.Join(h.Models, ",") != "c/three,b/two" {
		t.Errorf("--model: %v", h.Models)
	}
	h, _ = New("b/two")
	if strings.Join(h.Models, ",") != "b/two" {
		t.Errorf("fallback equal to the model repeated: %v", h.Models)
	}
}
