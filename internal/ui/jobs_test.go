package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// pumpUntil feeds cmd's messages back into m until cond holds, returning the
// command that continues the chain.
func pumpUntil(t *testing.T, m *Model, cmd tea.Cmd, what string, cond func() bool) tea.Cmd {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !cond() {
		if cmd == nil {
			t.Fatalf("update chain ended before %s", what)
		}
		done := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { done <- c() }(cmd)
		select {
		case msg := <-done:
			_, cmd = m.Update(msg)
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
	return cmd
}

func shellWithStoppingCommand(t *testing.T) (*Model, tea.Cmd) {
	t.Helper()
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	m.shell.lastCmd = "stopper"
	cmd := m.execArgv("sh", "-c", "kill -STOP $$; read x; echo got $x")
	t.Cleanup(m.closeTerminals)
	return m, pumpUntil(t, m, cmd, "suspend", func() bool { return len(m.jobs) == 1 })
}

func bufferHas(m *Model, text string) bool {
	return slices.ContainsFunc(m.shellBuffer, func(l string) bool { return strings.Contains(l, text) })
}

func TestStoppedCommandReturnsToPrompt(t *testing.T) {
	m, _ := shellWithStoppingCommand(t)
	if m.terminal != nil {
		t.Error("stopped program still holds the screen")
	}
	if !bufferHas(m, "[1]+  Stopped  stopper") {
		t.Errorf("no stop notice in %q", m.shellBuffer)
	}
}

func TestFgResumesStoppedCommand(t *testing.T) {
	m, cmd := shellWithStoppingCommand(t)
	m.handleBuiltinCmd("fg")
	if m.terminal == nil || len(m.jobs) != 0 || m.terminal.Stopped() {
		t.Fatalf("fg did not resume: terminal=%v jobs=%d", m.terminal != nil, len(m.jobs))
	}
	m.terminal.SendBytes([]byte("hi\r"))
	pumpUntil(t, m, cmd, "exit", func() bool { return m.terminal == nil && bufferHas(m, "got hi") })
}

func TestBgJobReportsDone(t *testing.T) {
	m, cmd := shellWithStoppingCommand(t)
	m.handleBuiltinCmd("bg")
	j := m.jobs[0]
	if j.t.Stopped() || m.terminal != nil {
		t.Fatal("bg did not resume the job in the background")
	}
	j.t.SendBytes([]byte("x\r"))
	pumpUntil(t, m, cmd, "done notice", func() bool { return len(m.jobs) == 0 })
	if !bufferHas(m, "[1]+  Done") {
		t.Errorf("no done notice in %q", m.shellBuffer)
	}
}

func TestFgWithoutJobs(t *testing.T) {
	m := New(t.TempDir())
	m.handleBuiltinCmd("fg %3")
	if !bufferHas(m, "fg: no current job") {
		t.Errorf("got %q", m.shellBuffer)
	}
}
