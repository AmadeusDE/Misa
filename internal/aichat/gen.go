package aichat

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

type eventKind int

const (
	evText eventKind = iota
	evThinking
	evNote
	evDone
)

// genEvent is one thing a generation reports: streamed text or thinking, a dimmed note, or the end.
type genEvent struct {
	kind    eventKind
	text    string
	usage   llm.Usage
	err     error
	noTools bool // evDone: the model rejected tools and the answer came without them
}

// genMsg carries the events of generation gen that arrived together.
type genMsg struct {
	gen    int
	events []genEvent
}

const (
	maxToolSteps = 6
	sendStall    = 10 * time.Second
)

// job is one running generation. It streams on its own goroutine and hands events over a channel; wait is the
// tea.Cmd that blocks for the next batch, so the Update goroutine never waits on the network.
type job struct {
	gen    int
	cancel context.CancelFunc
	events chan genEvent
}

func startJob(gen int, c *ai.Client, req llm.Request, tools []ai.Tool) *job {
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{gen: gen, cancel: cancel, events: make(chan genEvent, 64)}
	go j.run(ctx, c, req, tools)
	return j
}

// emit delivers an event, or gives the generation up when nobody has taken events for sendStall (the screen is gone).
func (j *job) emit(ctx context.Context, ev genEvent) {
	timer := time.NewTimer(sendStall)
	defer timer.Stop()
	select {
	case j.events <- ev:
	case <-ctx.Done():
	case <-timer.C:
		j.cancel()
	}
}

func (j *job) run(ctx context.Context, c *ai.Client, req llm.Request, tools []ai.Tool) {
	defer close(j.events)
	defer j.cancel()
	var text bool
	turn := ai.Turn{
		OnText: func(s string) {
			text = text || s != ""
			j.emit(ctx, genEvent{kind: evText, text: s})
		},
		OnThinking: func(s string) { j.emit(ctx, genEvent{kind: evThinking, text: s}) },
		OnToolCall: func(name, args string) { j.emit(ctx, genEvent{kind: evNote, text: describeCall(name, args)}) },
		OnToolResult: func(name, result string, err error) {
			if err != nil {
				j.emit(ctx, genEvent{kind: evNote, text: name + " failed: " + firstLine(err.Error())})
			}
		},
	}
	_, used, err := c.RunTools(ctx, req, tools, maxToolSteps, turn)
	done := genEvent{kind: evDone, usage: used, err: err}
	if err != nil && len(tools) > 0 && !text && rejectsTools(err) {
		j.emit(ctx, genEvent{kind: evNote, text: "model does not accept tools: answering without web search"})
		_, more, err := c.RunTools(ctx, req, nil, 1, turn)
		used.Add(more)
		done = genEvent{kind: evDone, usage: used, err: err, noTools: true}
	}
	j.emit(ctx, done)
}

// wait blocks for the next event and takes every other one already queued.
func (j *job) wait() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-j.events
		if !ok {
			return genMsg{gen: j.gen, events: []genEvent{{kind: evDone, err: context.Canceled}}}
		}
		events := []genEvent{ev}
		for ev.kind != evDone {
			select {
			case ev, ok = <-j.events:
				if !ok {
					return genMsg{gen: j.gen, events: events}
				}
				events = append(events, ev)
			default:
				return genMsg{gen: j.gen, events: events}
			}
		}
		return genMsg{gen: j.gen, events: events}
	}
}

// rejectsTools recognises a provider refusing the request because of its tool definitions.
func rejectsTools(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http 400") && strings.Contains(msg, "tool")
}

func describeCall(name, args string) string {
	var a struct{ Query, URL string }
	_ = json.Unmarshal([]byte(args), &a)
	switch {
	case name == "web_search" && a.Query != "":
		return "searched: " + a.Query
	case name == "fetch_url" && a.URL != "":
		return "fetched: " + a.URL
	}
	return "called " + name
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
