package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func pumpUntilExit(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for cmd != nil {
		done := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { done <- c() }(cmd)
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-deadline:
			t.Fatal("terminal never exited")
		}
		_, cmd = m.Update(msg)
		if _, ok := msg.(termExited); ok {
			return
		}
	}
	t.Fatal("update chain ended before exit")
}

func TestMenuLaunchRunsInPaneAndReturnsToMenu(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	cmd := m.launch(helperArgv(t, "status", "launched", "3")...)
	if cmd == nil || m.terminal == nil || m.screen != screenShell {
		t.Fatalf("launch did not start an in-pane terminal (screen=%v)", m.screen)
	}
	pumpUntilExit(t, m, cmd)
	if m.terminal != nil || m.screen != screenMenu || m.menuLaunch {
		t.Fatalf("not back at menu: terminal=%v screen=%v menuLaunch=%v", m.terminal, m.screen, m.menuLaunch)
	}
	if !strings.Contains(m.status, "3") {
		t.Errorf("status = %q, want exit status 3", m.status)
	}
}

func TestRenderWhileReaderRuns(t *testing.T) {
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	cmd := m.launch(helperArgv(t, "flood", "200000")...)
	if cmd == nil {
		t.Fatalf("launch did not start a terminal (status=%q)", m.shellBuffer)
	}
	deadline := time.After(20 * time.Second)
	for {
		done := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { done <- c() }(cmd)
		var msg tea.Msg
		for msg == nil {
			select {
			case msg = <-done:
			case <-deadline:
				t.Fatal("terminal never exited")
			default:
				m.View()
			}
		}
		_, cmd = m.Update(msg)
		if _, ok := msg.(termExited); ok {
			return
		}
	}
}
