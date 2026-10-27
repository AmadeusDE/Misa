// Package ledger gates every model request of a bot against its spend caps.
package ledger

import (
	"fmt"
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

type Caps struct {
	Daily, Monthly float64 // <= 0 is unlimited
}

// Hook has the shape of agent.Config.BeforeRequest.
type Hook func(llm.Estimate) (release func(llm.Usage), err error)

type Ledger struct {
	mu sync.Mutex
	st *store.Store
}

func New(st *store.Store) *Ledger { return &Ledger{st: st} }

func (l *Ledger) Reserve(bot, task, model string, est float64, caps Caps) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.st.Reserve(bot, task, model, est, caps.Daily, caps.Monthly)
}

func (l *Ledger) Settle(id int64, actual float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.st.Settle(id, actual)
}

func (l *Ledger) Release(id int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.st.Release(id)
}

// Remaining is the room left under the tighter of the daily and monthly caps;
// limited is false when neither cap is set.
func (l *Ledger) Remaining(bot string, caps Caps) (left float64, limited bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for _, c := range []struct {
		cap   float64
		since time.Time
	}{{caps.Daily, day}, {caps.Monthly, month}} {
		if c.cap <= 0 {
			continue
		}
		spent, err := l.st.SpendSince(bot, c.since)
		if err != nil {
			return 0, false, err
		}
		if room := max(c.cap-spent, 0); !limited || room < left {
			left, limited = room, true
		}
	}
	return left, limited, nil
}

// Hook reserves each request's worst-case price before it is sent and settles it
// with the usage the provider reported. Unpriced requests (Codex subscriptions,
// unknown models) reserve nothing but still record any cost they report, and are
// refused only once the bot is already over a cap.
func (l *Ledger) Hook(bot, task string, caps Caps) Hook {
	return func(est llm.Estimate) (func(llm.Usage), error) {
		reserve := 0.0
		if est.Priced {
			reserve = est.Cost
		}
		id, err := l.Reserve(bot, task, est.Ref, reserve, caps)
		if err != nil {
			return nil, fmt.Errorf("%s over its spend cap (daily %.2f, monthly %.2f): %w", bot, caps.Daily, caps.Monthly, err)
		}
		var once sync.Once
		return func(u llm.Usage) {
			once.Do(func() { l.close(id, est, u) })
		}, nil
	}
}

func (l *Ledger) close(id int64, est llm.Estimate, u llm.Usage) {
	switch {
	case u.Cost > 0:
		_ = l.Settle(id, u.Cost)
	case !est.Priced || u.In+u.Out == 0:
		_ = l.Release(id)
	default:
		_ = l.Settle(id, est.Cost*float64(u.In+u.Out)/float64(max(est.InputTokens+est.MaxOutputTokens, 1)))
	}
}

// Limit caps what one task may spend through inner, on top of the bot's own caps;
// limit <= 0 passes inner through.
func Limit(limit float64, inner Hook) Hook {
	if limit <= 0 {
		return inner
	}
	var mu sync.Mutex
	var spent float64
	add := func(d float64) { mu.Lock(); spent += d; mu.Unlock() }
	return func(est llm.Estimate) (func(llm.Usage), error) {
		mu.Lock()
		over := spent+est.Cost > limit
		if !over {
			spent += est.Cost
		}
		mu.Unlock()
		if over {
			return nil, fmt.Errorf("task budget $%.2f spent: %w", limit, store.ErrOverCap)
		}
		release, err := inner(est)
		if err != nil {
			add(-est.Cost)
			return nil, err
		}
		return func(u llm.Usage) {
			release(u)
			add(u.Cost - est.Cost)
		}, nil
	}
}
