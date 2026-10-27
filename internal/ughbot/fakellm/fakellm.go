// Package fakellm is a scripted llm.Provider for tests that must not touch the network.
package fakellm

import (
	"context"
	"strings"
	"sync"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// Script decides the events for one request; n counts requests from 1.
type Script func(req llm.Request, n int) []llm.Event

type Provider struct {
	Script Script

	mu       sync.Mutex
	requests []llm.Request
}

// Echo replies "echo: <last user text>" and reports cost per request.
func Echo(cost float64) *Provider {
	return &Provider{Script: func(req llm.Request, _ int) []llm.Event {
		return Reply("echo: "+LastUser(req), cost)
	}}
}

// Reply is a text answer that reports cost.
func Reply(text string, cost float64) []llm.Event {
	return []llm.Event{
		{Type: llm.EventTextDelta, TextDelta: text},
		{Type: llm.EventUsage, Usage: &llm.Usage{In: 10, Out: 2, Cost: cost}},
		{Type: llm.EventDone, Stop: "stop"},
	}
}

// ToolCall asks for one tool call.
func ToolCall(id, name, args string, cost float64) []llm.Event {
	return []llm.Event{
		{Type: llm.EventToolCall, Seq: 0, Call: &llm.ToolCall{ID: id, Name: name, Args: args}},
		{Type: llm.EventUsage, Usage: &llm.Usage{In: 10, Out: 2, Cost: cost}},
		{Type: llm.EventDone, Stop: "tool_calls"},
	}
}

func LastUser(req llm.Request) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if m := req.Messages[i]; m.Role == llm.RoleUser {
			return m.Text
		}
	}
	return ""
}

func (p *Provider) Stream(_ context.Context, req llm.Request) (<-chan llm.Event, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	n := len(p.requests)
	p.mu.Unlock()
	events := p.Script(req, n)
	ch := make(chan llm.Event, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

// Requests returns the requests seen so far.
func (p *Provider) Requests() []llm.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]llm.Request(nil), p.requests...)
}

// History is every message text of the latest request, joined, for "does it remember" checks.
func (p *Provider) History() string {
	reqs := p.Requests()
	if len(reqs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, m := range reqs[len(reqs)-1].Messages {
		b.WriteString(m.Text)
		b.WriteByte('\n')
	}
	return b.String()
}
