package ui

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type fakeExit struct{}

type fakeAgent struct {
	mu     sync.Mutex
	st     agentStatus
	opts   agentOptions
	host   *agentHost
	got    []tea.Msg
	closed bool
}

func (f *fakeAgent) Init() tea.Cmd { return nil }

func (f *fakeAgent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	f.mu.Lock()
	f.got = append(f.got, msg)
	f.mu.Unlock()
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "ctrl+q" {
		return f, func() tea.Msg { return fakeExit{} }
	}
	return f, nil
}

func (f *fakeAgent) View() tea.View { return tea.NewView("fake agent view") }

func (f *fakeAgent) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func (f *fakeAgent) status() agentStatus { return f.st }

func (f *fakeAgent) exited(msg tea.Msg) bool { _, ok := msg.(fakeExit); return ok }

func (f *fakeAgent) received(match func(tea.Msg) bool) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, msg := range f.got {
		if match(msg) {
			return true
		}
	}
	return false
}

func (f *fakeAgent) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// withFakeAgents makes `ugh` start fake agents, returned in start order.
func withFakeAgents(t *testing.T) *[]*fakeAgent {
	t.Helper()
	var started []*fakeAgent
	old := startAgent
	startAgent = func(o agentOptions, h *agentHost) (agent, error) {
		f := &fakeAgent{opts: o, host: h}
		started = append(started, f)
		return f, nil
	}
	t.Cleanup(func() { startAgent = old })
	return &started
}

func typeCommand(m *Model, line string) tea.Cmd {
	m.cmdInput.SetValue(line)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return cmd
}

func isKey(name string) func(tea.Msg) bool {
	return func(msg tea.Msg) bool {
		k, ok := msg.(tea.KeyPressMsg)
		return ok && k.String() == name
	}
}

