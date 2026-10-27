package tui

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
)

// Assistant text stayed "live" (dim, unrendered) until the whole run ended,
// and the status line said "thinking" whenever nothing else was set.
func TestAssistantTextFinalizesPerTurn(t *testing.T) {
	s := testState(t)
	s.running = true
	s.applyEvent(agent.Event{Kind: "thinking", Text: "hmm"})
	s.applyEvent(agent.Event{Kind: "text", Text: "Spawning the lanes."})
	if s.phase != "writing" || s.entries[len(s.entries)-2].live {
		t.Fatalf("phase=%q thinking still live=%v", s.phase, s.entries[len(s.entries)-2].live)
	}
	s.applyEvent(agent.Event{Kind: "usage"})
	last := s.entries[len(s.entries)-1]
	if last.kind != "assistant" || last.live {
		t.Fatalf("assistant text still live after the turn: %+v", last)
	}
	if s.phase != "" {
		t.Fatalf("phase = %q after the turn", s.phase)
	}
}
