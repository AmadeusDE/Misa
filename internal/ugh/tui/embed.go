package tui

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// The embedding seams. Standalone, Main attaches the program's Send and
// keeps tea.Quit; a host (Misa) attaches its own delivery and exit command
// and calls Close when the pane goes away.

// initSeams sets the standalone defaults: tea.Quit on quit, a fresh
// lifetime context, and nowhere to deliver yet.
func (s *State) initSeams() {
	s.done = make(chan struct{})
	s.ctx, s.stop = context.WithCancel(context.Background())
	s.exit = tea.Quit
	s.workCond = sync.NewCond(&s.workMu)
}

// SetDeliver sets how async messages (agent events, ask and login updates)
// reach this instance's Update. Safe to call from any goroutine.
func (s *State) SetDeliver(f func(tea.Msg)) { s.deliver.Store(&f) }

// SetExit sets the command returned when the user quits (tea.Quit by
// default; a host returns its own message instead of quitting itself).
func (s *State) SetExit(cmd tea.Cmd) { s.exit = cmd }

// post delivers an async message: nothing before a delivery is attached or
// after Close.
func (s *State) post(msg tea.Msg) {
	if s.closed.Load() {
		return
	}
	if f := s.deliver.Load(); f != nil {
		(*f)(msg)
	}
}

// tracked wraps an async command so Close can wait for it. Work is counted
// when the command starts, not when it is created: a command the program
// never runs must not hold Close up.
func (s *State) tracked(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		if !s.begin() {
			return nil
		}
		defer s.end()
		return cmd()
	}
}

// begin counts one piece of in-flight work; false once Close has run.
func (s *State) begin() bool {
	s.workMu.Lock()
	defer s.workMu.Unlock()
	if s.closed.Load() {
		return false
	}
	s.workN++
	return true
}

func (s *State) end() {
	s.workMu.Lock()
	s.workN--
	s.workMu.Unlock()
	s.workCond.Broadcast()
}

// closeWait bounds how long Close waits for in-flight work to unwind.
const closeWait = 5 * time.Second

// Close cancels every run and async start, declines pending asks and
// permission prompts, and waits (up to closeWait) for them to finish. After
// it, the instance sends nothing more. Idempotent.
func (s *State) Close() {
	s.closeOnce.Do(func() {
		s.workMu.Lock()
		s.closed.Store(true)
		s.workMu.Unlock()
		s.stop()
		close(s.done)
		finished := make(chan struct{})
		go func() {
			s.workMu.Lock()
			for s.workN > 0 {
				s.workCond.Wait()
			}
			s.workMu.Unlock()
			close(finished)
		}()
		select {
		case <-finished:
		case <-time.After(closeWait):
		}
	})
}

// Status is what a host shows about an instance it is not displaying, e.g.
// in a pane's title or a tab bar.
type Status struct {
	Model     string    // the root's model ref
	Running   bool      // a root run is in progress
	Agents    int       // subagents still working
	Asks      int       // agent questions waiting for the user
	Prompting bool      // a permission prompt is open
	Usage     llm.Usage // the root agent's usage so far
}

// Status reports the instance's state; call it on the UI goroutine.
func (s *State) Status() Status {
	st := Status{Model: s.modelRef, Running: s.running, Asks: s.asks.len(), Prompting: s.modal != nil, Usage: s.usage}
	if s.runtime != nil && s.runtime.Tree != nil {
		st.Agents = s.liveChildren()
	}
	return st
}
