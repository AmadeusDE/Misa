package tui

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

// fakeHost drives a State the way an embedding program does: it runs the
// returned commands, expands batches, and feeds every result and every
// delivered message back through Update on one goroutine.
type fakeHost struct {
	s    *State
	msgs chan tea.Msg
	stop chan struct{}
	mu   sync.Mutex
	seen []tea.Msg
}

func newFakeHost(s *State) *fakeHost {
	h := &fakeHost{s: s, msgs: make(chan tea.Msg, 1024), stop: make(chan struct{})}
	s.SetDeliver(h.put)
	return h
}

func (h *fakeHost) put(m tea.Msg) {
	if b, ok := m.(tea.BatchMsg); ok {
		for _, c := range b {
			h.run(c)
		}
		return
	}
	if m == nil {
		return
	}
	select {
	case h.msgs <- m:
	case <-h.stop:
	}
}

func (h *fakeHost) run(cmd tea.Cmd) {
	if cmd != nil {
		go func() { h.put(cmd()) }()
	}
}

// until feeds messages to Update until done reports true for one of them.
func (h *fakeHost) until(t *testing.T, done func(tea.Msg) bool) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case m := <-h.msgs:
			h.mu.Lock()
			h.seen = append(h.seen, m)
			h.mu.Unlock()
			_, cmd := h.s.Update(m)
			h.run(cmd)
			if done(m) {
				return
			}
		case <-deadline:
			t.Fatal("host: condition never met")
		}
	}
}

func (h *fakeHost) close() { close(h.stop) }

func isRunDone(m tea.Msg) bool { _, ok := m.(runDoneMsg); return ok }

// scriptProvider streams one scripted turn per request.
type scriptProvider struct {
	mu    sync.Mutex
	turns [][]llm.Event
}

func (p *scriptProvider) Stream(ctx context.Context, _ llm.Request) (<-chan llm.Event, error) {
	p.mu.Lock()
	var evs []llm.Event
	if len(p.turns) > 0 {
		evs, p.turns = p.turns[0], p.turns[1:]
	}
	p.mu.Unlock()
	ch := make(chan llm.Event, len(evs)+1)
	for _, e := range evs {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func embeddedState(t *testing.T, prov llm.Provider) *State {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s := sessionState(t)
	s.broker = perm.NewUIBroker(s.done)
	s.runtime = agent.NewRuntime()
	s.eng = perm.NewEngine(perm.Policy{Mode: "auto"}, s.broker, nil) // no classifiers: offline
	s.providers["fake"] = prov
	if err := s.switchSession(s.freshSession()); err != nil {
		t.Fatal(err)
	}
	return s
}

func typeAndSend(h *fakeHost, text string) {
	h.s.ta.SetValue(text)
	h.put(keyEnter())
}

// captureTerminal swaps the process's stdout and stderr for pipes and
// returns what was written to them. It changes process globals, so tests
// using it must not run in parallel with anything that writes.
func captureTerminal(t *testing.T) func() string {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = w, w
	got := make(chan string, 1)
	go func() { b, _ := io.ReadAll(r); got <- string(b) }()
	return func() string {
		os.Stdout, os.Stderr = oldOut, oldErr
		w.Close()
		return <-got
	}
}

// Embedded, a whole prompt round trip happens through Update and the host's
// delivery; nothing reaches the terminal.
func TestEmbeddedWritesNothingToTerminal(t *testing.T) {
	restore := captureTerminal(t)
	prov := &scriptProvider{turns: [][]llm.Event{{
		{Type: llm.EventTextDelta, TextDelta: "hello from ugh"},
		{Type: llm.EventDone, Stop: "stop"},
	}}}
	s := embeddedState(t, prov)
	h := newFakeHost(s)
	h.run(s.Init())
	h.put(tea.WindowSizeMsg{Width: 90, Height: 30})
	typeAndSend(h, "say hello")
	h.until(t, isRunDone)
	view := s.vp.GetContent()
	s.Close()
	h.close()
	if out := restore(); out != "" {
		t.Fatalf("wrote %d bytes to the terminal: %q", len(out), out)
	}
	if !strings.Contains(plain(view), "hello from ugh") {
		t.Fatalf("reply not rendered:\n%s", plain(view))
	}
}

// Quitting returns the exit command, never tea.Quit when a host replaced it.
func TestQuitReturnsTheExitCommand(t *testing.T) {
	type exitMsg struct{}
	s := embeddedState(t, nopProvider{})
	if msg := s.command("/quit")(); msg != (tea.QuitMsg{}) {
		t.Fatalf("standalone /quit = %#v, want tea.QuitMsg", msg)
	}
	s.SetExit(func() tea.Msg { return exitMsg{} })
	if msg := s.command("/quit")(); msg != (exitMsg{}) {
		t.Fatalf("embedded /quit = %#v", msg)
	}
	s.keyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if _, cmd := s.keyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil || cmd() != (exitMsg{}) {
		t.Fatal("embedded ctrl+c ctrl+c did not return the exit command")
	}
}

// Close, with a run inside a long bash command and an agent question
// pending, cancels the run, kills the command's process group, declines
// the question, and leaves the instance silent.
func TestCloseCancelsRun(t *testing.T) {
	pidFile := t.TempDir() + "/pid"
	prov := &scriptProvider{turns: [][]llm.Event{{
		{Type: llm.EventToolCall, Call: &llm.ToolCall{ID: "b1", Name: "bash",
			Args: `{"command":"sleep 60 & echo $! > ` + pidFile + `; wait","timeout_seconds":600}`}},
		{Type: llm.EventDone, Stop: "tool_calls"},
	}}}
	s := embeddedState(t, prov)
	h := newFakeHost(s)
	h.run(s.Init())
	typeAndSend(h, "run the long thing")
	h.until(t, func(tea.Msg) bool {
		_, err := os.Stat(pidFile)
		return err == nil
	})
	var child int
	for deadline := time.Now().Add(5 * time.Second); child == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("no pid from the bash command")
		}
		b, _ := os.ReadFile(pidFile)
		child, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	ask := startAsk(s, context.Background(), qs("A"))

	start := time.Now()
	s.Close()
	if d := time.Since(start); d >= closeWait {
		t.Fatalf("Close waited the full %s: the run did not unwind", d)
	}
	if r := <-ask; !r.ans.Declined {
		t.Fatalf("pending question not declined: %+v", r)
	}
	for deadline := time.Now().Add(5 * time.Second); syscall.Kill(child, 0) == nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(child, syscall.SIGKILL)
			t.Fatal("the bash command's child outlived Close")
		}
	}
	h.mu.Lock()
	before := len(h.msgs)
	h.mu.Unlock()
	s.post(askChangedMsg{})
	if len(h.msgs) != before {
		t.Fatal("instance delivered a message after Close")
	}
	h.close()
	s.Close() // idempotent
}

// Two instances run side by side in one process (run with -race).
func TestTwoInstances(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		prov := &scriptProvider{turns: [][]llm.Event{{
			{Type: llm.EventTextDelta, TextDelta: "reply " + strconv.Itoa(i)},
			{Type: llm.EventDone, Stop: "stop"},
		}}}
		s := embeddedState(t, prov)
		h := newFakeHost(s)
		go func() {
			defer wg.Done()
			h.run(s.Init())
			typeAndSend(h, "hi")
			h.until(t, isRunDone)
			if !strings.Contains(plain(s.vp.GetContent()), "reply "+strconv.Itoa(i)) {
				t.Errorf("instance %d lost its reply", i)
			}
			s.Close()
			h.close()
		}()
	}
	wg.Wait()
}
