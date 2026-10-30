package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"golang.org/x/net/html"
)

const maxSearchResults = 8

// duckduckgoURL is DuckDuckGo's HTML endpoint; tests point it at a fixture server.
var duckduckgoURL = "https://html.duckduckgo.com/html/"

// SearchResult is one hit of a web search.
type SearchResult struct{ Title, URL, Snippet string }

// WebSearch runs query on the engine configured in s: DuckDuckGo's HTML page (no key) or a SearXNG instance's JSON.
func WebSearch(ctx context.Context, s Search, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("web_search needs a query")
	}
	var results []SearchResult
	var err error
	switch s.Engine {
	case "", "duckduckgo":
		results, err = searchDuckDuckGo(ctx, query)
	case "searxng":
		results, err = searchSearxng(ctx, s.SearxngURL, query)
	default:
		return nil, errors.New("web search is off")
	}
	if len(results) > maxSearchResults {
		results = results[:maxSearchResults]
	}
	return results, err
}

func searchDuckDuckGo(ctx context.Context, query string) ([]SearchResult, error) {
	body, _, err := httpGet(ctx, duckduckgoURL+"?q="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	return parseDuckDuckGo(string(body)), nil
}

// parseDuckDuckGo reads results off the HTML endpoint: a.result__a is the title link, .result__snippet its text.
func parseDuckDuckGo(page string) []SearchResult {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	var results []SearchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "a" && hasClass(n, "result__a"):
				if link := resultLink(attr(n, "href")); link != "" {
					results = append(results, SearchResult{Title: nodeText(n), URL: link})
				}
				return
			case hasClass(n, "result__snippet") && len(results) > 0 && results[len(results)-1].Snippet == "":
				results[len(results)-1].Snippet = nodeText(n)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results
}

// resultLink unwraps DuckDuckGo's redirect (//duckduckgo.com/l/?uddg=<url>) and drops ads.
func resultLink(href string) string {
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if target := u.Query().Get("uddg"); target != "" {
		return target
	}
	if strings.Contains(u.Host, "duckduckgo.com") || u.Scheme != "https" && u.Scheme != "http" {
		return ""
	}
	return u.String()
}

func searchSearxng(ctx context.Context, base, query string) ([]SearchResult, error) {
	if strings.TrimSpace(base) == "" {
		return nil, errors.New("searxng_url is not set in ai.yaml")
	}
	base = strings.TrimSuffix(strings.TrimRight(base, "/"), "/search")
	body, _, err := httpGet(ctx, base+"/search?format=json&q="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	var reply struct {
		Results []struct{ Title, URL, Content string }
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return nil, fmt.Errorf("searxng did not answer JSON (enable the json format): %w", err)
	}
	var out []SearchResult
	for _, r := range reply.Results {
		out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return out, nil
}

func formatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "no results"
	}
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, r.Title, r.URL)
		if r.Snippet != "" {
			fmt.Fprintf(&b, "   %s\n", r.Snippet)
		}
	}
	return strings.TrimSpace(b.String())
}

// WebTools are the web_search and fetch_url tools for chat, or nil when search is off.
func WebTools(s Search) []Tool {
	if s.Engine == "off" {
		return nil
	}
	search := Tool{
		Spec: llmSpec("web_search", "Search the web and return titles, URLs and snippets. Use fetch_url to read a result.",
			map[string]any{"query": map[string]any{"type": "string", "description": "the search query"}}, "query"),
		Run: func(ctx context.Context, args json.RawMessage) (string, error) {
			var a struct{ Query string }
			if err := json.Unmarshal(args, &a); err != nil {
				return "", fmt.Errorf("bad arguments: %w", err)
			}
			results, err := WebSearch(ctx, s, a.Query)
			if err != nil {
				return "", err
			}
			return formatResults(results), nil
		},
	}
	return []Tool{search, fetchTool()}
}

func llmSpec(name, desc string, props map[string]any, required ...string) llm.ToolSpec {
	return llm.ToolSpec{Name: name, Description: desc, Parameters: map[string]any{
		"type": "object", "properties": props, "required": required,
	}}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
