package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/admission"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
)

// lingeringProvider emits an error but keeps its producer alive until the
// request context is cancelled, then takes a while to wind down.
type lingeringProvider struct {
	stopped atomic.Int64 // unix nanos when the producer finished
}

func (p *lingeringProvider) Stream(ctx context.Context, _ llm.Request) (<-chan llm.Event, error) {
	ch := make(chan llm.Event)
	go func() {
		defer close(ch)
		select {
		case ch <- llm.Event{Type: llm.EventError, Err: errors.New("boom")}:
		case <-ctx.Done():
		}
		<-ctx.Done()
		time.Sleep(80 * time.Millisecond) // winding down after cancellation
		p.stopped.Store(time.Now().UnixNano())
	}()
	return ch, nil
}

func testAdmission(t *testing.T, prio admission.Priority) Admission {
	t.Helper()
	mgr := admission.NewManager(admission.WithBaseDir(t.TempDir()))
	if err := mgr.SetConfig("", admission.AccountConfig{CombinedLimit: 1}); err != nil {
		t.Fatal(err)
	}
	return Admission{Manager: mgr, Priority: prio, Purpose: "test"}
}

func TestAdmissionHeldUntilErroringProducerStops(t *testing.T) {
	ad := testAdmission(t, admission.PriorityChild)
	p := &lingeringProvider{}
	a := New(Config{Provider: p, Model: "gpt-6.1-sol", ProviderName: "codex", Admission: ad, Cwd: t.TempDir()})

	var firstFree atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
			if a.cfg.Node == nil {
				continue
			}
			h, err := ad.Manager.Acquire(context.Background(), admission.Request{Provider: "codex", Model: "gpt-6.1-sol", Priority: admission.PriorityChild})
			if err == nil {
				if firstFree.Load() == 0 {
					firstFree.Store(time.Now().UnixNano())
				}
				_ = h.Close()
				if p.stopped.Load() != 0 {
					return
				}
				continue
			}
		}
	}()
	if _, err := a.Run(context.Background(), "hi"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Run err = %v, want boom", err)
	}
	<-done
	stopped, free := p.stopped.Load(), firstFree.Load()
	if stopped == 0 || free == 0 {
		t.Fatalf("stopped=%d free=%d", stopped, free)
	}
	if free < stopped {
		t.Fatalf("admission released %s before the producer stopped", time.Duration(stopped-free))
	}
}

func TestChildCapacityFallsToNextRoute(t *testing.T) {
	ad := testAdmission(t, admission.PriorityChild)
	// Occupy the explicitly configured single slot.
	h, err := ad.Manager.Acquire(context.Background(), admission.Request{Provider: "codex", Model: "gpt-6.1-sol", Priority: admission.PriorityRoot})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	codex := &funcProvider{script: func(int, llm.Request) []llm.Event { t.Error("codex must not be reached"); return nil }}
	mimo := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("on mimo")} }}
	old := resolveRoute
	resolveRoute = func(ref string) (llm.Provider, string, error) {
		if strings.HasPrefix(ref, "opencode-go/") {
			return mimo, "mimo-v2.6-pro", nil
		}
		return codex, "gpt-6.1-sol", nil
	}
	defer func() { resolveRoute = old }()
	tree := &Tree{}
	node := tree.Child(tree.Root("root", "m", ""), "worker", "codex/gpt-6.1-sol", t.TempDir())
	var fallback session.Record
	a := New(Config{Provider: codex, Model: "gpt-6.1-sol", Tree: tree, Node: node, Agent: node.ID, Cwd: t.TempDir(),
		OnRecord: func(r session.Record) { fallback = r }, Admission: ad, Routes: []string{"codex:gpt-6.1-sol", "opencode-go:mimo-v2.6-pro"}, Trace: &strings.Builder{}})
	final, err := a.Run(context.Background(), "work")
	if err != nil || final != "on mimo" || node.Model != "opencode-go/mimo-v2.6-pro" {
		t.Fatalf("final=%q err=%v model=%s", final, err, node.Model)
	}
	if fallback.Type != "route_fallback" || fallback.Meta["reason"] != "admission pool full" {
		t.Fatalf("fallback=%+v", fallback)
	}
}

