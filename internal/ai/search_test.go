package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const ddgFixture = `<html><body>
<div class="result results_links result--ad"><h2 class="result__title"><a class="result__a" href="https://duckduckgo.com/y.js?ad_provider=x">Buy things</a></h2></div>
<div class="result results_links web-result">
 <h2 class="result__title"><a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2Fdoc%2F&amp;rut=abc">The Go <b>Programming</b> Language</a></h2>
 <a class="result__snippet" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2Fdoc%2F">Go is an open source <b>programming</b> language.</a>
</div>
<div class="result results_links web-result">
 <h2 class="result__title"><a class="result__a" href="https://example.org/direct">Direct link</a></h2>
 <div class="result__snippet">Plain   snippet
 text</div>
</div>
</body></html>`

func TestParseDuckDuckGo(t *testing.T) {
	got := parseDuckDuckGo(ddgFixture)
	want := []SearchResult{
		{"The Go Programming Language", "https://go.dev/doc/", "Go is an open source programming language."},
		{"Direct link", "https://example.org/direct", "Plain snippet text"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d results %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("result %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func useWebServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	old, oldClient := duckduckgoURL, webClient
	webClient = srv.Client()
	t.Cleanup(func() { duckduckgoURL, webClient = old, oldClient })
	return srv
}

func TestWebSearchDuckDuckGo(t *testing.T) {
	var query, agent string
	srv := useWebServer(t, func(w http.ResponseWriter, r *http.Request) {
		query, agent = r.URL.Query().Get("q"), r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(ddgFixture))
	})
	duckduckgoURL = srv.URL + "/html/"
	res, err := WebSearch(context.Background(), Search{Engine: "duckduckgo"}, "go language")
	if err != nil || len(res) != 2 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if query != "go language" || agent == "" {
		t.Errorf("query %q, agent %q", query, agent)
	}
}

func TestWebSearchSearxng(t *testing.T) {
	var raw string
	srv := useWebServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RequestURI()
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{
			{"title": "A", "url": "https://a.example", "content": "about a"},
			{"title": "B", "url": "https://b.example"},
		}})
	})
	for _, base := range []string{srv.URL, srv.URL + "/", srv.URL + "/search"} {
		res, err := WebSearch(context.Background(), Search{Engine: "searxng", SearxngURL: base}, "a b")
		if err != nil || len(res) != 2 || res[0].Snippet != "about a" || res[1].URL != "https://b.example" {
			t.Fatalf("base %q: res = %+v, err = %v", base, res, err)
		}
		if raw != "/search?format=json&q=a+b" {
			t.Errorf("request %q", raw)
		}
	}
}

func TestWebSearchErrors(t *testing.T) {
	srv := useWebServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/bad") {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}
		w.Write([]byte("<html>not json</html>"))
	})
	ctx := context.Background()
	for name, tc := range map[string]struct {
		cfg   Search
		query string
		want  string
	}{
		"empty query":   {Search{Engine: "duckduckgo"}, " ", "needs a query"},
		"off":           {Search{Engine: "off"}, "x", "off"},
		"no searxng":    {Search{Engine: "searxng"}, "x", "searxng_url"},
		"http status":   {Search{Engine: "searxng", SearxngURL: srv.URL + "/bad"}, "x", "HTTP 403"},
		"html not json": {Search{Engine: "searxng", SearxngURL: srv.URL}, "x", "JSON"},
	} {
		if _, err := WebSearch(ctx, tc.cfg, tc.query); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

func TestWebSearchCapsResults(t *testing.T) {
	srv := useWebServer(t, func(w http.ResponseWriter, r *http.Request) {
		var rs []map[string]string
		for i := 0; i < 20; i++ {
			rs = append(rs, map[string]string{"title": "t", "url": "https://x.example"})
		}
		json.NewEncoder(w).Encode(map[string]any{"results": rs})
	})
	res, _ := WebSearch(context.Background(), Search{Engine: "searxng", SearxngURL: srv.URL}, "q")
	if len(res) != maxSearchResults {
		t.Errorf("got %d results, want %d", len(res), maxSearchResults)
	}
}

func TestWebToolsRunThroughRunTools(t *testing.T) {
	srv := useWebServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("<title>T</title><p>hello page</p>"))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{{"title": "A", "url": "https://a.example", "content": "c"}}})
	})
	if WebTools(Search{Engine: "off"}) != nil {
		t.Fatal("tools offered while search is off")
	}
	tools := WebTools(Search{Engine: "searxng", SearxngURL: srv.URL})
	if len(tools) != 2 || tools[0].Spec.Name != "web_search" || tools[1].Spec.Name != "fetch_url" {
		t.Fatalf("tools = %+v", tools)
	}
	out, err := tools[0].Run(context.Background(), json.RawMessage(`{"query":"q"}`))
	if err != nil || !strings.Contains(out, "1. A") || !strings.Contains(out, "https://a.example") {
		t.Errorf("search out = %q, err = %v", out, err)
	}
	out, err = tools[1].Run(context.Background(), json.RawMessage(`{"url":"`+srv.URL+`/page"}`))
	if err != nil || out != "T\n\nhello page" {
		t.Errorf("fetch out = %q, err = %v", out, err)
	}
	if _, err := tools[0].Run(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Error("bad arguments accepted")
	}
}
