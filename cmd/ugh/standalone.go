package main

import (
	"fmt"
	"os"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ugh/ughtui"
)

// runTUI runs ugh's TUI as its own program: the same ughtui.Model a host
// like Misa embeds, with this program as the host.
func runTUI(o ughtui.Options) error {
	h := &programHost{}
	o.Host = h
	m, err := ughtui.New(o)
	if err != nil {
		return err
	}
	defer m.Close() // cancels runs; unblocks any question still waiting
	prog := tea.NewProgram(standalone{m})
	h.prog.Store(prog)
	_, err = prog.Run()
	return err
}

// programHost delivers to the standalone program; warnings go to stderr.
type programHost struct{ prog atomic.Pointer[tea.Program] }

func (h *programHost) Send(msg tea.Msg) {
	if p := h.prog.Load(); p != nil {
		p.Send(msg)
	}
}

func (h *programHost) Warn(text string) { fmt.Fprintln(os.Stderr, text) }

// standalone maps ugh's ExitMsg to quitting the program.
type standalone struct{ m *ughtui.Model }

func (s standalone) Init() tea.Cmd { return s.m.Init() }

func (s standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ughtui.ExitMsg); ok {
		return s, tea.Quit
	}
	_, cmd := s.m.Update(msg)
	return s, cmd
}

func (s standalone) View() tea.View { return s.m.View() }
