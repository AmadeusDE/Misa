package bot

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/fakellm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

type dir map[string]*Bot

func (d dir) Lookup(n string) (*Bot, bool) { b, ok := d[n]; return b, ok }
func (d dir) All() []*Bot {
	var out []*Bot
	for _, b := range d {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// pair builds alpha (with caps) and beta and gamma (none), wired to each other.
func pair(t *testing.T, caps string) (rig, dir) {
	r := newRig(t, caps, 0, "[bots.beta]\nmodel='fake/m'\ncapabilities={tasks=1}", "[bots.gamma]\nmodel='fake/m'")
	d := dir{}
	for _, n := range []string{"alpha", "beta", "gamma"} {
		d[n] = r.named(t, n, fakellm.Echo(0), d)
	}
	t.Cleanup(func() {
		for _, b := range d {
			b.Stop()
		}
	})
	return r, d
}

func call(t *testing.T, b *Bot, tool, args string) (string, error) {
	t.Helper()
	tl, ok := b.mainTools().Get(tool)
	if !ok {
		t.Fatalf("%s has no %s tool", b.name, tool)
	}
	return tl.Run(context.Background(), "", json.RawMessage(args))
}

func names(b *Bot) []string {
	var out []string
	for _, s := range b.mainTools().Specs() {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}

func TestToolsFollowCapabilities(t *testing.T) {
	cases := map[string][]string{
		``:                          {"reply"},
		`files="read"`:              {"find", "grep", "ls", "read", "reply"},
		`files="write", shell=true`: {"bash", "edit", "find", "grep", "ls", "read", "reply", "write"},
		`tasks=1`:                   {"reply", "start_task"},
		`message=["*"]`:             {"ask_bot", "list_bots", "message_bot", "reply"},
		`manage=["*"]`:              {"list_bots", "manage_bot", "reply", "request_repo"},
	}
	for caps, want := range cases {
		r := newRig(t, caps, 0)
		b := r.bot(t, fakellm.Echo(0))
		if got := names(b); !slices.Equal(got, want) {
			t.Errorf("%q: got %v want %v", caps, got, want)
		}
		b.Stop()
	}
}

func TestMessageIsCheckedWhenCalled(t *testing.T) {
	r, d := pair(t, `message=["beta"]`)
	out, err := call(t, d["alpha"], "message_bot", `{"to":"beta","text":"hi"}`)
	if err != nil || !strings.Contains(out, "queued for beta") {
		t.Fatalf("%q %v", out, err)
	}
	m, ok, _ := r.st.Claim("beta")
	if !ok || m.From != "alpha" || m.Kind != store.KindMail || mailPrompt(m) != "[mail from alpha]\nhi" {
		t.Fatalf("%+v", m)
	}
	for args, want := range map[string]string{
		`{"to":"gamma","text":"hi"}`: "may not message",
		`{"to":"alpha","text":"hi"}`: "yourself",
		`{"to":"beta","text":" "}`:   "text is required",
	} {
		if _, err := call(t, d["alpha"], "message_bot", args); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", args, err)
		}
	}
	if n, _ := r.st.Pending("gamma"); n != 0 {
		t.Fatal("refused mail was delivered")
	}
	// Even a tool instance built for a bot without the capability refuses.
	if _, err := (messageBot{d["gamma"]}).Run(context.Background(), "", json.RawMessage(`{"to":"beta","text":"x"}`)); err == nil {
		t.Fatal("gamma has no message capability")
	}
}

func TestAskTakesOneReply(t *testing.T) {
	r, d := pair(t, `message=["*"]`)
	if _, err := call(t, d["alpha"], "ask_bot", `{"to":"beta","question":"why?"}`); err != nil {
		t.Fatal(err)
	}
	q, _, _ := r.st.Claim("beta")
	if !strings.HasPrefix(mailPrompt(q), "[question #") {
		t.Fatalf("%q", mailPrompt(q))
	}
	args := `{"question":` + itoa(q.ID) + `,"text":"because"}`
	if _, err := call(t, d["beta"], "reply", args); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, d["beta"], "reply", args); err == nil || !strings.Contains(err.Error(), "already replied") {
		t.Fatalf("second reply: %v", err)
	}
	if _, err := call(t, d["gamma"], "reply", args); err == nil || !strings.Contains(err.Error(), "not a question asked of you") {
		t.Fatalf("someone else's question: %v", err)
	}
	a, ok, _ := r.st.Claim("alpha")
	if !ok || a.Kind != store.KindReply || !strings.Contains(mailPrompt(a), "[reply from beta to your question #"+itoa(q.ID)+"]") {
		t.Fatalf("%+v", a)
	}
}

func TestManage(t *testing.T) {
	r, d := pair(t, `manage=["beta"]`)
	if _, err := call(t, d["alpha"], "manage_bot", `{"bot":"gamma","action":"status"}`); err == nil {
		t.Fatal("gamma is not managed by alpha")
	}
	out, err := call(t, d["alpha"], "manage_bot", `{"bot":"beta","action":"status"}`)
	if err != nil || !strings.Contains(out, "beta: stopped, 0 mail queued, 0/1 tasks") {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := call(t, d["alpha"], "manage_bot", `{"bot":"beta","action":"steer","text":"focus"}`); err != nil {
		t.Fatal(err)
	}
	m, _, _ := r.st.Claim("beta")
	if mailPrompt(m) != "[steering from alpha, who manages you]\nfocus" {
		t.Fatalf("%q", mailPrompt(m))
	}
	out, _ = call(t, d["alpha"], "manage_bot", `{"bot":"beta","action":"interrupt"}`)
	if !strings.Contains(out, "not running") {
		t.Fatalf("%q", out)
	}
	if _, err := call(t, d["alpha"], "list_bots", `{}`); err != nil {
		t.Fatal(err)
	}
}

func TestManagedTaskReportsToTheManager(t *testing.T) {
	r, d := pair(t, `manage=["*"]`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d["beta"].Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, d["alpha"], "manage_bot", `{"bot":"gamma","action":"start_task","text":"x"}`); err == nil ||
		!strings.Contains(err.Error(), "gamma may not run tasks") {
		t.Fatalf("gamma has no tasks: %v", err)
	}
	out, err := call(t, d["alpha"], "manage_bot", `{"bot":"beta","action":"start_task","text":"dig"}`)
	if err != nil || !strings.Contains(out, "task:beta/t1") {
		t.Fatalf("%q %v", out, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m, ok, err := r.st.Claim("alpha")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			if m.From != "task:beta/t1" || !strings.Contains(m.Body, "echo: dig") {
				t.Fatalf("%+v", m)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no task result")
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestBotsThatReachOthersKnowTheirName(t *testing.T) {
	_, d := pair(t, `message=["beta"]`)
	if s, _ := d["alpha"].system(); !strings.Contains(s, `You are the bot named "alpha"`) {
		t.Fatalf("%q", s)
	}
	if s, _ := d["gamma"].system(); s != "" {
		t.Fatalf("a bot that reaches no one gets its prompt untouched: %q", s)
	}
}