func TestNonCodexRootConsumesNoSlot(t *testing.T) {
	ad := testAdmission(t, admission.PriorityRoot)
	h, err := ad.Manager.Acquire(context.Background(), admission.Request{Provider: "codex", Model: "gpt-6.1-sol", Priority: admission.PriorityRoot})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	p := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("ok")} }}
	a := New(Config{Provider: p, Model: "openai/gpt-6.1-sol", ProviderName: "openrouter", Admission: ad, Cwd: t.TempDir()})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if final, err := a.Run(ctx, "hi"); err != nil || final != "ok" {
		t.Fatalf("final=%q err=%v", final, err)
	}
}

func TestChildAdmissionNeverInheritsExemption(t *testing.T) {
	parent := Admission{Priority: admission.PriorityRoot, Exemption: admission.ExemptionCouncilCoordinatorSol}
	c := childAdmission(parent, "delegated")
	if c.Exemption != admission.ExemptionNone || c.Priority != admission.PriorityChild {
		t.Fatalf("child admission = %+v", c)
	}
}

func TestAdaptHistoryDropsForeignReasoningKeepsToolCalls(t *testing.T) {
	calls := []llm.ToolCall{{ID: "c1", Name: "write", Args: `{"path":"a"}`}}
	hist := []llm.Message{
		{Role: llm.RoleUser, Text: "go"},
		{Role: llm.RoleAssistant, Text: "writing", ToolCalls: calls, Native: `[{"type":"reasoning","encrypted_content":"x"}]`, Reasoning: "think", Route: "codex/gpt-6.1-sol"},
		{Role: llm.RoleTool, ToolCallID: "c1", Text: "ok"},
	}
	got := adaptHistory(hist, "openrouter/anthropic/claude-sonnet-5.5")
	if got[1].Native != "" || got[1].Reasoning != "" || len(got[1].ToolCalls) != 1 || got[1].Text != "writing" {
		t.Fatalf("adapted turn = %+v", got[1])
	}
	if hist[1].Native == "" {
		t.Fatal("source history was modified")
	}
	if same := adaptHistory(hist, "codex/gpt-6.1-sol"); same[1].Native == "" {
		t.Fatal("same-route history must keep its native payload")
	}
}

// The root's model is the user's: a Sol root neither takes a premium slot
// nor waits for one, so it never starves delegated Sol work.
func TestRootTakesNoPremiumSlot(t *testing.T) {
	ad := testAdmission(t, admission.PriorityRoot)
	h, err := ad.Manager.Acquire(context.Background(), admission.Request{Provider: "codex", Model: "gpt-6.1-sol", Priority: admission.PriorityChild})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	p := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("ok")} }}
	a := New(Config{Provider: p, Model: "gpt-6.1-sol", ProviderName: "codex", Admission: ad, Cwd: t.TempDir()})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if final, err := a.Run(ctx, "hi"); err != nil || final != "ok" {
		t.Fatalf("final=%q err=%v (root must not wait on a full pool)", final, err)
	}
}

