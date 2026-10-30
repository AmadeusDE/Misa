// Package aihelp answers questions about shell commands with the model chosen for the "help" feature: the
// aihelp command, its --fix mode, Misa's AI command correction and the help viewer's "Ask AI".
//
// Ported from Gemhelp (github.com/AmadeusDE/Gemhelp, the author's own project, OSL-3.0) on internal/ai
// instead of the Gemini SDK. Man and tldr pages come from internal/help.
package aihelp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	archWikiAPI  = "https://wiki.archlinux.org/api.php"
	maxWikiBytes = 4 << 20
)

// Wiki is a client for the Arch Wiki's MediaWiki API. The zero value talks to wiki.archlinux.org.
type Wiki struct {
	BaseURL string
	HTTP    *http.Client
}

func (w Wiki) api() string {
	if w.BaseURL != "" {
		return w.BaseURL
	}
	return archWikiAPI
}

func (w Wiki) get(ctx context.Context, query url.Values, out any) error {
	query.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.api()+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	client := w.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("arch wiki api returned status %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxWikiBytes)).Decode(out)
}

// Search returns the titles of the pages matching query.
func (w Wiki) Search(ctx context.Context, query string) ([]string, error) {
	var data struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	err := w.get(ctx, url.Values{"action": {"query"}, "list": {"search"}, "srsearch": {query}}, &data)
	if err != nil {
		return nil, err
	}
	titles := make([]string, 0, len(data.Query.Search))
	for _, item := range data.Query.Search {
		titles = append(titles, item.Title)
	}
	return titles, nil
}

// Page returns a page as markdown.
func (w Wiki) Page(ctx context.Context, title string) (string, error) {
	var data struct {
		Parse struct {
			Title    string `json:"title"`
			Wikitext struct {
				Content string `json:"*"`
			} `json:"wikitext"`
		} `json:"parse"`
	}
	err := w.get(ctx, url.Values{"action": {"parse"}, "page": {title}, "prop": {"wikitext"}}, &data)
	if err != nil {
		return "", err
	}
	if data.Parse.Title == "" {
		return "", fmt.Errorf("arch wiki page %q not found", title)
	}
	return fmt.Sprintf("# Arch Wiki: %s\n\n%s", data.Parse.Title, cleanWikitext(data.Parse.Wikitext.Content)), nil
}

var wikitextRules = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`'''(.*?)'''`), "**$1**"},
	{regexp.MustCompile(`''(.*?)''`), "*$1*"},
	{regexp.MustCompile(`(?m)^=====(.*?)=====`), "#### $1"},
	{regexp.MustCompile(`(?m)^====(.*?)====`), "#### $1"},
	{regexp.MustCompile(`(?m)^===(.*?)===`), "### $1"},
	{regexp.MustCompile(`(?m)^==(.*?)==`), "## $1"},
	{regexp.MustCompile(`\[\[([^|\]]+)\|([^\]]+)\]\]`), "$2"},
	{regexp.MustCompile(`\[\[([^\]]+)\]\]`), "$1"},
	{regexp.MustCompile(`\[https?://\S+\s+([^\]]+)\]`), "$1"},
}

var (
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHTMLTag     = regexp.MustCompile(`<[^>]*>`)
	reBlankRuns   = regexp.MustCompile(`\n{3,}`)
)

func cleanWikitext(w string) string {
	for _, r := range wikitextRules {
		w = r.re.ReplaceAllString(w, r.with)
	}
	w = cleanTemplates(w)
	w = reHTMLComment.ReplaceAllString(w, "")
	w = reHTMLTag.ReplaceAllString(w, "")
	w = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&").Replace(w)
	return strings.TrimSpace(reBlankRuns.ReplaceAllString(w, "\n\n"))
}

// cleanTemplates rewrites {{templates}} from the innermost out so nesting works; unknown ones are dropped.
func cleanTemplates(s string) string {
	for {
		start := strings.LastIndex(s, "{{")
		if start == -1 {
			return s
		}
		end := strings.Index(s[start:], "}}")
		if end == -1 {
			return s
		}
		end += start
		parts := strings.Split(s[start+2:end], "|")
		s = s[:start] + templateText(parts) + s[end+2:]
	}
}

func templateText(parts []string) string {
	if len(parts) < 2 {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(parts[0]))
	switch name {
	case "ic":
		return "`" + parts[1] + "`"
	case "hc":
		return "\n```\n" + strings.Join(parts[1:], "\n") + "\n```\n"
	case "note", "tip", "warning", "important":
		text := parts[1]
		for _, p := range parts[1:] {
			if trimmed := strings.TrimSpace(p); strings.HasPrefix(trimmed, "text=") {
				text = strings.TrimPrefix(trimmed, "text=")
				break
			} else if !strings.Contains(p, "=") {
				text = p
				break
			}
		}
		return fmt.Sprintf("\n> **%s**: %s\n", strings.ToUpper(name), text)
	}
	return ""
}