func TestUghTakesOverPane(t *testing.T) {
	agents := withFakeAgents(t)
	m := fullShellModel(t)
	typeCommand(m, "ugh -m fixture/x -thinking off --resume")
	if len(*agents) != 1 || m.session == nil {
		t.Fatal("ugh did not start an agent in the pane")
	}
	a := (*agents)[0]
	if want := (agentOptions{Model: "fixture/x", Thinking: "off", Resume: true, Cwd: m.dir}); a.opts != want {
		t.Errorf("options = %+v, want %+v", a.opts, want)
	}
	if !strings.Contains(ansi.Strip(m.render()), "fake agent view") {
		t.Errorf("pane does not show the agent:\n%s", m.render())
	}
	if !a.received(func(msg tea.Msg) bool { s, ok := msg.(tea.WindowSizeMsg); return ok && s.Width > 0 && s.Height > 0 }) {
		t.Error("agent never got its size")
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m.Update(tea.PasteMsg{Content: "pasted"})
	if !a.received(isKey("x")) || !a.received(func(msg tea.Msg) bool { _, ok := msg.(tea.PasteMsg); return ok }) {
		t.Error("keys and paste did not reach the agent")
	}
	if m.cmdInput.Value() != "" {
		t.Errorf("prompt took input meant for the agent: %q", m.cmdInput.Value())
	}
}

func TestUghSubcommandsRunTheCLI(t *testing.T) {
	agents := withFakeAgents(t)
	for _, line := range []string{"ugh login", "ugh -p hi", "ugh -h"} {
		if _, ok := parseAgentArgs(strings.Fields(line)[1:]); ok {
			t.Errorf("%q parsed as an embedded session", line)
		}
	}
	if len(*agents) != 0 {
		t.Error("parsing started an agent")
	}
}

func TestAgentMailReachesAgentAndRearms(t *testing.T) {
	agents := withFakeAgents(t)
	m := fullShellModel(t)
	cmd := typeCommand(m, "ugh")
	a := (*agents)[0]
	type ping struct{ n int }
	go a.host.Send(ping{1})
	cmd = pumpUntil(t, m, cmd, "first mail", func() bool { return a.received(func(msg tea.Msg) bool { return msg == ping{1} }) })
	go a.host.Send(ping{2})
	pumpUntil(t, m, cmd, "second mail", func() bool { return a.received(func(msg tea.Msg) bool { return msg == ping{2} }) })
}

func TestAgentExitReturnsToPrompt(t *testing.T) {
	agents := withFakeAgents(t)
	m := fullShellModel(t)
	typeCommand(m, "ugh")
	a := (*agents)[0]
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	pumpUntil(t, m, cmd, "exit", func() bool { return m.session == nil })
	if !a.isClosed() {
		t.Error("exited agent was not closed")
	}
	if m.screen != screenShell || strings.Contains(m.render(), "fake agent view") {
		t.Error("pane did not return to its prompt")
	}
	m.Update(agentResult{a: a, msg: tea.KeyPressMsg{Code: 'z', Text: "z"}})
	if a.received(isKey("z")) {
		t.Error("message for a closed agent was delivered")
	}
}

func TestMenuUghTakesOverPaneAndReturnsToMenu(t *testing.T) {
	agents := withFakeAgents(t)
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	t.Cleanup(m.closeTerminals)
	m.menuCommand("ugh")
	if len(*agents) != 1 || m.session == nil || m.session.split || m.screen != screenShell {
		t.Fatalf("menu ugh did not take the pane over: agents=%d screen=%v", len(*agents), m.screen)
	}
	_, quit := m.Update(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	pumpUntil(t, m, quit, "exit", func() bool { return m.session == nil })
	if m.screen != screenMenu {
		t.Errorf("screen = %v, want back at the menu", m.screen)
	}
}

func TestMenuUghCLIRunsInPane(t *testing.T) {
	agents := withFakeAgents(t)
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	t.Cleanup(m.closeTerminals)
	m.menuCommand("ugh -h")
	if len(*agents) != 0 || m.session != nil {
		t.Error("ugh -h from the menu started an embedded session")
	}
}

func TestClosingPaneClosesAgent(t *testing.T) {
	agents := withFakeAgents(t)
	m := fullShellModel(t)
	typeCommand(m, "ugh")
	m.closeTerminals()
	if !(*agents)[0].isClosed() || m.session != nil {
		t.Error("closing the pane left the agent running")
	}
}

func TestAgentHostAfterCloseDropsMail(t *testing.T) {
	h := newAgentHost()
	h.close()
	h.Send("late")
	done := make(chan tea.Msg, 1)
	go func() { done <- h.next(nil)() }()
	select {
	case msg := <-done:
		if msg != nil {
			t.Errorf("closed host delivered %v", msg)
		}
	case <-time.After(time.Second):
		t.Error("waiting on a closed host never returned")
	}
}

// pump runs commands concurrently, as a program does, and feeds their
// messages through m. It lives across several waits so a command still in
// flight (an agent's mailbox) keeps delivering.
type pump struct {
	t    *testing.T
	m    *Model
	msgs chan tea.Msg
}

func newPump(t *testing.T, m *Model) *pump {
	return &pump{t: t, m: m, msgs: make(chan tea.Msg, 64)}
}

func (p *pump) run(cmd tea.Cmd) {
	if cmd != nil {
		go func() { p.msgs <- cmd() }()
	}
}

func (p *pump) until(what string, cond func() bool) {
	p.t.Helper()
	deadline := time.After(10 * time.Second)
	for !cond() {
		select {
		case msg := <-p.msgs:
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					p.run(c)
				}
				continue
			}
			if msg != nil {
				_, next := p.m.Update(msg)
				p.run(next)
			}
		case <-deadline:
			p.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func pumpRoot(t *testing.T, m *Model, cmd tea.Cmd, what string, cond func() bool) {
	t.Helper()
	p := newPump(t, m)
	p.run(cmd)
	p.until(what, cond)
}

func TestAgentMailReturnsToItsOwnPane(t *testing.T) {
	agents := withFakeAgents(t)
	m, first, _, _ := twoPaneRoot(t)
	var cmds []tea.Cmd
	for _, p := range m.mux.panes {
		m.focusPane(p)
		p.misa.enterFullShell()
		p.misa.cmdInput.SetValue("ugh")
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		cmds = append(cmds, cmd)
	}
	if len(*agents) != 2 {
		t.Fatalf("started %d agents, want 2", len(*agents))
	}
	m.focusPane(first)
	second := (*agents)[1]
	go second.host.Send("for second")
	pumpRoot(t, m, tea.Batch(cmds...), "mail", func() bool {
		return second.received(func(msg tea.Msg) bool { return msg == "for second" })
	})
	if (*agents)[0].received(func(msg tea.Msg) bool { return msg == "for second" }) {
		t.Error("mail reached the focused pane's agent instead of its owner")
	}
}

func TestHostColorRepliesReachEveryAgent(t *testing.T) {
	agents := withFakeAgents(t)
	m, _, _, _ := twoPaneRoot(t)
	var cmds []tea.Cmd
	for _, p := range m.mux.panes {
		m.focusPane(p)
		p.misa.enterFullShell()
		p.misa.cmdInput.SetValue("ugh")
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		cmds = append(cmds, cmd)
	}
	m.Update(tea.BackgroundColorMsg{})
	isColor := func(msg tea.Msg) bool { _, ok := msg.(tea.BackgroundColorMsg); return ok }
	pumpRoot(t, m, tea.Batch(cmds...), "colour replies", func() bool {
		return (*agents)[0].received(isColor) && (*agents)[1].received(isColor)
	})
}

func twoAgentRoot(t *testing.T) (*Model, []*fakeAgent) {
	t.Helper()
	agents := withFakeAgents(t)
	m, _, _, _ := twoPaneRoot(t)
	for _, p := range m.mux.panes {
		m.focusPane(p)
		p.misa.enterFullShell()
		p.misa.cmdInput.SetValue("ugh")
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	return m, *agents
}

func TestMuxBarSummarisesAgentSessions(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	if bar := ansi.Strip(m.muxBarView()); strings.Contains(bar, "ugh") {
		t.Errorf("bar mentions ugh with no session: %q", bar)
	}
	m, agents := twoAgentRoot(t)
	if bar := ansi.Strip(m.muxBarView()); !strings.Contains(bar, "ugh idle") {
		t.Errorf("bar = %q, want idle sessions shown", bar)
	}
	agents[0].st = agentStatus{Running: true, Agents: 3}
	agents[1].st = agentStatus{Waiting: true}
	if bar := ansi.Strip(m.muxBarView()); !strings.Contains(bar, "ugh 1 busy · 3 sub · 1 waiting") {
		t.Errorf("bar = %q", bar)
	}
}

func TestTagWithWaitingAgentIsFlagged(t *testing.T) {
	m, agents := twoAgentRoot(t)
	m.switchTag(1)
	if m.tagWaiting(0) {
		t.Error("tag flagged with nothing waiting")
	}
	agents[0].st = agentStatus{Waiting: true}
	if !m.tagWaiting(0) {
		t.Error("tag with a waiting session not flagged")
	}
}

func TestAgentPaneOpensSplitAndClosesOnExit(t *testing.T) {
	agents := withFakeAgents(t)
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	first := m.activePane()
	first.misa.dir = t.TempDir()
	m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	_, open := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if len(m.mux.panes) != 2 || len(*agents) != 1 || m.activePane().misa.session == nil {
		t.Fatalf("ctrl+w a: panes=%d agents=%d", len(m.mux.panes), len(*agents))
	}
	if got := (*agents)[0].opts.Cwd; got != first.misa.dir {
		t.Errorf("agent cwd = %q, want the focused pane's %q", got, first.misa.dir)
	}
	_, quit := m.Update(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	pumpRoot(t, m, tea.Batch(open, quit), "pane close", func() bool { return len(m.mux.panes) == 1 })
	if !(*agents)[0].isClosed() {
		t.Error("closed pane left its agent running")
	}
}

// splitSession opens a ctrl+w a split and returns its agent and the
// command stream to pump.
func splitSession(t *testing.T, m *Model) (*fakeAgent, tea.Cmd) {
	t.Helper()
	agents := withFakeAgents(t)
	m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	return (*agents)[len(*agents)-1], cmd
}

func closeFocusedPane(m *Model) {
	m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
}

func TestClosingSplitDetachesSession(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	a, cmd := splitSession(t, m)
	closeFocusedPane(m)
	if len(m.mux.panes) != 1 || a.isClosed() || len(m.detached) != 1 {
		t.Fatalf("panes=%d closed=%v detached=%d", len(m.mux.panes), a.isClosed(), len(m.detached))
	}
	if bar := ansi.Strip(m.muxBarView()); !strings.Contains(bar, "1 detached") {
		t.Errorf("bar = %q, want the detached session shown", bar)
	}
	go a.host.Send("while detached")
	pumpRoot(t, m, cmd, "detached mail", func() bool {
		return a.received(func(msg tea.Msg) bool { return msg == "while detached" })
	})
}

func TestTakeoverSessionEndsWithItsPane(t *testing.T) {
	agents := withFakeAgents(t)
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	m.addMuxPane()
	p := m.activePane()
	p.misa.enterFullShell()
	p.misa.cmdInput.SetValue("ugh")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	closeFocusedPane(m)
	if !(*agents)[0].isClosed() || len(m.detached) != 0 {
		t.Error("a session typed into a pane outlived the pane")
	}
}

func TestReattachDetachedSession(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	a, cmd := splitSession(t, m)
	closeFocusedPane(m)
	m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	_, reattach := m.Update(tea.KeyPressMsg{Code: 'A', Text: "A", Mod: tea.ModShift})
	if len(m.mux.panes) != 2 || len(m.detached) != 0 {
		t.Fatalf("panes=%d detached=%d", len(m.mux.panes), len(m.detached))
	}
	s := m.activePane().misa.session
	if s == nil || s.a != agent(a) {
		t.Fatal("reattached pane does not hold the session")
	}
	go a.host.Send("after reattach")
	pumpRoot(t, m, tea.Batch(cmd, reattach), "mail after reattach", func() bool {
		return a.received(func(msg tea.Msg) bool { return msg == "after reattach" })
	})
	if !strings.Contains(ansi.Strip(m.render()), "fake agent view") {
		t.Error("reattached session not shown")
	}
}

func TestDetachedSessionQuitEndsIt(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	a, cmd := splitSession(t, m)
	closeFocusedPane(m)
	pumpRoot(t, m, tea.Batch(cmd, agentCmd(a, func() tea.Msg { return fakeExit{} })), "end", func() bool {
		return len(m.detached) == 0
	})
	if !a.isClosed() {
		t.Error("quit detached session not closed")
	}
}

func TestShutdownClosesDetachedSessions(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	a, _ := splitSession(t, m)
	closeFocusedPane(m)
	m.Shutdown()
	if !a.isClosed() {
		t.Error("Misa exit left a detached session running")
	}
}
