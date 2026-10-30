package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/llm/chat"
)

// Tool is a function the model may call: its declaration and the code that runs it.
type Tool struct {
	Spec llm.ToolSpec
	Run  func(ctx context.Context, args json.RawMessage) (string, error)
}

// Turn holds callbacks for streaming UIs; any may be nil.
type Turn struct {
	OnText, OnThinking func(string)
	OnToolCall         func(name, args string)
	OnToolResult       func(name, result string, err error)
}

const defaultMaxSteps = 10

// RunTools streams turns, runs the tools the model requests, feeds the results back, and stops when the model
// answers without calling a tool. At most maxSteps turns are made (default 10); the last one is sent without
// tools so the model has to answer. It returns the messages appended to req.Messages (assistant turns and
// tool results) and the usage summed over every turn. A tool that fails reports its error to the model.
func (c *Client) RunTools(ctx context.Context, req llm.Request, tools []Tool, maxSteps int, t Turn) ([]llm.Message, llm.Usage, error) {
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	byName := map[string]Tool{}
	req.Tools = nil
	for _, tool := range tools {
		byName[tool.Spec.Name] = tool
		req.Tools = append(req.Tools, tool.Spec)
	}
	history := append([]llm.Message(nil), req.Messages...)
	var added []llm.Message
	var used llm.Usage
	for step := 1; ; step++ {
		turnReq := req
		turnReq.Messages = history
		if step == maxSteps {
			turnReq.Tools = nil
		}
		msg, u, err := c.runTurn(ctx, turnReq, t)
		used.Add(u)
		if err != nil {
			return added, used, err
		}
		if step == maxSteps {
			msg.ToolCalls, msg.Native = nil, "" // none were offered; a call now has no result to pair with
		}
		added = append(added, msg)
		history = append(history, msg)
		if len(msg.ToolCalls) == 0 {
			return added, used, nil
		}
		for _, call := range msg.ToolCalls {
			res := runTool(ctx, byName, call, t)
			added = append(added, res)
			history = append(history, res)
		}
	}
}

func runTool(ctx context.Context, byName map[string]Tool, call llm.ToolCall, t Turn) llm.Message {
	if t.OnToolCall != nil {
		t.OnToolCall(call.Name, call.Args)
	}
	var result string
	var err error
	if tool, ok := byName[call.Name]; ok {
		args := json.RawMessage(call.Args)
		if strings.TrimSpace(call.Args) == "" {
			args = json.RawMessage("{}")
		}
		result, err = tool.Run(ctx, args)
	} else {
		err = fmt.Errorf("unknown tool %q", call.Name)
	}
	if err != nil {
		result = "error: " + err.Error()
	}
	if t.OnToolResult != nil {
		t.OnToolResult(call.Name, result, err)
	}
	return llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, ToolName: call.Name, Text: result}
}

// runTurn streams one assistant turn and assembles its text, reasoning and tool calls.
func (c *Client) runTurn(ctx context.Context, req llm.Request, t Turn) (llm.Message, llm.Usage, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch, err := c.Stream(ctx, req)
	if err != nil {
		return llm.Message{}, llm.Usage{}, err
	}
	defer drain(ch)
	acc := chat.NewCallAccumulator()
	var text, reasoning strings.Builder
	var native []string
	var used llm.Usage
	for ev := range ch {
		switch ev.Type {
		case llm.EventTextDelta:
			text.WriteString(ev.TextDelta)
			if t.OnText != nil {
				t.OnText(ev.TextDelta)
			}
		case llm.EventThinkingDelta:
			reasoning.WriteString(ev.Thinking)
			if t.OnThinking != nil {
				t.OnThinking(ev.Thinking)
			}
		case llm.EventNative:
			if ev.Native != "" {
				native = append(native, ev.Native)
			}
		case llm.EventToolCall:
			if ev.Call != nil {
				acc.Add(ev.Seq, *ev.Call)
			}
		case llm.EventUsage:
			if ev.Usage != nil {
				used.Add(*ev.Usage)
			}
		case llm.EventError:
			return llm.Message{}, used, scrub(ev.Err)
		}
	}
	if err := ctx.Err(); err != nil {
		return llm.Message{}, used, err
	}
	calls := acc.Finish()
	named := nameCalls(calls)
	msg := llm.Message{Role: llm.RoleAssistant, Text: text.String(), Reasoning: reasoning.String(), ToolCalls: calls, Route: c.Ref}
	if len(native) > 0 && !named {
		msg.Native = "[" + strings.Join(native, ",") + "]"
	}
	return msg, used, nil
}

// nameCalls gives calls without an id one, so each result can be paired; it reports whether it did.
func nameCalls(calls []llm.ToolCall) bool {
	changed := false
	for i := range calls {
		if calls[i].ID == "" {
			calls[i].ID = fmt.Sprintf("call_%d", i+1)
			changed = true
		}
	}
	return changed
}
