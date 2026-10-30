package aihelp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

// FallbackKey is the ai.yaml models entry tried when the help model keeps failing.
const FallbackKey = "help_fallback"

const (
	maxSteps      = 6
	attempts      = 3
	initialDelay  = 2 * time.Second
	helpSystemFmt = "You are a terminal command help assistant. " +
		"You help the user understand commands and resolve command-line tasks. " +
		"You have tools for fetching man pages (man_page), TLDR pages (tldr_page) and Arch Wiki content (arch_wiki). " +
		"Use them to verify flags and syntax instead of guessing; a tool that fails just means that source is unavailable. " +
		"The user's system is %s. " +
		"Be concise, clear and direct. Use Markdown for styling. " +
		"Keep answers focused and practical, with code block examples."
)

// Helper answers help questions. Build one with New; tests fill the fields directly.
type Helper struct {
	// Models are the refs tried in order: the help model, then help_fallback.
	Models []string
	Open   func(ref string) (*ai.Client, error)
	Tools  []ai.Tool
	Cache  *Cache
	// Notice receives retry and fallback messages (nil discards them).
	Notice io.Writer
	// Delay is the first backoff, doubled after each retry. Sleep waits; nil means a real timer.
	Delay time.Duration
	Sleep func(ctx context.Context, d time.Duration) error
}

// Query is one question. Command may be empty (a free question); Context is extra material such as a man page.
type Query struct {
	Command, Question, Context string
	NoCache                    bool
}

// Available reports whether a help model is chosen, so callers can offer AI features only when they can work.
func Available() bool {
	_, err := ai.ModelFor(ai.FeatureHelp)
	return err == nil
}

// New builds a Helper for the help model, or model when it is not empty. The error names `aiconfig` when no
// model is chosen.
func New(model string) (*Helper, error) {
	s, err := ai.LoadSettings()
	if err != nil {
		return nil, err
	}
	if model == "" {
		if model, err = ai.ModelFor(ai.FeatureHelp); err != nil {
			return nil, err
		}
	}
	h := &Helper{Models: []string{model}, Open: ai.Open, Tools: Tools(Wiki{}), Cache: DefaultCache(), Delay: initialDelay}
	if fb := s.Models[FallbackKey]; fb != "" && fb != model {
		h.Models = append(h.Models, fb)
	}
	return h, nil
}

func helpSystem() string { return fmt.Sprintf(helpSystemFmt, runtime.GOOS) }

func (q Query) prompt() string {
	var p string
	switch {
	case q.Command != "" && q.Question != "":
		p = fmt.Sprintf("About the command '%s': %s", q.Command, q.Question)
	case q.Command != "":
		p = fmt.Sprintf("Describe the command '%s'.", q.Command)
	default:
		p = q.Question
	}
	if q.Context != "" {
		p += "\n\nThe user is reading this page:\n\n" + clip(q.Context)
	}
	return p
}

// Ask answers q, streaming the text to onText (nil is fine) and returning it whole. A cached answer is
// delivered in one piece. NoCache skips the lookup but still refreshes the cache.
func (h *Helper) Ask(ctx context.Context, q Query, onText func(string)) (string, error) {
	prompt := q.prompt()
	if prompt == "" {
		return "", errors.New("nothing to ask")
	}
	if !q.NoCache && len(h.Models) > 0 {
		if cached, ok := h.Cache.Get(h.Models[0], prompt); ok {
			if onText != nil {
				onText(cached)
			}
			return cached, nil
		}
	}
	text, err := h.run(ctx, helpSystem(), prompt, onText)
	if err != nil {
		return "", err
	}
	_ = h.Cache.Put(h.Models[0], prompt, text)
	return text, nil
}

// Fix asks for a corrected version of a failed command. Fixes are never cached: the output decides them.
func (h *Helper) Fix(ctx context.Context, cmd, output string, exit int) (fixed, explanation string, err error) {
	if strings.TrimSpace(cmd) == "" {
		return "", "", errors.New("no command to fix")
	}
	text, err := h.run(ctx, fixSystem, fixPrompt(cmd, output, exit), nil)
	if err != nil {
		return "", "", err
	}
	return ParseFix(text)
}

// run tries each model in turn; the first to answer wins.
func (h *Helper) run(ctx context.Context, system, prompt string, onText func(string)) (text string, err error) {
	if len(h.Models) == 0 {
		return "", errors.New("no model chosen for help: run `aiconfig` to pick one")
	}
	for i, ref := range h.Models {
		text, err = h.runModel(ctx, ref, system, prompt, onText)
		if err == nil {
			return text, nil
		}
		if ctx.Err() != nil {
			return "", err
		}
		if i+1 < len(h.Models) {
			h.notef("%s failed: %v. Trying %s...\n", ref, err, h.Models[i+1])
		}
	}
	return "", err
}

// runModel runs one model, retrying with backoff while the provider answers 429 or 5xx and nothing has been
// shown yet (a retry would repeat what the reader has seen).
func (h *Helper) runModel(ctx context.Context, ref, system, prompt string, onText func(string)) (string, error) {
	client, err := h.Open(ref)
	if err != nil {
		return "", err
	}
	delay := h.Delay
	for attempt := 1; ; attempt++ {
		var out strings.Builder
		spoke := false
		turn := ai.Turn{
			OnText: func(s string) {
				out.WriteString(s)
				spoke = spoke || s != ""
				if onText != nil {
					onText(s)
				}
			},
			OnToolCall: func(name, args string) {
				h.notef("  [%s %s]\n", name, args)
				if spoke && onText != nil {
					onText("\n\n") // keep a remark made before a lookup apart from the answer
				}
				spoke = false
			},
		}
		req := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Text: prompt}}, Foreground: true}
		added, _, err := client.RunTools(ctx, req, h.Tools, maxSteps, turn)
		if err == nil {
			return finalText(added, out.String()), nil
		}
		if attempt >= attempts || out.Len() > 0 || !Retryable(err) {
			return "", err
		}
		h.notef("Rate limited or unavailable (%s). Retrying in %v...\n", ref, delay)
		if err := h.sleep(ctx, delay); err != nil {
			return "", err
		}
		delay *= 2
	}
}

// finalText is the last assistant turn: what the model said after its tool calls, which is the answer.
func finalText(added []llm.Message, streamed string) string {
	for i := len(added) - 1; i >= 0; i-- {
		if added[i].Role == llm.RoleAssistant {
			return strings.TrimSpace(added[i].Text)
		}
	}
	return strings.TrimSpace(streamed)
}

func (h *Helper) sleep(ctx context.Context, d time.Duration) error {
	if h.Sleep != nil {
		return h.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (h *Helper) notef(format string, args ...any) {
	if h.Notice != nil {
		fmt.Fprintf(h.Notice, format, args...)
	}
}

// Retryable reports whether err is a rate limit (429) or a server error (5xx).
func Retryable(err error) bool {
	var he *llm.HTTPError
	if errors.As(err, &he) {
		return he.StatusCode == 429 || he.StatusCode >= 500
	}
	msg := strings.ToUpper(err.Error())
	for _, s := range []string{"HTTP 429", "HTTP 5", "RESOURCE_EXHAUSTED", "RATE_LIMIT", "RATE LIMIT"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
