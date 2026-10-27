package llm

import (
	"testing"
)

func TestCacheTTL(t *testing.T) {
	if got := CacheTTL(0); got != "5m" {
		t.Errorf("CacheTTL(0) = %q, want 5m", got)
	}
	if got := CacheTTL(2); got != "1h" {
		t.Errorf("CacheTTL(2) = %q, want 1h (long waits need the extended cache)", got)
	}
}

func TestCacheBreakpointsSelection(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Text: "start the task"},
		{Role: RoleAssistant, Text: "on it", ToolCalls: []ToolCall{{ID: "1", Name: "bash"}}},
		{Role: RoleTool, ToolCallID: "1", Text: "output"},
		{Role: RoleAssistant, Text: "done"},
	}
	got := CacheBreakpoints(msgs, CacheMaxBreakpoints-1)
	if len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Fatalf("breakpoints = %v, want [0 3] (skip tool_call/tool turns)", got)
	}
}

func TestCacheBreakpointsMax(t *testing.T) {
	var msgs []Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, Message{Role: RoleUser, Text: "turn"})
	}
	got := CacheBreakpoints(msgs, CacheMaxBreakpoints-1)
	if len(got) > CacheMaxBreakpoints-1 {
		t.Fatalf("breakpoints = %v, exceeds the budget", got)
	}
	if len(got) != 3 {
		t.Fatalf("breakpoints = %v, want the first and last two anchors", got)
	}
	if got[0] != 0 {
		t.Fatalf("breakpoints = %v, want the preamble anchor first", got)
	}
	if got[1] != len(msgs)-1 {
		t.Fatalf("breakpoints = %v, want the final message among them", got)
	}
}

func TestWithTextContentParts(t *testing.T) {
	m := Message{Role: RoleUser, Text: "hi"}
	v := WithTextContentParts(m, true, "1h")
	blob := v.(map[string]any)
	part := blob["content"].([]any)[0].(map[string]any)
	cc := part["cache_control"].(map[string]any)
	if cc["type"] != "ephemeral" || cc["ttl"] != "1h" {
		t.Fatalf("cache_control = %v, want ephemeral/1h", cc)
	}
	if part["text"] != "hi" || blob["role"] != "user" {
		t.Fatalf("part = %v", part)
	}
	// Off: the plain message comes back untouched.
	if _, ok := WithTextContentParts(m, false, "5m").(Message); !ok {
		t.Fatalf("no-breakpoint message was rewritten: %v", WithTextContentParts(m, false, "5m"))
	}
	// Tool results keep their pairing fields and never become parts.
	tool := Message{Role: RoleTool, ToolCallID: "1", Text: "out"}
	if _, ok := WithTextContentParts(tool, true, "5m").(Message); !ok {
		t.Fatalf("tool message was rewritten: %v", WithTextContentParts(tool, true, "5m"))
	}
	// The short TTL omits ttl entirely.
	v = WithTextContentParts(m, true, "5m")
	cc = v.(map[string]any)["content"].([]any)[0].(map[string]any)["cache_control"].(map[string]any)
	if _, ok := cc["ttl"]; ok {
		t.Fatalf("5m cache_control = %v, want no ttl key", cc)
	}
}

func TestWithSystemContentParts(t *testing.T) {
	if s, ok := WithSystemContentParts("sys", "").(string); !ok || s != "sys" {
		t.Fatalf("off system = %v, want the plain string", s)
	}
	parts, ok := WithSystemContentParts("sys", "5m").([]any)
	if !ok {
		t.Fatalf("5m system = %v, want content parts", parts)
	}
	part := parts[0].(map[string]any)
	if part["text"] != "sys" {
		t.Fatalf("system part = %v", part)
	}
	if _, ok := part["cache_control"].(map[string]any)["ttl"]; ok {
		t.Fatalf("5m system cache_control = %v, want no ttl key", part["cache_control"])
	}
	parts, ok = WithSystemContentParts("sys", "1h").([]any)
	if !ok {
		t.Fatalf("1h system = %v, want content parts", parts)
	}
	part = parts[0].(map[string]any)
	if part["text"] != "sys" || part["cache_control"].(map[string]any)["ttl"] != "1h" {
		t.Fatalf("system part = %v", part)
	}
}

func TestWithTextContentPartsKeepsReasoning(t *testing.T) {
	m := Message{Role: RoleAssistant, Text: "hi", Reasoning: "why"}
	blob := WithTextContentParts(m, true, "5m").(map[string]any)
	if blob["reasoning_content"] != "why" {
		t.Fatalf("blob = %v, want reasoning_content kept", blob)
	}
}

func TestCacheableSkipsNativeReplay(t *testing.T) {
	if cacheable(Message{Role: RoleAssistant, Text: "hi", Native: `{"x":1}`}) {
		t.Fatal("a turn with a provider-native replay payload must not be rebuilt as content parts")
	}
}
