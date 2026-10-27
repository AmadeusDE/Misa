package usage

import (
	"bufio"
	"encoding/json"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The ledger sums API-reported cost per provider across ugh processes
// (~/.local/state/ugh/cost.jsonl). It is the fallback usage source for
// providers whose account usage cannot be queried, e.g. a BYOK connection.

type ledgerEntry struct {
	Provider string    `json:"p"`
	Cost     float64   `json:"c"`
	At       time.Time `json:"t"`
}

var (
	ledgerMu   sync.Mutex
	ledgerPath = defaultLedgerPath
)

func defaultLedgerPath() string { return stateFile(roots.Roots{}, "cost.jsonl") }

func stateFile(r roots.Roots, name string) string {
	p, err := r.StateFile(name)
	if err != nil {
		return ""
	}
	return p
}

// Ledger is one instance's cost and route logs. The zero Roots give the
// user's shared ugh logs; explicit roots give private ones.
type Ledger struct {
	costPath, routesPath func() string
}

var (
	ledgersMu sync.Mutex
	ledgers   = map[roots.Roots]*Ledger{}
)

// NewLedger returns the one Ledger writing under r.
func NewLedger(r roots.Roots) *Ledger {
	if !r.Explicit() {
		return shared
	}
	ledgersMu.Lock()
	defer ledgersMu.Unlock()
	if l, ok := ledgers[r]; ok {
		return l
	}
	l := &Ledger{
		costPath:   func() string { return stateFile(r, "cost.jsonl") },
		routesPath: func() string { return stateFile(r, "routes.jsonl") },
	}
	ledgers[r] = l
	return l
}

var shared = &Ledger{
	costPath:   func() string { return ledgerPath() },
	routesPath: func() string { return routesPath() },
}

// Record appends one reported cost (USD). Failures are ignored: accounting
// must never break a turn.
func Record(provider string, cost float64) { shared.Record(provider, cost) }

// Record appends one reported cost (USD).
func (l *Ledger) Record(provider string, cost float64) {
	if provider == "" || cost <= 0 {
		return
	}
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	path := l.costPath()
	if path == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(ledgerEntry{provider, cost, time.Now().UTC()})
	_, _ = f.Write(append(b, '\n'))
}

// Spent totals the recorded cost for a provider and the number of calls.
func Spent(provider string) (total float64, calls int) { return shared.Spent(provider) }

// Spent totals the recorded cost for a provider and the number of calls.
func (l *Ledger) Spent(provider string) (total float64, calls int) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	f, err := os.Open(l.costPath())
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e ledgerEntry
		if json.Unmarshal([]byte(strings.TrimSpace(sc.Text())), &e) == nil && e.Provider == provider {
			total += e.Cost
			calls++
		}
	}
	return total, calls
}
