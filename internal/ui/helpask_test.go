package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func withAsker(t *testing.T, fn func(command, question, page string) (string, error)) {
	t.Helper()
	old := aiAsker
	aiAsker = fn
	t.Cleanup(func() { aiAsker = old })
}

func helpModel(t *testing.T) *Model {
	t.Helper()
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.mux.primaryW, m.mux.primaryH = 80, 24
	m.showHelpPage("tar", "# tar\n\n> Archive files.\n")
	return m
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestAskAIInTheHelpViewer(t *testing.T) {
	var gotCommand, gotQuestion, gotPage string
	withAsker(t, func(command, question, page string) (string, error) {
		gotCommand, gotQuestion, gotPage = command, question, page
		return "Use `tar -xzf`.", nil
	})
	m := helpModel(t)
	baseHeight := m.helpView.Height()
	if !strings.Contains(m.View().Content, "a: ask AI") && !strings.Contains(m.helpView.View(), "a: ask AI") {
		t.Error("the footer does not offer the AI")
	}

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if !m.helpAsk.prompting || m.helpView.Height() != baseHeight-1 || !strings.Contains(m.helpPromptView(), "Ask AI:") {
		t.Fatalf("prompt not open: %+v height %d", m.helpAsk.prompting, m.helpView.Height())
	}
	typeText(m, "how to extract")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || m.helpAsk.prompting || m.helpView.Height() != baseHeight {
		t.Fatalf("question not sent: cmd=%v prompting=%v height=%d", cmd != nil, m.helpAsk.prompting, m.helpView.Height())
	}
	if !strings.Contains(m.helpAsk.transcript, "asking AI...") {
		t.Errorf("no progress shown: %q", m.helpAsk.transcript)
	}
	msg := cmd()
	if gotCommand != "tar" || gotQuestion != "how to extract" || !strings.Contains(gotPage, "Archive files.") {
		t.Errorf("asked %q %q %q", gotCommand, gotQuestion, gotPage)
	}
	m.Update(msg)
	if m.helpAsk.busy || strings.Contains(m.helpAsk.transcript, "asking AI...") ||
		!strings.Contains(m.helpAsk.transcript, "Q: how to extract") || !strings.Contains(m.helpAsk.transcript, "tar -xzf") {
		t.Errorf("transcript %q", m.helpAsk.transcript)
	}
	if m.screen != screenHelp {
		t.Errorf("left the help screen: %v", m.screen)
	}
}

func TestAskAIEscapeAndEmptyQuestion(t *testing.T) {
	withAsker(t, func(string, string, string) (string, error) { t.Error("asked"); return "", nil })
	m := helpModel(t)
	h := m.helpView.Height()
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || m.helpAsk.prompting || m.helpView.Height() != h {
		t.Error("an empty question was sent or the prompt stayed")
	}
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	typeText(m, "q")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.helpAsk.prompting || m.screen != screenHelp || m.helpView.Height() != h {
		t.Errorf("esc: prompting=%v screen=%v", m.helpAsk.prompting, m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	typeText(m, "q") // letters like q and a go to the prompt, not the viewer
	if m.screen != screenHelp || m.helpAsk.input.Value() != "q" {
		t.Errorf("typing in the prompt: screen=%v value=%q", m.screen, m.helpAsk.input.Value())
	}
}

func TestAskAIErrorAndStaleAnswer(t *testing.T) {
	withAsker(t, func(string, string, string) (string, error) { return "", errors.New("run aiconfig") })
	m := helpModel(t)
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	typeText(m, "x")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(cmd())
	if !strings.Contains(m.helpAsk.transcript, "AI: run aiconfig") {
		t.Errorf("transcript %q", m.helpAsk.transcript)
	}

	m.showHelpPage("ls", "# ls\n")
	m.helpAsk.busy = true
	m.Update(helpAnswerMsg{command: "tar", question: "x", answer: "stale"})
	if strings.Contains(m.helpAsk.transcript, "stale") {
		t.Error("an answer about another page was shown")
	}
}

func TestAskAIWithoutAISupport(t *testing.T) {
	withAsker(t, nil)
	m := helpModel(t)
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.helpAsk.prompting || !strings.Contains(m.helpAsk.transcript, "built without AI support") {
		t.Errorf("transcript %q", m.helpAsk.transcript)
	}
}