// Sol as the tier's only route with the pool full used to fail the child
// (and its parent respawned it in a loop). Now it is admitted once against
// the next preset's limit; past that it steps down to the low tier.
func TestLastSolRouteOverflowsThenStepsDown(t *testing.T) {
	ad := testAdmission(t, admission.PriorityChild)
	var held []admission.Handle
	t.Cleanup(func() {
		for _, h := range held {
			h.Close()
		}
	})
	sol := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("on sol")} }}
	luna := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("on luna")} }}
	old := resolveRoute
	resolveRoute = func(ref string) (llm.Provider, string, error) {
		if strings.HasSuffix(ref, "luna") {
			return luna, "gpt-6-luna", nil
		}
		return sol, "gpt-6.1-sol", nil
	}
	t.Cleanup(func() { resolveRoute = old })
	run := func() (string, string) {
		tree := &Tree{}
		node := tree.Child(tree.Root("root", "m", ""), "worker", "codex/gpt-6.1-sol", t.TempDir())
		a := New(Config{Provider: sol, Model: "gpt-6.1-sol", Tree: tree, Node: node, Agent: node.ID, Cwd: t.TempDir(), Admission: ad,
			Routes: []string{"codex:gpt-6.1-sol", "codex:gpt-6-luna"}, StepDown: 1, Trace: &strings.Builder{}})
		final, err := a.Run(context.Background(), "work")
		if err != nil {
			t.Fatal(err)
		}
		return final, node.Model
	}
	hold := func() error {
		h, err := ad.Manager.Acquire(context.Background(), admission.Request{Provider: "codex", Model: "gpt-6.1-sol", Overflow: true})
		if err == nil {
			held = append(held, h)
		}
		return err
	}
	if err := hold(); err != nil { // the configured single slot
		t.Fatal(err)
	}
	if final, model := run(); final != "on sol" || model != "codex/gpt-6.1-sol" {
		t.Fatalf("with overflow room: final=%q model=%s", final, model)
	}
	for hold() == nil { // fill the overflow limit too
	}
	if len(held) != admission.OverflowLimit(1) {
		t.Fatalf("held %d slots, want the overflow limit %d", len(held), admission.OverflowLimit(1))
	}
	if final, model := run(); final != "on luna" || model != "codex/gpt-6-luna" {
		t.Fatalf("pool and overflow full: final=%q model=%s", final, model)
	}
}

func boundaryAgent(t *testing.T, filler int) (*Agent, *funcProvider) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := &funcProvider{script: func(_ int, req llm.Request) []llm.Event {
		if req.System == CompactSystem {
			return []llm.Event{say("SUMMARY")}
		}
		return []llm.Event{say("task done")}
	}}
	a := New(Config{Provider: p, Model: "p/m", Cwd: t.TempDir(), MaxTurns: 5, Trace: &strings.Builder{},
		AutoCompact: 0.9, CompactThreshold: 10000})
	for i := 0; i < 10; i++ {
		a.hist = append(a.hist, llm.Message{Role: llm.RoleUser, Text: strings.Repeat("a", filler)},
			llm.Message{Role: llm.RoleAssistant, Text: strings.Repeat("b", filler)})
	}
	return a, p
}

// A finished task with the context past half the limit (but under it) compacts
// now, so the next task does not start near the limit.
func TestFinishedTaskCompactsPastHalfTheLimit(t *testing.T) {
	a, _ := boundaryAgent(t, 1500) // ~30 KB ≈ 8.6k tokens of a 10k limit
	if est := a.estimateTokens(a.hist, nil); est >= 10000 || est < 5000 {
		t.Fatalf("setup: estimate %d", est)
	}
	if final, err := a.Run(context.Background(), "next"); err != nil || final != "task done" {
		t.Fatalf("final=%q err=%v", final, err)
	}
	if a.Compactions != 1 || !strings.Contains(a.hist[0].Text, "SUMMARY") {
		t.Fatalf("compactions=%d first=%q", a.Compactions, a.hist[0].Text)
	}
	if a.hist[0].Source() != llm.SourceHarness {
		t.Errorf("seed source = %q", a.hist[0].Source())
	}
	if last := a.hist[len(a.hist)-1]; last.Text != "task done" {
		t.Errorf("the final answer must stay in the kept tail, got %q", last.Text)
	}
}

func TestFinishedTaskBelowHalfTheLimitKeepsHistory(t *testing.T) {
	a, p := boundaryAgent(t, 500) // ≈ 2.9k tokens
	if _, err := a.Run(context.Background(), "next"); err != nil {
		t.Fatal(err)
	}
	if a.Compactions != 0 || p.n != 1 {
		t.Fatalf("compactions=%d requests=%d", a.Compactions, p.n)
	}
}
