package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/provstate"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
)

func TestRouteFallbackEventsAndLiveStatus(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		failure      error
		disabled     bool
	}{
		{name: "quota", reason: "quota", failure: &llm.HTTPError{Provider: "codex", StatusCode: 402, Body: "secret-key"}},
		{name: "transient", reason: "transient", failure: &llm.HTTPError{Provider: "codex", StatusCode: 503, Body: "secret-key"}},
		{name: "ineligible", reason: "ineligible: provider disabled", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routingEnv(t)
			oldBackoff := retryBackoff
			retryBackoff = nil
			defer func() { retryBackoff = oldBackoff }()
			codex := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{{Type: llm.EventError, Err: tc.failure}} }}
			mimo := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("done")} }}
			old := resolveRoute
			resolveRoute = func(string) (llm.Provider, string, error) { return mimo, "mimo-v2.6-pro", nil }
			defer func() { resolveRoute = old }()
			if tc.disabled {
				provstate.Mark("codex", provstate.State{Kind: "disabled"})
			}
			tree := &Tree{}
			root := tree.Root("root", "m", "")
			node := tree.Child(root, "worker", "codex/gpt-6.1-sol", t.TempDir())
			store, err := session.Create(node.Cwd, session.Record{Model: node.Model})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			var records []session.Record
			a := New(Config{Provider: codex, Model: "gpt-6.1-sol", Tree: tree, Node: node, Agent: node.ID, Cwd: node.Cwd,
				Routes: []string{"codex:gpt-6.1-sol", "opencode-go:mimo-v2.6-pro"},
				OnRecord: func(r session.Record) {
					records = append(records, r)
					if err := store.Append(r); err != nil {
						t.Error(err)
					}
				},
				OnMessage: func(m llm.Message) {
					if err := store.Append(session.Record{Type: "message", Message: &m}); err != nil {
						t.Error(err)
					}
				},
			})
			if _, err := a.Run(context.Background(), "work"); err != nil {
				t.Fatal(err)
			}
			if len(records) != 1 || records[0].Type != "route_fallback" {
				t.Fatalf("events=%+v", records)
			}
			meta := records[0].Meta
			if meta["from"] != "codex/gpt-6.1-sol" || meta["to"] != "opencode-go/mimo-v2.6-pro" || meta["reason"] != tc.reason {
				t.Fatalf("fallback=%v", meta)
			}
			report := tree.SubtreeReport(root)
			if !strings.Contains(report, "opencode-go/mimo-v2.6-pro (wanted codex/gpt-6.1-sol)") {
				t.Fatalf("status=%s", report)
			}
			loaded, err := session.Load(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			if got := loaded.Messages[len(loaded.Messages)-1].Route; got != "opencode-go/mimo-v2.6-pro" {
				t.Fatalf("actual=%s", got)
			}
		})
	}
}

func TestInitialRouteResolutionFallbackIsRecorded(t *testing.T) {
	routingEnv(t)
	grant(t, "codex", "opencode-go")
	r := NewRuntime()
	r.Config.Routing.Tiers["low"] = config.RouteList{"opencode-go/mimo-v2.6-flash", "codex/gpt-6-luna"}
	old := resolveRoute
	resolveRoute = func(ref string) (llm.Provider, string, error) {
		if strings.HasPrefix(ref, "opencode-go/") {
			return nil, "", errors.New("unavailable secret-key")
		}
		return &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("done")} }}, "gpt-6-luna", nil
	}
	defer func() { resolveRoute = old }()
	parent := New(Config{Model: "m", Cwd: t.TempDir(), Tree: r.Tree})
	res := r.startChild(context.Background(), parent, Task{Task: "work", Role: "worker"})
	if res.Status != "ok" {
		t.Fatalf("child=%+v", res)
	}
	loaded, err := session.Load(res.Session)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Meta.Model != "opencode-go/mimo-v2.6-flash" {
		t.Fatalf("requested=%s", loaded.Meta.Model)
	}
	data, err := os.ReadFile(res.Session)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") {
		t.Fatal("raw resolver error leaked into session")
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		var rec session.Record
		if json.Unmarshal([]byte(line), &rec) == nil && rec.Type == "route_fallback" {
			found = true
			if rec.Meta["from"] != "opencode-go/mimo-v2.6-flash" || rec.Meta["to"] != "codex/gpt-6-luna" || rec.Meta["reason"] != "ineligible: route unavailable" {
				t.Fatalf("fallback=%v", rec.Meta)
			}
		}
	}
	if !found {
		t.Fatal("missing fallback event")
	}
	if !strings.Contains(r.Tree.StatusReport(), "codex/gpt-6-luna (wanted opencode-go/mimo-v2.6-flash)") {
		t.Fatal(r.Tree.StatusReport())
	}
}
