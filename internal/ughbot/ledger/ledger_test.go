package ledger

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

func newLedger(t *testing.T) (*Ledger, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "l.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st), st
}

func priced(cost float64) llm.Estimate {
	return llm.Estimate{Ref: "p/m", InputTokens: 100, MaxOutputTokens: 100, Cost: cost, Priced: true}
}

func TestHookRefusesOverCapWithWrappedError(t *testing.T) {
	l, _ := newLedger(t)
	hook := l.Hook("a", "main", Caps{Daily: 1})
	rel, err := hook(priced(0.6))
	if err != nil {
		t.Fatal(err)
	}
	rel(llm.Usage{In: 10, Out: 5, Cost: 0.6})
	if _, err := hook(priced(0.6)); !errors.Is(err, store.ErrOverCap) {
		t.Fatalf("err = %v, want ErrOverCap", err)
	}
}

func TestReleaseWithoutUsageFreesTheReservation(t *testing.T) {
	l, st := newLedger(t)
	hook := l.Hook("a", "main", Caps{Daily: 1})
	rel, err := hook(priced(0.9))
	if err != nil {
		t.Fatal(err)
	}
	rel(llm.Usage{})
	rel(llm.Usage{Cost: 5}) // a second release is ignored
	if _, err := hook(priced(0.9)); err != nil {
		t.Fatalf("reservation not released: %v", err)
	}
	if rows, _ := st.CostByDay("", "", ""); len(rows) != 0 {
		t.Fatalf("settled rows = %v", rows)
	}
}

func TestUnpricedRequestsPassAtZeroAndRecordReportedCost(t *testing.T) {
	l, st := newLedger(t)
	hook := l.Hook("a", "main", Caps{Daily: 0.5})
	rel, err := hook(llm.Estimate{Ref: "codex/x"})
	if err != nil {
		t.Fatal(err)
	}
	rel(llm.Usage{In: 1, Out: 1})
	rel2, err := hook(llm.Estimate{Ref: "codex/x"})
	if err != nil {
		t.Fatalf("unpriced refused under cap: %v", err)
	}
	rel2(llm.Usage{Cost: 0.7})
	if _, err := hook(llm.Estimate{Ref: "codex/x"}); !errors.Is(err, store.ErrOverCap) {
		t.Fatalf("over cap should refuse even unpriced: %v", err)
	}
	if rows, _ := st.CostByDay("a", "", ""); len(rows) != 1 || rows[0].USD != 0.7 {
		t.Fatalf("rows = %v", rows)
	}
}

func TestConcurrentBotsNeverExceedCaps(t *testing.T) {
	l, st := newLedger(t)
	caps := Caps{Daily: 1}
	var wg sync.WaitGroup
	var granted [3]atomic.Int64
	for b, bot := range []string{"x", "y", "z"} {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rel, err := l.Hook(bot, "main", caps)(priced(0.25))
				if err != nil {
					return
				}
				granted[b].Add(1)
				rel(llm.Usage{In: 1, Out: 1, Cost: 0.25})
			}()
		}
	}
	wg.Wait()
	for b := range granted {
		if n := granted[b].Load(); n != 4 {
			t.Errorf("bot %d granted %d requests, want 4", b, n)
		}
	}
	rows, _ := st.CostByDay("", "", "")
	for _, r := range rows {
		if r.USD > 1 {
			t.Errorf("%s spent %.2f over the cap", r.Bot, r.USD)
		}
	}
}

func TestLimitStopsATaskAtItsSlice(t *testing.T) {
	l, _ := newLedger(t)
	hook := Limit(0.5, l.Hook("a", "t1", Caps{Daily: 10}))
	rel, err := hook(priced(0.3))
	if err != nil {
		t.Fatal(err)
	}
	rel(llm.Usage{Cost: 0.3})
	if _, err := hook(priced(0.3)); !errors.Is(err, store.ErrOverCap) {
		t.Fatalf("second request err = %v", err)
	}
}

func TestRemainingUsesTheTighterCap(t *testing.T) {
	l, _ := newLedger(t)
	caps := Caps{Daily: 1, Monthly: 10}
	id, _ := l.Reserve("a", "", "m", 0.4, caps)
	_ = l.Settle(id, 0.4)
	left, limited, err := l.Remaining("a", caps)
	if err != nil || !limited || left < 0.599 || left > 0.601 {
		t.Fatalf("left=%v limited=%v err=%v", left, limited, err)
	}
	if _, limited, _ := l.Remaining("a", Caps{}); limited {
		t.Fatal("no caps should be unlimited")
	}
}
