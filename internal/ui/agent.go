package ui

import (
	"flag"
	"io"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// agent is a Bubble Tea program embedded in a pane: ugh's TUI.
type agent interface {
	Init() tea.Cmd
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() tea.View
	Close()
	status() agentStatus
	// exited reports whether msg is the agent telling its host it quit.
	exited(msg tea.Msg) bool
}

// agentOptions are the `ugh` arguments an embedded session understands.
type agentOptions struct {
	Model, Thinking, System, Mode, Cwd string
	MaxTurns                           int
	Resume                             bool
}

// startAgent starts an embedded agent, or is nil in builds without ugh
// (-tags nough), where `ugh` runs the external binary.
var startAgent func(agentOptions, *agentHost) (agent, error)

// parseAgentArgs reads `ugh` arguments for an embedded session. It reports
// false for anything the ugh CLI must handle instead: subcommands, headless
// -p prompts, help and flags it doesn't know.
func parseAgentArgs(args []string) (agentOptions, bool) {
	var o agentOptions
	var prompt string
	fs := flag.NewFlagSet("ugh", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.Model, "m", "", "")
	fs.StringVar(&o.Thinking, "thinking", "", "")
	fs.StringVar(&o.System, "system", "", "")
	fs.StringVar(&o.Cwd, "cwd", "", "")
	fs.StringVar(&o.Mode, "mode", "", "")
	fs.IntVar(&o.MaxTurns, "max-turns", 0, "")
	fs.BoolVar(&o.Resume, "resume", false, "")
	fs.StringVar(&prompt, "p", "", "")
	if fs.Parse(args) != nil || fs.NArg() > 0 || prompt != "" {
		return agentOptions{}, false
	}
	return o, true
}

// agentMail is a message the agent sent through its host; handling it
// re-arms the wait for the next one.
type agentMail struct {
	a   agent
	msg tea.Msg
}

// agentResult is the result of a command the agent returned.
type agentResult struct {
	a   agent
	msg tea.Msg
}

type agentWarning string

// agentHost is the ugh Host for one session: a mailbox that any goroutine
// fills and the pane drains through Bubble Tea commands, so delivery needs no
// handle on the program and lands in the pane that owns the session.
type agentHost struct {
	mu     sync.Mutex
	queue  []tea.Msg
	ready  chan struct{}
	done   chan struct{}
	closed bool
}

func newAgentHost() *agentHost {
	return &agentHost{ready: make(chan struct{}, 1), done: make(chan struct{})}
}

func (h *agentHost) Send(msg tea.Msg) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.queue = append(h.queue, msg)
	select {
	case h.ready <- struct{}{}:
	default:
	}
}

func (h *agentHost) Warn(text string) { h.Send(agentWarning(text)) }

func (h *agentHost) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		h.closed = true
		h.queue = nil
		close(h.done)
	}
}

// next waits for the next message sent to a, or returns nil once the host
// is closed.
func (h *agentHost) next(a agent) tea.Cmd {
	return func() tea.Msg {
		for {
			h.mu.Lock()
			if h.closed {
				h.mu.Unlock()
				return nil
			}
			if len(h.queue) > 0 {
				msg := h.queue[0]
				h.queue = h.queue[1:]
				h.mu.Unlock()
				return agentMail{a: a, msg: msg}
			}
			h.mu.Unlock()
			select {
			case <-h.ready:
			case <-h.done:
			}
		}
	}
}

// agentCmd tags the results of the agent's commands so they come back to it.
func agentCmd(a agent, cmd tea.Cmd) tea.Cmd {
	return tagCmd(cmd, func(msg tea.Msg) tea.Msg { return agentResult{a: a, msg: msg} })
}

// agentSession is one embedded agent with its mailbox. A pane holds it, or
// the root while it is detached.
type agentSession struct {
	a    agent
	host *agentHost
	size tea.WindowSizeMsg
	// split sessions have a pane of their own: quitting closes it, and
	// closing it detaches the session instead of ending it.
	split bool
	// fromMenu sessions return the pane to its menu when they end.
	fromMenu bool
}

func (s *agentSession) close() {
	s.a.Close()
	s.host.close()
}

