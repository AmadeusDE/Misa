package ai

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReadableText(t *testing.T) {
	page := `<html><head><title> My  page </title><style>p{color:red}</style><script>var x = "<b>";</script></head>
<body><nav>Menu</nav><h1>Heading</h1><p>First   paragraph
with a <a href="/x">link</a>.</p><ul><li>one</li><li>two</li></ul>
<noscript>enable js</noscript><svg><text>icon</text></svg><p>Last &amp; end</p></body></html>`
	want := "My page\n\nMenu\n\nHeading\n\nFirst paragraph with a link.\n\n- one\n- two\n\nLast & end"
	if got := readableText(page); got != want {
		t.Errorf("readableText =\n%q\nwant\n%q", got, want)
	}
}

func TestFetchURL(t *testing.T) {
	srv := useWebServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<p>hi</p>"))
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("plain body"))
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"a":1}`))
		case "/png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG"))
		case "/big":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(strings.Repeat("é", fetchTextCap)))
		case "/slow":
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	for path, want := range map[string]string{"/html": "hi", "/text": "plain body", "/json": `{"a":1}`} {
		if got, err := FetchURL(ctx, srv.URL+path); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	big, err := FetchURL(ctx, srv.URL+"/big")
	if err != nil || len(big) > fetchTextCap+20 || !strings.HasSuffix(big, "[truncated]") {
		t.Errorf("big: len %d, err %v, tail %q", len(big), err, big[max(len(big)-15, 0):])
	}
	if strings.ContainsRune(strings.TrimSuffix(big, "\n[truncated]"), '�') {
		t.Error("cut inside a character")
	}
	for path, want := range map[string]string{"/png": "unsupported content type", "/missing": "HTTP 404"} {
		if _, err := FetchURL(ctx, srv.URL+path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", path, err, want)
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := FetchURL(cctx, srv.URL+"/slow"); err == nil {
		t.Error("slow page did not time out")
	}
}

func TestFetchURLOnlyHTTP(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.org/x", "javascript:alert(1)", "example.org", "", "http://"} {
		if _, err := FetchURL(context.Background(), raw); err == nil || !strings.Contains(err.Error(), "http or https") {
			t.Errorf("%q: err = %v", raw, err)
		}
	}
}
