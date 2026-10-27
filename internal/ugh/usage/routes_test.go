package usage

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRouteStatsTotalsAndCacheRate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.jsonl")
	old := routesPath
	routesPath = func() string { return path }
	t.Cleanup(func() { routesPath = old })
	RecordRoute("codex/gpt-6.1-sol", "root", 1000, 50, 900, 0)
	RecordRoute("openrouter/anthropic/claude-haiku-5.5", "worker", 2000, 100, 1000, 0.02)
	RecordRoute("openrouter/anthropic/claude-haiku-5.5", "helper", 2000, 100, 1000, 0.01)
	RecordRoute("codex/gpt-6.1-sol", "worker", 0, 0, 0, 0) // nothing to record
	stats := RouteStats(time.Now().Add(-time.Hour))
	if len(stats) != 2 || stats[0].Route != "openrouter/anthropic/claude-haiku-5.5" {
		t.Fatalf("stats = %+v", stats)
	}
	h, s := stats[0], stats[1]
	if h.Calls != 2 || h.DelegatedCalls != 2 || h.In != 4000 || h.CacheRate() != 0.5 || h.Cost < 0.0299 {
		t.Fatalf("haiku = %+v", h)
	}
	if s.Calls != 1 || s.RootCalls != 1 || s.CacheRate() != 0.9 {
		t.Fatalf("sol = %+v", s)
	}
	if len(RouteStats(time.Now().Add(time.Hour))) != 0 {
		t.Fatal("since is ignored")
	}
	table := RenderRoutes(stats, map[string][]string{"codex/gpt-6.1-sol": {"high"}})
	if !strings.Contains(table, "high") || !strings.Contains(table, "90%") || !strings.Contains(table, "$0.03") {
		t.Fatalf("table:\n%s", table)
	}
}