// handle runs one message for the session. Mail re-arms the wait for the
// next; a warning is returned for the holder to show.
func (s *agentSession) handle(msg tea.Msg, mail bool) (cmd tea.Cmd, exited bool, warning string) {
	var rearm tea.Cmd
	if mail {
		rearm = s.host.next(s.a)
	}
	switch msg := msg.(type) {
	case nil:
		return rearm, false, ""
	case agentWarning:
		return rearm, false, string(msg)
	case tea.BatchMsg:
		return tea.Batch(agentCmd(s.a, func() tea.Msg { return msg }), rearm), false, ""
	}
	if s.a.exited(msg) {
		return nil, true, ""
	}
	_, cmd = s.a.Update(msg)
	return tea.Batch(agentCmd(s.a, cmd), rearm), false, ""
}

// startAgentSession takes the pane over with an embedded agent. It reports
// false when `ugh args` is for the CLI or ugh isn't built in.
func (m *Model) startAgentSession(args []string) (bool, tea.Cmd) {
	o, ok := parseAgentArgs(args)
	if startAgent == nil || !ok {
		return false, nil
	}
	if o.Cwd == "" {
		o.Cwd = m.dir
	}
	host := newAgentHost()
	a, err := startAgent(o, host)
	if err != nil {
		m.shellBuffer = append(m.shellBuffer, "ugh: "+err.Error())
		return true, nil
	}
	m.session = &agentSession{a: a, host: host}
	if m.screen != screenShell {
		m.enterFullShell()
	}
	return true, tea.Batch(agentCmd(a, a.Init()), m.sizeAgent(), host.next(a))
}

// menuCommand runs a menu entry's command in this pane: an embedded ugh
// session for `ugh`, the shell for anything else.
func (m *Model) menuCommand(cmd string) tea.Cmd {
	cmd = expandEnv(cmd)
	if fields := strings.Fields(cmd); len(fields) > 0 && fields[0] == "ugh" {
		fromMenu := m.screen != screenShell
		if handled, start := m.startAgentSession(fields[1:]); handled {
			if m.session != nil {
				m.session.fromMenu = fromMenu
			}
			return start
		}
	}
	return m.launch("sh", "-c", cmd)
}

// sizeAgent gives the agent the pane's size when it changed.
func (m *Model) sizeAgent() tea.Cmd {
	size, changed := m.newAgentSize()
	if !changed {
		return nil
	}
	_, cmd := m.session.a.Update(size)
	return agentCmd(m.session.a, cmd)
}

// queueAgentSize is sizeAgent for layout changes, which can't return a
// command: the size arrives as mail.
func (m *Model) queueAgentSize() {
	if size, changed := m.newAgentSize(); changed {
		m.session.host.Send(size)
	}
}

func (m *Model) newAgentSize() (tea.WindowSizeMsg, bool) {
	size := tea.WindowSizeMsg{Width: m.mux.primaryW, Height: m.mux.primaryH}
	if m.session == nil || size == m.session.size || size.Width <= 0 || size.Height <= 0 {
		return size, false
	}
	m.session.size = size
	return size, true
}

func (m *Model) stopAgent() {
	if m.session == nil {
		return
	}
	m.session.close()
	m.session = nil
	m.updateFullShellVP()
}

// updateAgentMsg delivers mail and command results to the pane's session,
// dropping those for a session it no longer holds.
func (m *Model) updateAgentMsg(a agent, msg tea.Msg, mail bool) (tea.Model, tea.Cmd) {
	s := m.session
	if s == nil || s.a != a {
		return m, nil
	}
	cmd, exited, warning := s.handle(msg, mail)
	if warning != "" {
		m.shellBuffer = append(m.shellBuffer, "ugh: "+warning)
	}
	if exited {
		m.stopAgent()
		m.wantsClose = s.split
		if s.fromMenu {
			m.exitFullShell()
		}
	}
	return m, cmd
}

// updateAgentInput hands input to the session that has taken the pane over.
func (m *Model) updateAgentInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouse, ok := msg.(mouseMsg); ok {
		msg = mouse.toTea()
	}
	_, cmd := m.session.a.Update(msg)
	return m, agentCmd(m.session.a, cmd)
}

// sendAgentMail queues msg for this pane's session, if one is running.
func (m *Model) sendAgentMail(msg tea.Msg) {
	if m.session != nil {
		m.session.host.Send(msg)
	}
}
