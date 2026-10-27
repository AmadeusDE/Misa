package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// countCacheControl counts cache_control directives in the marshaled body.
func countCacheControl(t *testing.T, body []byte) int {
	t.Helper()
	return strings.Count(string(body), `"cache_control"`)
}

func cacheReq() llm.Request {
	return llm.Request{
		Model:    "anthropic/claude-haiku-5.5",
		System:   "you are ugh",
		CacheTTL: "1h",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "start"},
			{Role: llm.RoleAssistant, Text: "working"},
			{Role: llm.RoleUser, Text: "continue"},
		},
	}
}

// Anthropic models via OpenRouter must carry cache_control breakpoints on
// content parts (the OpenRouter-documented form): the system prompt and the
// stable message anchors, at most 4 per request.
func TestBuildRequestAnthropicCacheControl(t *testing.T) {
	body, err := buildRequest(cacheReq(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := countCacheControl(t, body); n != 4 {
		t.Fatalf("cache_control count = %d, want 4 (system + 3 stable anchors)\n%s", n, body)
	}
	var parsed struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(parsed.Messages))
	}
	sys := parsed.Messages[0]
	if sys.Role != "system" {
		t.Fatalf("first message role = %q, want system", sys.Role)
	}
	part := sys.Content.([]any)[0].(map[string]any)
	if part["text"] != "you are ugh" {
		t.Fatalf("system part = %v", part)
	}
	cc := part["cache_control"].(map[string]any)
	if cc["type"] != "ephemeral" || cc["ttl"] != "1h" {
		t.Fatalf("system cache_control = %v, want ephemeral ttl 1h", cc)
	}
	for i, m := range parsed.Messages[1:] {
		p, ok := m.Content.([]any)
		if !ok || len(p) != 1 {
			t.Fatalf("message %d content = %#v, want one text part", i, m.Content)
		}
		text := p[0].(map[string]any)
		cc, ok := text["cache_control"].(map[string]any)
		if !ok || cc["type"] != "ephemeral" || cc["ttl"] != "1h" {
			t.Fatalf("message %d cache_control = %#v, want ephemeral ttl 1h", i, text["cache_control"])
		}
	}
}

// The 5m default TTL omits the ttl key.
func TestBuildRequestCacheControl5m(t *testing.T) {
	req := cacheReq()
	req.CacheTTL = "5m"
	body, err := buildRequest(req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"ttl"`) {
		t.Fatalf("5m body carries a ttl field:\n%s", body)
	}
	if n := countCacheControl(t, body); n != 4 {
		t.Fatalf("cache_control count = %d, want 4", n)
	}
}

// Long histories still fit the 4-breakpoint budget.
func TestBuildRequestCacheControlBudget(t *testing.T) {
	req := cacheReq()
	for i := 0; i < 30; i++ {
		req.Messages = append(req.Messages,
			llm.Message{Role: llm.RoleUser, Text: "more"},
			llm.Message{Role: llm.RoleAssistant, Text: "ok"},
		)
	}
	body, err := buildRequest(req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := countCacheControl(t, body); n > 4 {
		t.Fatalf("cache_control count = %d, want at most 4", n)
	}
}

// Tool results and assistant tool-call turns keep their pairing fields: they
// never become content parts.
func TestBuildRequestCacheSkipsToolTurns(t *testing.T) {
	req := cacheReq()
	req.Messages = append(req.Messages,
		llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "bash", Args: "{}"}}},
		llm.Message{Role: llm.RoleTool, ToolCallID: "1", Text: "result"},
	)
	body, err := buildRequest(req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"tool_call_id":"1"`) || !strings.Contains(string(body), `"tool_calls"`) {
		t.Fatalf("tool pairing lost:\n%s", body)
	}
	if n := countCacheControl(t, body); n > 4 {
		t.Fatalf("cache_control count = %d, want at most 4", n)
	}
}

// Without a TTL the wire shape stays plain strings; non-Anthropic models never
// get cache_control even when a TTL is set.
func TestBuildRequestNoCacheControlForOthers(t *testing.T) {
	for name, mutate := range map[string]func(*llm.Request){
		"ttl off":       func(r *llm.Request) { r.CacheTTL = "" },
		"non-anthropic": func(r *llm.Request) { r.Model = "openai/gpt-6-luna" },
		"glm":           func(r *llm.Request) { r.Model = "z-ai/glm-5.2" },
	} {
		t.Run(name, func(t *testing.T) {
			req := cacheReq()
			mutate(&req)
			body, err := buildRequest(req, nil)
			if err != nil {
				t.Fatal(err)
			}
			if n := countCacheControl(t, body); n != 0 {
				t.Fatalf("cache_control count = %d, want 0:\n%s", n, body)
			}
			var parsed struct {
				Messages []struct {
					Content any `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatal(err)
			}
			for i, m := range parsed.Messages {
				if _, ok := m.Content.(string); !ok {
					t.Fatalf("message %d content = %#v, want a plain string", i, m.Content)
				}
			}
		})
	}
}

// Cached-token counts arrive under several names: the OpenRouter-normalized
// prompt_tokens_details.cached_tokens and the Anthropic-native
// cache_read counters. Cache writes are never reported as cached.
func TestDecodeUsageCachedTokens(t *testing.T) {
	for name, tc := range map[string]struct {
		usage  string
		cached int
	}{
		"prompt_tokens_details": {usage: `{"prompt_tokens":100,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":64}}`, cached: 64},
		"cache_read_input":      {usage: `{"prompt_tokens":100,"completion_tokens":5,"cache_read_input_tokens":42}`, cached: 42},
		"cache_read":            {usage: `{"prompt_tokens":100,"completion_tokens":5,"cache_read":33}`, cached: 33},
		"cache_write_only":      {usage: `{"prompt_tokens":100,"completion_tokens":5,"cache_write":21}`, cached: 0},
		"cache_creation_input":  {usage: `{"prompt_tokens":100,"completion_tokens":5,"cache_creation_input_tokens":17}`, cached: 0},
		"read wins over write":  {usage: `{"prompt_tokens":100,"completion_tokens":5,"cache_read":90,"cache_write":10}`, cached: 90},
	} {
		t.Run(name, func(t *testing.T) {
			evs := decodeEvent(`{"choices":[],"usage":` + tc.usage + `}`)
			if len(evs) != 1 || evs[0].Type != llm.EventUsage {
				t.Fatalf("events = %+v, want one usage event", evs)
			}
			if evs[0].Usage.Cached != tc.cached {
				t.Fatalf("cached = %d, want %d", evs[0].Usage.Cached, tc.cached)
			}
			if evs[0].Usage.In != 100 || evs[0].Usage.Out != 5 {
				t.Fatalf("usage = %+v, want in 100 out 5", evs[0].Usage)
			}
		})
	}
}

func TestBuildRequestCacheKeepsReasoning(t *testing.T) {
	req := llm.Request{
		Model: "anthropic/claude-haiku-5.5", CacheTTL: "5m",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Text: "start"},
			{Role: llm.RoleAssistant, Text: "answer", Reasoning: "thought it through"},
		},
	}
	body, err := buildRequest(req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"reasoning_content":"thought it through"`) || countCacheControl(t, body) == 0 {
		t.Fatalf("reasoning lost on a cached assistant turn: %s", body)
	}
}
