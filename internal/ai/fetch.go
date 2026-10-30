package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	fetchLimit   = 2 << 20
	fetchTextCap = 20 << 10
	webUA        = "Mozilla/5.0 (compatible; Misa; +https://github.com/AmadeusDE/Misa)"
)

// webClient serves search and fetch; tests replace it to reach httptest servers.
var webClient = &http.Client{Timeout: 15 * time.Second}

// FetchURL downloads an http(s) page and returns its readable text, capped at about 20 KB.
func FetchURL(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("fetch_url needs an http or https URL, got %q", raw)
	}
	body, ctype, err := httpGet(ctx, u.String())
	if err != nil {
		return "", err
	}
	if ctype == "" {
		ctype = http.DetectContentType(body)
	}
	var text string
	switch {
	case strings.Contains(ctype, "html"):
		text = readableText(string(body))
	case strings.HasPrefix(ctype, "text/") || strings.Contains(ctype, "json") || strings.Contains(ctype, "xml"):
		text = string(body)
	default:
		return "", fmt.Errorf("unsupported content type %s", ctype)
	}
	return capText(text, fetchTextCap), nil
}

func httpGet(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", webUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,text/plain;q=0.8,*/*;q=0.5")
	resp, err := webClient.Do(req)
	if err != nil {
		return nil, "", scrub(errors.New(strings.TrimSpace(urlError(err))))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, req.URL.Host)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchLimit))
	return body, strings.ToLower(resp.Header.Get("Content-Type")), err
}

func urlError(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Op + " " + ue.URL + ": " + ue.Err.Error()
	}
	return err.Error()
}

var (
	skipTags  = map[string]bool{"script": true, "style": true, "noscript": true, "template": true, "svg": true, "iframe": true, "object": true}
	blockTags = map[string]bool{
		"p": true, "div": true, "br": true, "ul": true, "ol": true, "li": true, "tr": true, "table": true, "section": true,
		"article": true, "header": true, "footer": true, "pre": true, "blockquote": true, "hr": true, "nav": true, "aside": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "dt": true, "dd": true, "main": true,
	}
)

// readableText strips markup, scripts and styles from an HTML page: the title, then the body text one block per line.
func readableText(page string) string {
	var out strings.Builder
	var title strings.Builder
	z := html.NewTokenizer(strings.NewReader(page))
	skip, inTitle := 0, false
	for {
		switch z.Next() {
		case html.ErrorToken:
			text := tidyLines(out.String())
			if t := strings.Join(strings.Fields(title.String()), " "); t != "" {
				return t + "\n\n" + text
			}
			return text
		case html.TextToken:
			switch {
			case skip > 0:
			case inTitle:
				title.Write(z.Text())
			default:
				out.WriteString(flatten(string(z.Text())))
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			switch {
			case tag == "title":
				inTitle = true
			case skipTags[tag]:
				skip++
			case tag == "li":
				out.WriteString("\n- ")
			case blockTags[tag]:
				out.WriteString("\n")
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			switch {
			case tag == "title":
				inTitle = false
			case skipTags[tag]:
				skip = max(skip-1, 0)
			case blockTags[tag] && tag != "li":
				out.WriteString("\n")
			}
		}
	}
}

// flatten turns every whitespace run, newlines included, into one space: line breaks come from block tags only.
func flatten(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return " "
	}
	text := strings.Join(words, " ")
	if r, _ := utf8.DecodeRuneInString(s); unicode.IsSpace(r) {
		text = " " + text
	}
	if r, _ := utf8.DecodeLastRuneInString(s); unicode.IsSpace(r) {
		text += " "
	}
	return text
}

// tidyLines collapses runs of whitespace inside lines and drops repeated blank lines.
func tidyLines(s string) string {
	var out []string
	blank := true
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" || line == "-" {
			if !blank {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// capText cuts s to at most n bytes on a character boundary and says so.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "\n[truncated]"
}

func fetchTool() Tool {
	return Tool{
		Spec: llmSpec("fetch_url", "Download a web page (http or https) and return its readable text, truncated to about 20 KB.",
			map[string]any{"url": map[string]any{"type": "string", "description": "the full URL"}}, "url"),
		Run: func(ctx context.Context, args json.RawMessage) (string, error) {
			var a struct{ URL string }
			if err := json.Unmarshal(args, &a); err != nil {
				return "", fmt.Errorf("bad arguments: %w", err)
			}
			return FetchURL(ctx, a.URL)
		},
	}
}
