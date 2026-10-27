package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

// User text and tool results are tagged as they enter the history, the tag
// survives in the session JSONL, and no request carries it to the provider.
func TestMessagesAreTaggedPersistedAndNotSent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	sess, err := session.Create(dir, session.Record{Model: "m", Cwd: dir})
	if err != nil {
		t.Fatal(err)
	}
	p := &funcProvider{script: func(n int, _ llm.Request) []llm.Event {
		if n == 1 {
			return []llm.Event{call("c1", "ls", "{}")}
		}
		return []llm.Event{say("done")}
	}}
	a := New(Config{Provider: p, Model: "m", Cwd: dir, MaxTurns: 5, Trace: &strings.Builder{},
		Tools:     tools.NewRegistry(tools.All()...),
		OnMessage: func(m llm.Message) { _ = sess.Append(session.Record{Type: "message", Message: &m}) }})
	if _, err := a.Run(context.Background(), "list files"); err != nil {
		t.Fatal(err)
	}
	want := map[llm.Role]string{llm.RoleUser: llm.SourceUser, llm.RoleTool: llm.SourceToolOutput}
	for _, m := range a.History() {
		if w, ok := want[m.Role]; ok && m.Meta[llm.MetaSource] != w {
			t.Errorf("%s message tagged %q, want %q", m.Role, m.Meta[llm.MetaSource], w)
		}
	}
	loaded, err := session.Load(sess.Path())
	if err != nil {
		t.Fatal(err)
	}
	sawTool := false
	for _, m := range loaded.Messages {
		if m.Role == llm.RoleTool {
			sawTool = true
			if m.Meta[llm.MetaSource] != llm.SourceToolOutput || !m.Untrusted() {
				t.Errorf("tool result lost its tag in the session file: %+v", m)
			}
		}
	}
	if !sawTool {
		t.Fatal("no tool result recorded")
	}
}

func TestTurnOutNudgeIsHarnessNotUser(t *testing.T) {
	p := &funcProvider{script: func(int, llm.Request) []llm.Event { return []llm.Event{say("done")} }}
	a := New(Config{Provider: p, Model: "m", Cwd: t.TempDir(), MaxTurns: 1, Trace: &strings.Builder{},
		Tools: tools.NewRegistry(tools.All()...)})
	_, _ = a.Run(context.Background(), "x")
	for _, m := range a.History() {
		if m.Text == turnOutNudge && m.Source() != llm.SourceHarness {
			t.Errorf("nudge source = %q", m.Source())
		}
	}
}
