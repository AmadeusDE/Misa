package aihelp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ai"
	"github.com/AmadeusDE/Misa/internal/help"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
)

const maxToolBytes = 24 << 10

func clip(s string) string {
	if len(s) <= maxToolBytes {
		return s
	}
	cut := maxToolBytes
	for cut > 0 && s[cut]&0xC0 == 0x80 { // do not split a UTF-8 sequence
		cut--
	}
	return s[:cut] + "\n[truncated]"
}

func schema(props map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func stringProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

// Tools are what the model may call: man_page and tldr_page from internal/help, and arch_wiki.
func Tools(wiki Wiki) []ai.Tool {
	return []ai.Tool{
		{
			Spec: llm.ToolSpec{
				Name:        "man_page",
				Description: "Gets the local man page for a command: detailed flags and usage.",
				Parameters: schema(map[string]any{
					"name":    stringProp("The command name, e.g. 'ls' or 'grep'."),
					"section": stringProp("Optional manual section, e.g. '1' or '5'."),
				}, "name"),
			},
			Run: manPage,
		},
		{
			Spec: llm.ToolSpec{
				Name:        "tldr_page",
				Description: "Gets the TLDR page for a command: a short description and common examples.",
				Parameters:  schema(map[string]any{"name": stringProp("The command name, e.g. 'tar'.")}, "name"),
			},
			Run: tldrPage,
		},
		{
			Spec: llm.ToolSpec{
				Name: "arch_wiki",
				Description: "Searches the Arch Wiki and returns the best matching page, plus the titles of other matches. " +
					"Good for configuration and troubleshooting beyond a single command.",
				Parameters: schema(map[string]any{"query": stringProp("What to look up, e.g. 'systemd timers'.")}, "query"),
			},
			Run: wiki.tool,
		},
	}
}

type nameArgs struct {
	Name    string `json:"name"`
	Section string `json:"section"`
}

func parseNameArgs(raw json.RawMessage) (nameArgs, error) {
	var a nameArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return a, fmt.Errorf("bad arguments: %w", err)
	}
	if a.Name = strings.TrimSpace(a.Name); a.Name == "" {
		return a, errors.New("missing name parameter")
	}
	return a, nil
}

func manPage(_ context.Context, raw json.RawMessage) (string, error) {
	a, err := parseNameArgs(raw)
	if err != nil {
		return "", err
	}
	path, ok := help.FindManPageIn(a.Name, strings.TrimSpace(a.Section))
	if !ok {
		return "", fmt.Errorf("man page for %q not found locally", a.Name)
	}
	page, err := help.ParseManPage(path)
	if err != nil {
		return "", err
	}
	return clip(page), nil
}

func tldrPage(_ context.Context, raw json.RawMessage) (string, error) {
	a, err := parseNameArgs(raw)
	if err != nil {
		return "", err
	}
	page, err := help.GetTldrPage(a.Name)
	if err != nil {
		return "", err
	}
	return clip(page), nil
}

func (w Wiki) tool(ctx context.Context, raw json.RawMessage) (string, error) {
	var a struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("bad arguments: %w", err)
	}
	if a.Query = strings.TrimSpace(a.Query); a.Query == "" {
		return "", errors.New("missing query parameter")
	}
	titles, err := w.Search(ctx, a.Query)
	if err != nil {
		return "", err
	}
	if len(titles) == 0 {
		return "no Arch Wiki pages match " + a.Query, nil
	}
	best := titles[0]
	for _, t := range titles {
		if strings.EqualFold(t, a.Query) {
			best = t
			break
		}
	}
	page, err := w.Page(ctx, best)
	if err != nil {
		return "", err
	}
	return clip(page) + "\n\nOther matches: " + strings.Join(titles, ", "), nil
}
