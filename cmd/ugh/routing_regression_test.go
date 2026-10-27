package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
)

func TestServingRouteInWatchTreeAndLog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	const wanted = "codex/gpt-6.1-sol"
	const actual = "opencode-go/mimo-v2.6-pro"
	s, err := session.Create(t.TempDir(), session.Record{Model: wanted, Agent: "root/worker-1", Run: "route-test"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ev := session.Record{Type: "route_fallback", Meta: map[string]any{"from": wanted, "to": actual, "reason": "admission pool full"}}
	msg := llm.Message{Role: llm.RoleAssistant, Route: actual, Text: "working"}
	for _, r := range []session.Record{ev, {Type: "message", Message: &msg}, {Type: "compaction", Seed: []llm.Message{{Role: llm.RoleUser, Text: "summary"}}}} {
		if err := s.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	ta := &tail{path: s.Path()}
	var log strings.Builder
	if err := ta.poll(func(r session.Record) { printRecord(&log, r, false) }); err != nil {
		t.Fatal(err)
	}
	// Even an old/stale result naming the requested model cannot override a turn.
	ta.apply(session.Record{Type: "result", Model: wanted, Status: "ok"})
	var watch strings.Builder
	renderWatch(&watch, "route-test", map[string]*tail{s.Path(): ta}, time.Now(), 300)
	for _, want := range []string{"mimo-v2.6-pro (wanted gpt-6.1-sol)", "fallback: " + wanted + " → " + actual + " (admission pool full)"} {
		if !strings.Contains(watch.String(), want) {
			t.Errorf("watch missing %q:\n%s", want, watch.String())
		}
	}
	for _, want := range []string{"route: " + actual, "fallback: " + wanted + " → " + actual + " (admission pool full)"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log missing %q:\n%s", want, log.String())
		}
	}
	base, _ := session.StateDir()
	paths, _ := filepath.Glob(filepath.Join(base, "*", "*.jsonl"))
	nodes, err := runNodes(paths, "route-test")
	if err != nil {
		t.Fatal(err)
	}
	if got := nodes["root/worker-1"].model; got != actual+" (wanted "+wanted+")" {
		t.Fatalf("tree model=%q", got)
	}
	// Legacy sessions without a Route still show the requested model.
	legacy := &tail{}
	legacy.apply(session.Record{Type: "meta", Model: wanted})
	legacy.apply(session.Record{Type: "message", Message: &llm.Message{Role: llm.RoleAssistant, Text: "old"}})
	if got := displayRoute(legacy.actual, legacy.meta.Model); got != "gpt-6.1-sol" {
		t.Fatalf("legacy model=%s", got)
	}
}

func TestRouteDisplayPreservesProviderSwitch(t *testing.T) {
	if got := displayRoute("new/model", "old/model"); got != "new/model (wanted old/model)" {
		t.Fatalf("display=%s", got)
	}
}
