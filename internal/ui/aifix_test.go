package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func withAIFixer(t *testing.T, ready bool, fn func(cmd, output string, exit int) (string, string, error)) {
	t.Helper()
	oldFix, oldReady := aiFixer, aiFixReady
	aiFixer, aiFixReady = fn, func() bool { return ready }
	t.Cleanup(func() { aiFixer, aiFixReady = oldFix, oldReady })
}

func bufferText(m *Model) string { return strings.Join(m.shellBuffer, "\n") }

func failedModel(t *testing.T, cmd string) *Model {
	t.Helper()
	t.Setenv("PATH", t.TempDir()) // no similar commands for thefuck's typo rules
	m := New(t.TempDir())
	m.nestedMode = true // panes hold the shell; the root forwards to them
	m.width, m.height = 80, 24
	m.shell.lastCmd = cmd
	return m
}

func TestAIFixIsOfferedNotAsked(t *testing.T) {
	asked := 0
	withAIFixer(t, true, func(string, string, int) (string, string, error) { asked++; return "", "", nil })
	m := failedModel(t, "frobnicate --wat")
	m.offerFix("usage: frobnicate", 2)
	if asked != 0 || m.aiFix.cmd != "frobnicate --wat" || m.thefuckFix != "" || !strings.Contains(bufferText(m), "ctrl+f: ask AI") {
		t.Errorf("asked=%d state=%+v fix=%q buffer=%q", asked, m.aiFix, m.thefuckFix, bufferText(m))
	}
}

func TestAIFixNotOfferedWithoutAModelOrOnSuccess(t *testing.T) {
	withAIFixer(t, false, func(string, string, int) (string, string, error) { return "x", "", nil })
	m := failedModel(t, "frobnicate")
	m.offerFix("", 1)
	if m.aiFix.cmd != "" || bufferText(m) != "" || m.askAIFix() != nil {
		t.Errorf("offered without a model: %+v %q", m.aiFix, bufferText(m))
	}
	withAIFixer(t, true, nil)
	m.offerFix("", 1)
	if m.aiFix.cmd != "" {
		t.Error("offered with no asker (nough)")
	}
	withAIFixer(t, true, func(string, string, int) (string, string, error) { return "x", "", nil })
	m.offerFix("", 0)
	if m.aiFix.cmd != "" {
		t.Error("offered for a command that succeeded")
	}
}

func TestPatternFixWinsOverTheAI(t *testing.T) {
	withAIFixer(t, true, func(string, string, int) (string, string, error) { t.Error("asked the AI"); return "", "", nil })
	m := failedModel(t, "mkdir a/b")
	m.offerFix("mkdir: cannot create directory: No such file or directory", 1)
	if m.thefuckFix != "mkdir -p a/b" || m.aiFix.cmd != "" || strings.Contains(bufferText(m), "ask AI") {
		t.Errorf("fix %q, AI state %+v, buffer %q", m.thefuckFix, m.aiFix, bufferText(m))
	}
}

func TestCtrlFAsksTheAIAndShowsTheAnswerAsTheFix(t *testing.T) {
	var gotCmd, gotOutput string
	var gotExit int
	withAIFixer(t, true, func(cmd, output string, exit int) (string, string, error) {
		gotCmd, gotOutput, gotExit = cmd, output, exit
		return "git push", "push, not psuh.\nSecond line.", nil
	})
	m := failedModel(t, "git psuh")
	m.offerFix("git: 'psuh' is not a git command.", 1)
	m.inputMode = true

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if !m.aiFix.busy || !strings.Contains(bufferText(m), "asking AI") {
		t.Fatalf("not asking: %+v %q", m.aiFix, bufferText(m))
	}
	if again := m.askAIFix(); again != nil {
		t.Error("a second ctrl+f asked again while busy")
	}
	msg := cmd()
	if gotCmd != "git psuh" || gotOutput != "git: 'psuh' is not a git command." || gotExit != 1 {
		t.Errorf("asked about %q %q %d", gotCmd, gotOutput, gotExit)
	}
	m.Update(msg)
	if m.thefuckFix != "git push" || m.aiFix.cmd != "" {
		t.Errorf("fix = %q, state %+v", m.thefuckFix, m.aiFix)
	}
	out := bufferText(m)
	for _, want := range []string{"fix → ", "git push", "[ctrl+f]", "push, not psuh.", "Second line."} {
		if !strings.Contains(out, want) {
			t.Errorf("buffer lacks %q:\n%s", want, out)
		}
	}
}

func TestAIFixErrorsAndStaleAnswers(t *testing.T) {
	withAIFixer(t, true, func(string, string, int) (string, string, error) { return "", "", errors.New("no model") })
	m := failedModel(t, "lss")
	m.offerFix("", 127)
	cmd := m.askAIFix()
	m.Update(cmd())
	if m.thefuckFix != "" || !strings.Contains(bufferText(m), "AI fix failed: no model") {
		t.Errorf("fix %q buffer %q", m.thefuckFix, bufferText(m))
	}

	m = failedModel(t, "lss")
	m.offerFix("", 127)
	m.shell.lastCmd = "ls"
	m.offerFix("", 0) // another command ran and passed
	m.Update(aiFixMsg{cmd: "lss", fix: "ls"})
	if m.thefuckFix != "" {
		t.Errorf("stale AI answer became the fix: %q", m.thefuckFix)
	}

	m = failedModel(t, "ls")
	m.offerFix("", 1)
	m.Update(aiFixMsg{cmd: "ls", fix: "ls", why: "fine"})
	if m.thefuckFix != "" || !strings.Contains(bufferText(m), "looks right") {
		t.Errorf("unchanged command: fix %q buffer %q", m.thefuckFix, bufferText(m))
	}
}

func TestNewCommandClearsTheAIOffer(t *testing.T) {
	m := failedModel(t, "x")
	m.aiFix = aiFixState{cmd: "x"}
	m.thefuckFix = "y"
	m.clearFix()
	if m.aiFix.cmd != "" || m.thefuckFix != "" {
		t.Error("clearFix left state behind")
	}
}
