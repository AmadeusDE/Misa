package tui

import (
	"context"
	"slices"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// toolsProvider records the tool names offered in each request.
type toolsProvider struct{ seen [][]string }

func (p *toolsProvider) Stream(_ context.Context, req llm.Request) (<-chan llm.Event, error) {
	var names []string
	for _, t := range req.Tools {
		names = append(names, t.Name)
	}
	p.seen = append(p.seen, names)
	ch := make(chan llm.Event, 2)
	ch <- llm.Event{Type: llm.EventTextDelta, TextDelta: "ok"}
	ch <- llm.Event{Type: llm.EventDone, Stop: "stop"}
	close(ch)
	return ch, nil
}

func TestRootAgentCanAskTheUser(t *testing.T) {
	rec := &toolsProvider{}
	s := embeddedState(t, rec)
	if _, err := s.agent.Run(context.Background(), "q"); err != nil {
		t.Fatal(err)
	}
	if len(rec.seen) == 0 || !slices.Contains(rec.seen[0], "ask_user") {
		t.Fatalf("root agent tools = %v, want ask_user among them", rec.seen)
	}
}
