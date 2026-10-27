package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

// ledger is a counting fake of a shared spend ledger: a reserve holds the
// estimate's cost until its release settles the actual cost.
type ledger struct {
	mu                sync.Mutex
	cap               float64
	reserved, spent   float64
	peak              float64
	reserves, release int
	refused           int
	estimates         []llm.Estimate
	actuals           []llm.Usage
}

func (l *ledger) hook(est llm.Estimate) (func(llm.Usage), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.estimates = append(l.estimates, est)
	if l.cap > 0 && l.spent+l.reserved+est.Cost > l.cap {
		l.refused++
		return nil, errors.New("daily cap reached")
	}
	l.reserves++
	l.reserved += est.Cost
	l.peak = max(l.peak, l.spent+l.reserved)
	return func(actual llm.Usage) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.release++
		l.reserved -= est.Cost
		l.spent += actual.Cost
		l.actuals = append(l.actuals, actual)
	}, nil
}

func (l *ledger) balanced(t *testing.T) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.reserves != l.release || l.reserved != 0 {
		t.Fatalf("reserves=%d releases=%d still reserved=%v", l.reserves, l.release, l.reserved)
	}
}

// pricedCatalog prices every model at 0 per input token and 1 per output
// token, so an attempt with MaxOutputTokens 100 estimates exactly 100.
func pricedCatalog(t *testing.T) {
	t.Helper()
	old := priceOf
	priceOf = func(string) (float64, float64, bool) { return 0, 1, true }
	t.Cleanup(func() { priceOf = old })
}

func budgetAgent(p llm.Provider, l *ledger) *Agent {
	return New(Config{Provider: p, Model: "p/m", Cwd: "/tmp", MaxTurns: 5, Trace: &strings.Builder{},
		MaxOutputTokens: 100, BeforeRequest: l.hook})
}

func TestBeforeRequestEstimateAndSettleOnSuccess(t *testing.T) {
	pricedCatalog(t)
	l := &ledger{}
	p := &funcProvider{script: func(int, llm.Request) []llm.Event {
		return []llm.Event{{Type: llm.EventUsage, Usage: &llm.Usage{In: 10, Out: 5, Cost: 0.25}}, say("ok")}
	}}
	a := budgetAgent(p, l)
	a.tokPerByte = 0.5
	if _, err := a.Run(context.Background(), strings.Repeat("x", 400)); err != nil {
		t.Fatal(err)
	}
	l.balanced(t)
	est := l.estimates[0]
	if est.MaxOutputTokens != 100 || est.Cost != 100 || !est.Priced || est.Ref != "p/m" {
		t.Errorf("estimate = %+v", est)
	}
	if est.InputTokens < 200 || est.InputTokens > 260 { // 400 prompt bytes plus overhead at 0.5 tokens per byte
		t.Errorf("input tokens = %d", est.InputTokens)
	}
	if len(l.actuals) != 1 || l.actuals[0].Cost != 0.25 || l.actuals[0].Out != 5 {
		t.Errorf("settled with %+v", l.actuals)
	}
}

func TestBeforeRequestUnpricedModelStillReserves(t *testing.T) {
	old := priceOf
	priceOf = func(string) (float64, float64, bool) { return 0, 0, false }
	t.Cleanup(func() { priceOf = old })
	l := &ledger{}
	a := budgetAgent(&funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("ok")} }}, l)
	if _, err := a.Run(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if e := l.estimates[0]; e.Priced || e.Cost != 0 || e.InputTokens == 0 {
		t.Errorf("estimate = %+v", e)
	}
	l.balanced(t)
}

func TestBeforeRequestReleasedOnStreamError(t *testing.T) {
	pricedCatalog(t)
	l := &ledger{}
	p := &funcProvider{script: func(int, llm.Request) []llm.Event {
		return []llm.Event{say("part"), {Type: llm.EventError, Err: errors.New("boom")}}
	}}
	if _, err := budgetAgent(p, l).Run(context.Background(), "x"); err == nil {
		t.Fatal("want the stream error")
	}
	l.balanced(t)
	if l.reserves == 0 {
		t.Fatal("never reserved")
	}
}

// blockingProvider streams nothing until its context is cancelled.
type blockingProvider struct{ started chan struct{} }

func (p *blockingProvider) Stream(ctx context.Context, _ llm.Request) (<-chan llm.Event, error) {
	ch := make(chan llm.Event)
	go func() { close(p.started); <-ctx.Done(); close(ch) }()
	return ch, nil
}

func TestBeforeRequestReleasedOnCancel(t *testing.T) {
	pricedCatalog(t)
	l := &ledger{}
	p := &blockingProvider{started: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := budgetAgent(p, l).Run(ctx, "x"); done <- err }()
	<-p.started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not stop")
	}
	l.balanced(t)
	if l.reserves != 1 {
		t.Fatalf("reserves = %d", l.reserves)
	}
}

