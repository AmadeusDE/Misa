package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ai/aitest"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// turn is one scripted answer of the fake provider.
type turn struct {
	events []llm.Event
	err    error // returned by Stream itself, like an HTTP 400
	hold   bool  // keep the stream open after the events until the request is cancelled
}

type fakeProvider struct {
	mu    sync.Mutex
	turns []turn
	reqs  []llm.Request
}

func (f *fakeProvider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	f.mu.Lock()
	i := len(f.reqs)
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	if i >= len(f.turns) {
		return nil, errors.New("script exhausted")
	}
	t := f.turns[i]
	if t.err != nil {
		return nil, t.err
	}
	ch := make(chan llm.Event)
	go func() {
		defer close(ch)
		for _, ev := range t.events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
		if t.hold {
			<-ctx.Done()
		}
	}()
	return ch, nil
}

func (f *fakeProvider) requests() []llm.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]llm.Request(nil), f.reqs...)
}

func text(parts ...string) turn {
	var t turn
	for _, p := range parts {
		t.events = append(t.events, llm.Event{Type: llm.EventTextDelta, TextDelta: p})
	}
	return t
}

func withUsage(t turn, in, out int, cost float64) turn {
	t.events = append(t.events, llm.Event{Type: llm.EventUsage, Usage: &llm.Usage{In: in, Out: out, Cost: cost}})
	return t
}

func withThinking(t turn, s string) turn {
	t.events = append([]llm.Event{{Type: llm.EventThinkingDelta, Thinking: s}}, t.events...)
	return t
}

func toolCall(id, name, args string) turn {
	return turn{events: []llm.Event{{Type: llm.EventToolCall, Call: &llm.ToolCall{ID: id, Name: name, Args: args}}}}
}

// rig is a Chat wired to a fake provider with everything in temp dirs.
type rig struct {
	t      *testing.T
	chat   *Chat
	p      *fakeProvider
	opened []string
	copied []string
	tools  []ai.Tool
}

func newRig(t *testing.T, turns ...turn) *rig {
	t.Helper()
	aitest.Isolate(t)
	r := &rig{t: t, p: &fakeProvider{turns: turns}}
	r.chat = r.chatWith(Options{Dir: t.TempDir()})
	return r
}

func (r *rig) chatWith(opt Options) *Chat {
	r.t.Helper()
	if opt.Dir == "" {
		opt.Dir = r.chat.store.Dir
	}
	opt.ModelFor = func() (string, error) { return "fake/model", nil }
	opt.Open = func(ref string) (*ai.Client, error) {
		r.opened = append(r.opened, ref)
		return &ai.Client{Ref: ref, Model: ref[strings.Index(ref, "/")+1:], P: r.p}, nil
	}
	opt.Models = func(context.Context) ([]string, []string) {
		return []string{"fake/model", "fake/other", "local/llama"}, []string{"broken: HTTP 500"}
	}
	opt.Tools = func() ([]ai.Tool, error) { return r.tools, nil }
	opt.Copy = func(s string) tea.Cmd { r.copied = append(r.copied, s); return nil }
	c, err := New(opt)
	if err != nil {
		r.t.Fatal(err)
	}
	c.Resize(100, 30)
	return c
}

// drive runs cmds to completion, feeding every message they produce back to the chat.
func (r *rig) drive(cmds ...tea.Cmd) {
	r.t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		queue := cmds
		for len(queue) > 0 {
			c := queue[0]
			queue = queue[1:]
			if c == nil {
				continue
			}
			msg := c()
			if batch, ok := msg.(tea.BatchMsg); ok {
				queue = append(queue, batch...)
				continue
			}
			queue = append(queue, r.chat.Update(msg))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		r.t.Fatal("commands did not finish")
	}
}

func (r *rig) press(names ...string) tea.Cmd {
	r.t.Helper()
	var cmds []tea.Cmd
	for _, n := range names {
		cmds = append(cmds, r.chat.Update(tea.KeyPressMsg(parseKey(r.t, n))))
	}
	return tea.Batch(cmds...)
}

func (r *rig) typeText(s string) {
	for _, c := range s {
		r.chat.Update(tea.KeyPressMsg{Code: c, Text: string(c)})
	}
}

// ask types a message, sends it and waits for the answer.
func (r *rig) ask(s string) {
	r.t.Helper()
	r.typeText(s)
	r.drive(r.press("enter"))
}

func parseKey(t *testing.T, name string) tea.Key {
	t.Helper()
	var k tea.Key
	parts := strings.Split(name, "+")
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "ctrl":
			k.Mod |= tea.ModCtrl
		case "alt":
			k.Mod |= tea.ModAlt
		case "shift":
			k.Mod |= tea.ModShift
		default:
			t.Fatalf("bad modifier in %q", name)
		}
	}
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "up": tea.KeyUp, "down": tea.KeyDown, "tab": tea.KeyTab,
		"backspace": tea.KeyBackspace, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "delete": tea.KeyDelete,
		"left": tea.KeyLeft, "right": tea.KeyRight,
	}
	last := parts[len(parts)-1]
	if code, ok := named[last]; ok {
		k.Code = code
	} else {
		k.Code = []rune(last)[0]
		if k.Mod == 0 {
			k.Text = last
		}
	}
	return k
}

func (r *rig) view() string { return stripANSI(r.chat.View()) }

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < '@' || s[i] > '~') {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func (r *rig) saved(id string) *Conversation {
	r.t.Helper()
	c, err := r.chat.store.Load(id)
	if err != nil {
		r.t.Fatal(err)
	}
	return c
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
