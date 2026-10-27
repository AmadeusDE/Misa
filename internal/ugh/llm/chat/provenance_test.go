package chat

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// Provenance tags are harness metadata: they persist in the session but must
// never appear in a request body, cached or not.
func TestRequestNeverCarriesMessageMeta(t *testing.T) {
	meta := map[string]string{llm.MetaSource: llm.SourceToolOutput, "secret-key": "SECRET-VALUE"}
	msgs := []llm.Message{
		{Role: llm.RoleUser, Text: "hi", Meta: meta},
		{Role: llm.RoleAssistant, Text: "ok", ToolCalls: []llm.ToolCall{{ID: "c1", Name: "ls", Args: "{}"}}, Meta: meta},
		{Role: llm.RoleTool, ToolCallID: "c1", ToolName: "ls", Text: "out", Meta: meta},
		{Role: llm.RoleUser, Text: "again", Meta: meta},
	}
	for _, ttl := range []string{"", "5m", "1h"} {
		body, err := buildRequest(llm.Request{Model: "anthropic/claude-x", System: "sys", Messages: msgs, CacheTTL: ttl}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, leak := range []string{"SECRET-VALUE", "secret-key", "tool_output", `"meta"`, `"Meta"`} {
			if strings.Contains(string(body), leak) {
				t.Errorf("ttl %q: body leaks %q: %s", ttl, leak, body)
			}
		}
	}
}
