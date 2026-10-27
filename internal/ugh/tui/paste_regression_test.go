package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

func TestPasteGoesIntoThePromptInput(t *testing.T) {
	s := testState(t)
	s.ta.Focus()
	s.Update(tea.PasteMsg{Content: "hello\nworld"})
	if got := s.ta.Value(); got != "hello\nworld" {
		t.Fatalf("input = %q, want the pasted text", got)
	}
}

func TestPasteNeverAnswersAPermissionPrompt(t *testing.T) {
	s := testState(t)
	s.ta.Focus()
	s.modal = &perm.UIRequest{}
	s.Update(tea.PasteMsg{Content: "y"})
	if s.modal == nil || s.ta.Value() != "" {
		t.Fatalf("paste changed state under a permission prompt: modal=%v input=%q", s.modal, s.ta.Value())
	}
}

func TestPasteFillsAnAskWriteIn(t *testing.T) {
	s := testState(t)
	it := s.asks.add(agent.AskRequest{Questions: []agent.AskQuestion{{Question: "Why?"}}})
	it.typing = true
	s.Update(tea.PasteMsg{Content: "my\r\nown"})
	if it.typed != "my own" || s.ta.Value() != "" {
		t.Fatalf("typed = %q, input = %q", it.typed, s.ta.Value())
	}
	it.typing = false
	s.Update(tea.PasteMsg{Content: "ignored"})
	if it.typed != "my own" || s.ta.Value() != "" {
		t.Fatalf("paste outside the write-in leaked: typed = %q, input = %q", it.typed, s.ta.Value())
	}
}
