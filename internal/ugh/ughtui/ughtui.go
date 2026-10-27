// Package ughtui is ugh's TUI as an embeddable Bubble Tea model, for a host
// program (Misa) that runs it inside one of its own panes. The standalone
// ugh binary runs on this package too, so there is one code path.
//
// # Contract
//
// Messages: the host delivers tea.WindowSizeMsg (the pane's size), key,
// paste and mouse messages while the pane is focused,
// tea.BackgroundColorMsg / ForegroundColorMsg, and every message ugh sent
// through Host.Send.
//
// Commands: the host runs the tea.Cmds that Init and Update return and
// routes their results back to the same instance. Program-level commands
// are the host's to carry out or drop: today tea.SetClipboard (/copy, OSC
// 52) and tea.RequestBackgroundColor. ugh never writes to the terminal,
// never returns tea.Quit, and never assumes it owns the program.
//
// View: View().Content is drawn inside the pane. AltScreen, MouseMode and
// WindowTitle are hints the host may ignore; Cursor, if set, is relative to
// the pane.
//
// Quitting: when the user quits, a command returns ExitMsg. The host closes
// the pane and calls Close.
//
// After Close: ugh sends nothing more. The host may still receive messages
// that were already in flight for a closed pane and must drop them.
package ughtui

import (
	"fmt"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ugh/provider"
	"github.com/AmadeusDE/Misa/internal/ugh/tui"
)

// Host is what an embedding program provides to each instance.
type Host interface {
	// Send delivers an async message to this instance's Update. It is called
	// from any goroutine and must not block for long.
	Send(msg tea.Msg)
	// Warn shows a non-fatal warning (it is never printed by ugh).
	Warn(text string)
}

// Options configure an instance; zero values use the user's config.
type Options struct {
	Model    string // provider/model ref
	Thinking string // off|minimal|low|medium|high|xhigh|max
	System   string // system prompt
	Cwd      string // working directory (default: the process's)
	Mode     string // permission mode: manual|classify|auto
	MaxTurns int    // root turn budget (0: config)
	Resume   bool   // resume the cwd's latest session
	Host     Host   // required
}

// ExitMsg is the result of the command ugh returns when the user quits.
type ExitMsg struct{}

// Status is what a host can show about an instance (see tui.Status).
type Status = tui.Status

// Model is one ugh instance.
type Model struct {
	s    *tui.State
	host Host
}

// New starts an instance. It loads config and the session but sends
// nothing until the host starts running its commands.
func New(o Options) (*Model, error) {
	if o.Host == nil {
		return nil, fmt.Errorf("ughtui: Options.Host is required")
	}
	cwd := o.Cwd
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return nil, err
		}
	}
	s, err := tui.New(tui.Options{Model: o.Model, Thinking: o.Thinking, System: o.System, Cwd: cwd,
		Mode: o.Mode, MaxTurns: o.MaxTurns, Resume: o.Resume}, cwd)
	if err != nil {
		return nil, err
	}
	s.SetDeliver(o.Host.Send)
	s.SetExit(func() tea.Msg { return ExitMsg{} })
	m := &Model{s: s, host: o.Host}
	addHost(m)
	return m, nil
}

func (m *Model) Init() tea.Cmd { return m.s.Init() }

// Update handles one message. The returned model is always m.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.s.Update(msg)
	return m, cmd
}

func (m *Model) View() tea.View { return m.s.View() }

// Close cancels everything the instance is running and silences it.
// Idempotent.
func (m *Model) Close() {
	m.s.Close()
	removeHost(m)
}

// Status reports the instance's state; call it on the UI goroutine.
func (m *Model) Status() Status { return m.s.Status() }

// Provider warnings are per process (console providers, model scope), so
// they go to every live instance's host.
var (
	hostsMu sync.Mutex
	hosts   = map[*Model]bool{}
)

func addHost(m *Model) {
	hostsMu.Lock()
	defer hostsMu.Unlock()
	hosts[m] = true
	provider.SetWarn(warnAll)
}

func removeHost(m *Model) {
	hostsMu.Lock()
	defer hostsMu.Unlock()
	delete(hosts, m)
	if len(hosts) == 0 {
		provider.SetWarn(nil)
	}
}

func warnAll(text string) {
	hostsMu.Lock()
	live := make([]Host, 0, len(hosts))
	for m := range hosts {
		live = append(live, m.host)
	}
	hostsMu.Unlock()
	for _, h := range live {
		h.Warn(text)
	}
}
