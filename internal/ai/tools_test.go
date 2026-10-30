package ai

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// scripted answers each Stream call with the next list of events and records the requests.
type scripted struct {
	turns [][]llm.Event
	reqs  []llm.Request
}

func (s *scripted) Stream(_ context.Context, req llm.Request) (<-chan llm.Event, error) {
	s.reqs = append(s.reqs, req)
	i := len(s.reqs) - 1
	if i >= len(s.turns) {
		return nil, errors.New("script exhausted")
	}
	ch := make(chan llm.Event, len(s.turns[i]))
	for _, ev := range s.turns[i] {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func call(seq int, id, name, args string) llm.Event {
	return llm.Event{Type: llm.EventToolCall, Seq: seq, Call: &llm.ToolCall{ID: id, Name: name, Args: args}}
}

func text(s string) llm.Event { return llm.Event{Type: llm.EventTextDelta, TextDelta: s} }

func usage(in, out int) llm.Event {
	return llm.Event{Type: llm.EventUsage, Usage: &llm.Usage{In: in, Out: out}}
}

func echoTool(got *[]string) Tool {
	return Tool{
		Spec: llm.ToolSpec{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}},
		Run: func(_ context.Context, args json.RawMessage) (string, error) {
			var a struct{ Q string }
			if err := json.Unmarshal(args, &a); err != nil {
				return "", err
			}
			*got = append(*got, a.Q)
			return "result:" + a.Q, nil
		},
	}
}

func TestRunToolsRunsToolsThenAnswers(t *testing.T) {
	p := &scripted{turns: [][]llm.Event{
		{llm.Event{Type: llm.EventThinkingDelta, Thinking: "hmm"},
			call(0, "c1", "echo", ""), call(1, "c2", "echo", `{"Q":"b"}`), call(0, "", "", `{"Q":`), call(0, "", "", `"a"}`), usage(10, 2)},
		{text("done "), text("now"), usage(20, 3)},
	}}
	c := &Client{Ref: "x/m", Model: "m", P: p}
	var ran, texts, thinking, started, results []string
	turn := Turn{
		OnText:       func(s string) { texts = append(texts, s) },
		OnThinking:   func(s string) { thinking = append(thinking, s) },
		OnToolCall:   func(n, a string) { started = append(started, n+a) },
		OnToolResult: func(n, r string, err error) { results = append(results, n+"="+r) },
	}
	base := []llm.Message{{Role: llm.RoleUser, Text: "q"}}
	added, used, err := c.RunTools(context.Background(), llm.Request{System: "sys", Messages: base}, []Tool{echoTool(&ran)}, 5, turn)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 4 || added[0].Role != llm.RoleAssistant || added[1].Role != llm.RoleTool || added[3].Text != "done now" {
		t.Fatalf("added %+v", added)
	}
	wantCalls := []llm.ToolCall{{ID: "c1", Name: "echo", Args: `{"Q":"a"}`}, {ID: "c2", Name: "echo", Args: `{"Q":"b"}`}}
	if !reflect.DeepEqual(added[0].ToolCalls, wantCalls) || added[0].Reasoning != "hmm" || added[0].Route != "x/m" {
		t.Fatalf("assembled calls %+v", added[0])
	}
	if added[1].ToolCallID != "c1" || added[1].Text != "result:a" || added[2].ToolCallID != "c2" || added[2].ToolName != "echo" {
		t.Fatalf("results %+v", added[1:3])
	}
	if !reflect.DeepEqual(ran, []string{"a", "b"}) {
		t.Fatalf("ran %v", ran)
	}
	if used.In != 30 || used.Out != 5 {
		t.Fatalf("usage %+v", used)
	}
	if strings.Join(texts, "") != "done now" || strings.Join(thinking, "") != "hmm" ||
		strings.Join(started, "|") != `echo{"Q":"a"}|echo{"Q":"b"}` || strings.Join(results, "|") != "echo=result:a|echo=result:b" {
		t.Fatalf("callbacks %v %v %v %v", texts, thinking, started, results)
	}
	if len(p.reqs) != 2 || p.reqs[0].Model != "m" || p.reqs[0].System != "sys" || len(p.reqs[0].Tools) != 1 {
		t.Fatalf("first request %+v", p.reqs[0])
	}
	if got := len(p.reqs[1].Messages); got != 4 { // user, assistant, two results
		t.Fatalf("second request carried %d messages", got)
	}
	if len(base) != 1 {
		t.Fatal("caller's messages were modified")
	}
}

func TestRunToolsReportsToolFailuresToTheModel(t *testing.T) {
	p := &scripted{turns: [][]llm.Event{
		{call(0, "", "boom", "{}"), call(1, "", "missing", "{}")},
		{text("sorry")},
	}}
	boom := Tool{Spec: llm.ToolSpec{Name: "boom"}, Run: func(context.Context, json.RawMessage) (string, error) {
		return "", errors.New("it broke")
	}}
	var errs []error
	c := &Client{Ref: "x/m", Model: "m", P: p}
	added, _, err := c.RunTools(context.Background(), llm.Request{}, []Tool{boom}, 3,
		Turn{OnToolResult: func(_, _ string, err error) { errs = append(errs, err) }})
	if err != nil {
		t.Fatal(err)
	}
	if added[0].ToolCalls[0].ID == "" || added[0].ToolCalls[0].ID == added[0].ToolCalls[1].ID {
		t.Fatalf("calls need distinct ids: %+v", added[0].ToolCalls)
	}
	if added[1].Text != "error: it broke" || !strings.Contains(added[2].Text, `unknown tool "missing"`) || added[1].ToolCallID != added[0].ToolCalls[0].ID {
		t.Fatalf("results %+v", added[1:3])
	}
	if len(errs) != 2 || errs[0] == nil || errs[1] == nil {
		t.Fatalf("callback errors %v", errs)
	}
}

func TestRunToolsLastStepWithholdsTools(t *testing.T) {
	call1 := []llm.Event{call(0, "c1", "echo", `{"Q":"x"}`)}
	p := &scripted{turns: [][]llm.Event{call1, call1, {text("final")}}}
	var ran []string
	c := &Client{Ref: "x/m", Model: "m", P: p}
	added, _, err := c.RunTools(context.Background(), llm.Request{}, []Tool{echoTool(&ran)}, 3, Turn{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.reqs) != 3 || len(p.reqs[1].Tools) != 1 || len(p.reqs[2].Tools) != 0 {
		t.Fatalf("tools per request: %d %d %d", len(p.reqs[0].Tools), len(p.reqs[1].Tools), len(p.reqs[2].Tools))
	}
	if added[len(added)-1].Text != "final" || len(ran) != 2 {
		t.Fatalf("added %+v ran %v", added, ran)
	}
}

func TestRunToolsStopsAtMaxStepsEvenIfModelKeepsCalling(t *testing.T) {
	call1 := []llm.Event{call(0, "c1", "echo", `{"Q":"x"}`)}
	p := &scripted{turns: [][]llm.Event{call1, call1}}
	var ran []string
	c := &Client{Ref: "x/m", Model: "m", P: p}
	added, _, err := c.RunTools(context.Background(), llm.Request{}, []Tool{echoTool(&ran)}, 2, Turn{})
	if err != nil || len(p.reqs) != 2 || len(added) != 3 || len(added[2].ToolCalls) != 0 {
		t.Fatalf("made %d requests, added %+v (err %v)", len(p.reqs), added, err)
	}
}

func TestRunToolsErrors(t *testing.T) {
	c := &Client{Ref: "x/m", Model: "m", P: &scripted{turns: [][]llm.Event{
		{text("partial"), {Type: llm.EventError, Err: errors.New("HTTP 500")}},
	}}}
	if _, _, err := c.RunTools(context.Background(), llm.Request{}, nil, 2, Turn{}); err == nil || err.Error() != "HTTP 500" {
		t.Fatalf("event error: %v", err)
	}
	c = &Client{Ref: "x/m", Model: "m", P: &scripted{}}
	if _, _, err := c.RunTools(context.Background(), llm.Request{}, nil, 2, Turn{}); err == nil {
		t.Fatal("stream error swallowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c = &Client{Ref: "x/m", Model: "m", P: &scripted{turns: [][]llm.Event{{text("hi")}}}}
	if _, _, err := c.RunTools(ctx, llm.Request{}, nil, 2, Turn{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestCompleteReportsProviderError(t *testing.T) {
	c := &Client{Ref: "x/m", Model: "m", P: &scripted{turns: [][]llm.Event{
		{text("a"), {Type: llm.EventError, Err: errors.New("boom")}},
	}}}
	if _, _, err := c.Complete(context.Background(), "", nil); err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}