func failoverAgent(t *testing.T, l *ledger, first, second llm.Provider) *Agent {
	t.Helper()
	routingEnv(t)
	old := resolveRoute
	resolveRoute = func(ref string) (llm.Provider, string, error) { return second, "m2", nil }
	t.Cleanup(func() { resolveRoute = old })
	tree := &Tree{}
	node := tree.Child(tree.Root("root", "m", ""), "worker", "alpha/m1", t.TempDir())
	return New(Config{Provider: first, Model: "m1", Tree: tree, Node: node, Agent: node.ID, Cwd: t.TempDir(), MaxTurns: 5,
		Routes: []string{"alpha:m1", "beta:m2"}, Trace: &strings.Builder{}, MaxOutputTokens: 100, BeforeRequest: l.hook})
}

func TestBeforeRequestRunsOnEachRouteAttempt(t *testing.T) {
	pricedCatalog(t)
	l := &ledger{}
	bad := &funcProvider{script: func(int, llm.Request) []llm.Event {
		return []llm.Event{{Type: llm.EventError, Err: &llm.HTTPError{Provider: "alpha", StatusCode: 400, Body: "rejected"}}}
	}}
	good := &funcProvider{script: func(int, llm.Request) []llm.Event {
		return []llm.Event{{Type: llm.EventUsage, Usage: &llm.Usage{In: 1, Out: 1, Cost: 0.5}}, say("on beta")}
	}}
	final, err := failoverAgent(t, l, bad, good).Run(context.Background(), "x")
	if err != nil || final != "on beta" {
		t.Fatalf("final=%q err=%v", final, err)
	}
	l.balanced(t)
	if l.reserves != 2 || l.estimates[0].Ref != "alpha/m1" || l.estimates[1].Ref != "beta/m2" {
		t.Fatalf("reserves=%d estimates=%+v", l.reserves, l.estimates)
	}
	if l.spent != 0.5 {
		t.Errorf("spent = %v, want only the successful attempt's cost", l.spent)
	}
}

func TestBudgetRefusalStopsTurnWithoutFailover(t *testing.T) {
	pricedCatalog(t)
	l := &ledger{cap: 50} // one attempt estimates 100
	first := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("never")} }}
	second := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("never")} }}
	_, err := failoverAgent(t, l, first, second).Run(context.Background(), "x")
	if !errors.Is(err, ErrBudget) || ClassifyFailure(err) != FailBudget {
		t.Fatalf("err = %v", err)
	}
	var be *BudgetError
	if !errors.As(err, &be) || be.Estimate.Cost != 100 || !strings.Contains(err.Error(), "budget exhausted: daily cap reached") {
		t.Fatalf("err = %v (%+v)", err, be)
	}
	if first.n != 0 || second.n != 0 || l.estimates[len(l.estimates)-1].Ref != "alpha/m1" || len(l.estimates) != 1 {
		t.Errorf("provider calls %d/%d, estimates %+v: a refusal must not fail over", first.n, second.n, l.estimates)
	}
	l.balanced(t)
}

func TestBudgetRefusalOnFinalTurnIsNotRetriedWithTools(t *testing.T) {
	pricedCatalog(t)
	l := &ledger{cap: 50}
	p := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("never")} }}
	a := New(Config{Provider: p, Model: "p/m", Cwd: t.TempDir(), MaxTurns: 1, Trace: &strings.Builder{},
		Tools: tools.NewRegistry(tools.All()...), MaxOutputTokens: 100, BeforeRequest: l.hook})
	if _, err := a.Run(context.Background(), "x"); !errors.Is(err, ErrBudget) {
		t.Fatalf("err = %v", err)
	}
	if l.refused != 1 {
		t.Errorf("refused %d times, want one (no tools-offered retry)", l.refused)
	}
}

func TestSharedLedgerNeverExceedsCap(t *testing.T) {
	pricedCatalog(t)
	const agents, cap = 12, 450 // an attempt estimates 100 and settles at 40
	l := &ledger{cap: cap}
	p := &funcProvider{script: func(int, llm.Request) []llm.Event {
		time.Sleep(2 * time.Millisecond)
		return []llm.Event{{Type: llm.EventUsage, Usage: &llm.Usage{In: 1, Out: 1, Cost: 40}}, say("ok")}
	}}
	var wg sync.WaitGroup
	var mu sync.Mutex
	done, refused := 0, 0
	for i := 0; i < agents; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := budgetAgent(p, l).Run(context.Background(), "x")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				done++
			case errors.Is(err, ErrBudget):
				refused++
			default:
				t.Errorf("err = %v", err)
			}
		}()
	}
	wg.Wait()
	l.balanced(t)
	if l.peak > cap {
		t.Fatalf("peak committed %v exceeded the cap %d", l.peak, cap)
	}
	if done == 0 || refused == 0 || done+refused != agents {
		t.Errorf("done=%d refused=%d: the cap should admit some and refuse some", done, refused)
	}
}
