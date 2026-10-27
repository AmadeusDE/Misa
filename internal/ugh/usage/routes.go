package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The route log records every model request's token usage and cost by route
// (~/.local/state/ugh/routes.jsonl), across ugh processes, so tiers can be
// judged on what they actually cost and how well they cache.

type routeEntry struct {
	Route  string    `json:"r"`           // provider/model
	Role   string    `json:"a,omitempty"` // agent role: root, worker, …
	In     int       `json:"i"`
	Out    int       `json:"o"`
	Cached int       `json:"k,omitempty"`
	Cost   float64   `json:"c,omitempty"`
	At     time.Time `json:"t"`
}

var (
	routesMu   sync.Mutex
	routesPath = func() string {
		if testing.Testing() { // tests of other packages run real agents; keep them out of the user's log
			return ""
		}
		return filepath.Join(filepath.Dir(ledgerPath()), "routes.jsonl")
	}
)

// RecordRoute appends one request's usage. Failures are ignored: accounting
// must never break a turn.
func RecordRoute(route, role string, in, out, cached int, cost float64) {
	shared.RecordRoute(route, role, in, out, cached, cost)
}

// RecordRoute appends one request's usage.
func (l *Ledger) RecordRoute(route, role string, in, out, cached int, cost float64) {
	if route == "" || in+out == 0 && cost == 0 {
		return
	}
	routesMu.Lock()
	defer routesMu.Unlock()
	path := l.routesPath()
	if path == "" || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(routeEntry{route, role, in, out, cached, cost, time.Now().UTC()})
	_, _ = f.Write(append(b, '\n'))
}

// RouteStat is one route's totals over a period.
type RouteStat struct {
	Route                     string
	Calls, In, Out, Cached    int
	Cost                      float64
	RootCalls, DelegatedCalls int
}

// CacheRate is the share of input tokens served from the provider's cache.
func (s RouteStat) CacheRate() float64 {
	if s.In == 0 {
		return 0
	}
	return float64(s.Cached) / float64(s.In)
}

// RouteStats totals the route log since a time, most expensive first (then
// by input tokens).
func RouteStats(since time.Time) []RouteStat { return shared.RouteStats(since) }

// RouteStats totals this ledger's route log since a time.
func (l *Ledger) RouteStats(since time.Time) []RouteStat {
	routesMu.Lock()
	defer routesMu.Unlock()
	f, err := os.Open(l.routesPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	by := map[string]*RouteStat{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var e routeEntry
		if json.Unmarshal([]byte(strings.TrimSpace(sc.Text())), &e) != nil || e.At.Before(since) {
			continue
		}
		s := by[e.Route]
		if s == nil {
			s = &RouteStat{Route: e.Route}
			by[e.Route] = s
		}
		s.Calls++
		s.In += e.In
		s.Out += e.Out
		s.Cached += e.Cached
		s.Cost += e.Cost
		if e.Role == "root" {
			s.RootCalls++
		} else {
			s.DelegatedCalls++
		}
	}
	out := make([]RouteStat, 0, len(by))
	for _, s := range by {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cost != out[j].Cost {
			return out[i].Cost > out[j].Cost
		}
		return out[i].In > out[j].In
	})
	return out
}

// RenderRoutes is a table of route stats; tiers maps a route to the tiers
// that list it.
func RenderRoutes(stats []RouteStat, tiers map[string][]string) string {
	if len(stats) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-44s %-9s %6s %6s %9s %6s %8s %9s\n", "route", "tiers", "root", "deleg", "input", "cache", "output", "cost")
	var total RouteStat
	for _, s := range stats {
		t := strings.Join(tiers[s.Route], ",")
		if t == "" {
			t = "-"
		}
		fmt.Fprintf(&b, "%-44s %-9s %6d %6d %9s %5.0f%% %8s %9s\n", clip(s.Route, 44), t, s.RootCalls, s.DelegatedCalls,
			tokens(s.In), 100*s.CacheRate(), tokens(s.Out), dollars(s.Cost))
		total.In, total.Out, total.Cached, total.Cost = total.In+s.In, total.Out+s.Out, total.Cached+s.Cached, total.Cost+s.Cost
		total.RootCalls, total.DelegatedCalls = total.RootCalls+s.RootCalls, total.DelegatedCalls+s.DelegatedCalls
	}
	fmt.Fprintf(&b, "%-44s %-9s %6d %6d %9s %5.0f%% %8s %9s", "total", "", total.RootCalls, total.DelegatedCalls,
		tokens(total.In), 100*total.CacheRate(), tokens(total.Out), dollars(total.Cost))
	return b.String()
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func tokens(n int) string {
	switch {
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.0fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

func dollars(c float64) string {
	if c == 0 {
		return "-"
	}
	return fmt.Sprintf("$%.2f", c)
}
