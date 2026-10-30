package aihelp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func wikiServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		q := r.URL.Query()
		switch {
		case q.Get("action") == "query" && q.Get("srsearch") == "nothing":
			w.Write([]byte(`{"query":{"search":[]}}`))
		case q.Get("action") == "query":
			w.Write([]byte(`{"query":{"search":[{"title":"Systemd"},{"title":"Systemd/Timers"}]}}`))
		case q.Get("action") == "parse" && q.Get("page") == "Systemd/Timers":
			w.Write([]byte(`{"parse":{"title":"Systemd/Timers","wikitext":{"*":"==Units==\nUse {{ic|systemctl list-timers}} and [[Cron|cron]].\n{{Note|Timers are '''great'''.}}"}}}`))
		case q.Get("action") == "parse" && q.Get("page") == "Systemd":
			w.Write([]byte(`{"parse":{"title":"Systemd","wikitext":{"*":"init system"}}}`))
		case q.Get("action") == "parse":
			w.Write([]byte(`{"error":{"code":"missingtitle"}}`))
		default:
			http.Error(w, "bad", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

func TestWikiSearchAndPage(t *testing.T) {
	srv, queries := wikiServer(t)
	w := Wiki{BaseURL: srv.URL}
	titles, err := w.Search(context.Background(), "systemd timer & more")
	if err != nil || strings.Join(titles, ",") != "Systemd,Systemd/Timers" {
		t.Fatalf("Search = %v, %v", titles, err)
	}
	if !strings.Contains((*queries)[0], "srsearch=systemd+timer+%26+more") {
		t.Errorf("query not escaped: %s", (*queries)[0])
	}
	page, err := w.Page(context.Background(), "Systemd/Timers")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Arch Wiki: Systemd/Timers", "## Units", "`systemctl list-timers`", "and cron.", "> **NOTE**: Timers are **great**."} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	if _, err := w.Page(context.Background(), "Nope"); err == nil {
		t.Error("missing page gave no error")
	}
}

func TestWikiStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 503) }))
	defer srv.Close()
	if _, err := (Wiki{BaseURL: srv.URL}).Search(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v", err)
	}
}

func TestCleanWikitext(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"'''b''' ''i''", "**b** *i*"},
		{"===Sub===\n====Deeper====", "### Sub\n#### Deeper"},
		{"[[A|shown]] [[B]] [https://x.org site]", "shown B site"},
		{"{{hc|$ ls|file}}", "```\n$ ls\nfile\n```"},
		{"{{Translateme|{{ic|x}}}}", ""},
		{"a<!-- hidden -->b <b>c</b> &lt;d&gt; &amp;\n\n\n\ne", "ab c <d> &\n\ne"},
		{"{{unclosed", "{{unclosed"},
	} {
		if got := cleanWikitext(c.in); got != c.want {
			t.Errorf("cleanWikitext(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
