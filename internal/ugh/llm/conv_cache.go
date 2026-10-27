package llm

import "time"

// Prompt-cache breakpoint selection shared by the Chat Completions clients.
// The OpenRouter-documented form applies Anthropic cache_control directives to
// message content parts: https://openrouter.ai/docs/features/prompt-caching
// (at most CacheMaxBreakpoints breakpoints per request, ttl "5m" by default,
// "1h" for conversations with long waits between turns).

// CacheMaxBreakpoints is the per-request cache_control budget: one goes to the
// system prompt, the rest to stable message anchors.
const CacheMaxBreakpoints = 4

// CacheLongGap is a pause between model calls long enough to expire the short
// cache before the next turn reads it.
const CacheLongGap = 5 * time.Minute

// CacheTTL reports the prompt-cache TTL to request for a conversation with
// longWaits long pauses between model calls (subagent waits and other long
// tools): "1h" then, since the short cache would expire unread between turns,
// "5m" otherwise.
func CacheTTL(longWaits int) string {
	if longWaits > 0 {
		return "1h"
	}
	return "5m"
}

// CacheBreakpoints picks the messages carrying cache_control breakpoints: the
// first cacheable message (the stable preamble anchor) and the last cacheable
// ones (where each turn appends), for at most max breakpoints total. Tool
// results and assistant tool-call turns cannot carry content parts without
// losing their pairing fields, and turns with a provider-native replay payload
// stay in their plain wire form, so they are skipped.
func CacheBreakpoints(msgs []Message, max int) []int {
	if max <= 0 {
		return nil
	}
	var out []int
	add := func(i int) {
		for _, j := range out {
			if j == i {
				return
			}
		}
		out = append(out, i)
	}
	for i := range msgs {
		if cacheable(msgs[i]) {
			add(i)
			break
		}
	}
	tail := 0
	for i := len(msgs) - 1; i >= 0 && tail < 2; i-- {
		if cacheable(msgs[i]) {
			add(i)
			tail++
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func cacheable(msg Message) bool {
	if msg.Text == "" || len(msg.ToolCalls) > 0 || msg.ToolCallID != "" || msg.Native != "" {
		return false
	}
	return msg.Role == RoleUser || msg.Role == RoleAssistant
}

// cacheControl is the cache_control directive value for a breakpoint; nil when
// caching is off (ttl "") for this conversation.
func cacheControl(ttl string) map[string]any {
	if ttl == "" {
		return nil
	}
	cc := map[string]any{"type": "ephemeral"}
	if ttl == "1h" {
		cc["ttl"] = "1h"
	}
	return cc
}

// WithTextContentParts rewrites msg.Content into OpenRouter content parts with
// a cache_control breakpoint when breakpoint is set; the message is returned
// in its plain wire form otherwise.
func WithTextContentParts(msg Message, breakpoint bool, ttl string) any {
	var cc map[string]any
	if breakpoint {
		cc = cacheControl(ttl)
	}
	if !cacheable(msg) || cc == nil {
		msg.Meta = nil // harness metadata never goes on the wire
		return msg
	}
	out := map[string]any{
		"role":    string(msg.Role),
		"content": []any{map[string]any{"type": "text", "text": msg.Text, "cache_control": cc}},
	}
	if msg.Reasoning != "" {
		out["reasoning_content"] = msg.Reasoning
	}
	return out
}

// WithSystemContentParts returns the request's system content: the plain
// string, or content parts with a cache_control breakpoint. The system prompt
// is the biggest and most stable prefix, so it takes the first breakpoint.
func WithSystemContentParts(system string, ttl string) any {
	cc := cacheControl(ttl)
	if system == "" || cc == nil {
		return system
	}
	return []any{map[string]any{"type": "text", "text": system, "cache_control": cc}}
}
