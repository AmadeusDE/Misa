package ai

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
)

var acme = config.Provider{BaseURL: "https://api.acme.test/v1", KeyEnv: "ACME_KEY", NoThinking: []string{"glm-", "x"},
	Headers: map[string]string{"X-B": "2", "a": `say "hi"`}}

const acmeText = `[providers.acme]
base_url = "https://api.acme.test/v1"
key_env = "ACME_KEY"
no_thinking = ["glm-", "x"]

[providers.acme.headers]
X-B = "2"
a = "say \"hi\""
`

func TestRenderProvider(t *testing.T) {
	if got := renderProvider("acme", acme); got != acmeText {
		t.Fatalf("got:\n%s", got)
	}
	got := renderProvider("my.name", config.Provider{BaseURL: "http://localhost:1/v1", SessionHeader: true, Usage: "none"})
	want := "[providers.\"my.name\"]\nbase_url = \"http://localhost:1/v1\"\nsession_header = true\nusage = \"none\"\n"
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
}

func TestTomlString(t *testing.T) {
	for in, want := range map[string]string{
		"plain": `"plain"`, `a"b\c`: `"a\"b\\c"`, "l1\nl2\t": `"l1\nl2\t"`, "\x01": `"\u0001"`, "é": `"é"`,
	} {
		if got := tomlString(in); got != want {
			t.Errorf("tomlString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestParseHeader(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
		ok   bool
	}{
		{"[providers.acme]", []string{"providers", "acme"}, true},
		{"  [ providers . acme ]  # note", []string{"providers", "acme"}, true},
		{`[providers."my.name".headers]`, []string{"providers", "my.name", "headers"}, true},
		{"[[array.table]]", []string{"array", "table"}, true},
		{"key = [1, 2]", nil, false},
		{"[unterminated", nil, false},
		{"[a] trailing", nil, false},
		{"# [a]", nil, false},
	} {
		got, ok := parseHeader(tc.line)
		if ok != tc.ok || strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("parseHeader(%q) = %v %v, want %v %v", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

func TestReplaceTables(t *testing.T) {
	prefix := []string{"providers", "acme"}
	for _, tc := range []struct{ name, src, text, want string }{
		{"missing file", "", acmeText, acmeText},
		{"append keeps content", "# mine\n[ui]\ntheme = \"dark\"\n", acmeText,
			"# mine\n[ui]\ntheme = \"dark\"\n\n" + acmeText},
		{"append after no trailing newline", "[ui]\ntheme = \"dark\"", acmeText,
			"[ui]\ntheme = \"dark\"\n\n" + acmeText},
		{"replace in place keeps neighbours and comments",
			"[auth]\nstore = \"file\" # why\n\n# acme is for work\n[providers.acme]\nbase_url = \"https://old.test/v1\"\n\n[providers.acme.headers]\nX = \"y\"\n\n# other\n[providers.zed]\nbase_url = \"https://zed.test/v1\"\n",
			acmeText,
			"[auth]\nstore = \"file\" # why\n\n# acme is for work\n" + acmeText + "\n# other\n[providers.zed]\nbase_url = \"https://zed.test/v1\"\n"},
		{"replace last table",
			"[providers.acme]\nbase_url = \"https://old.test/v1\"\n", acmeText, acmeText},
		{"drops stale headers when new has none",
			"[providers.acme]\nbase_url = \"https://old.test/v1\"\n\n[providers.acme.headers]\nX = \"y\"\n\n[ui]\ntheme = \"dark\"\n",
			"[providers.acme]\nbase_url = \"https://new.test/v1\"\n",
			"[providers.acme]\nbase_url = \"https://new.test/v1\"\n\n[ui]\ntheme = \"dark\"\n"},
		{"other providers untouched",
			"[providers.acmeco]\nbase_url = \"https://x.test/v1\"\n", acmeText,
			"[providers.acmeco]\nbase_url = \"https://x.test/v1\"\n\n" + acmeText},
		{"remove with comment above",
			"[ui]\ntheme = \"dark\"\n\n# acme\n[providers.acme]\nbase_url = \"https://old.test/v1\"\n\n[models]\ndefault = \"acme/m\"\n", "",
			"[ui]\ntheme = \"dark\"\n\n[models]\ndefault = \"acme/m\"\n"},
		{"remove last leaves no trailing blanks",
			"[ui]\ntheme = \"dark\"\n\n[providers.acme]\nbase_url = \"https://old.test/v1\"\n", "",
			"[ui]\ntheme = \"dark\"\n"},
		{"remove absent is a no-op", "[ui]\ntheme = \"dark\"\n", "", "[ui]\ntheme = \"dark\"\n"},
		{"quoted header matches", "[providers.\"acme\"]\nbase_url = \"https://old.test/v1\"\n", acmeText, acmeText},
		{"crlf-free comment block below stays with the next table",
			"[providers.acme]\nbase_url = \"https://old.test/v1\"\n# about ui\n[ui]\ntheme = \"dark\"\n",
			"[providers.acme]\nbase_url = \"https://new.test/v1\"\n",
			"[providers.acme]\nbase_url = \"https://new.test/v1\"\n\n# about ui\n[ui]\ntheme = \"dark\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := replaceTables(tc.src, prefix, tc.text); got != tc.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestSetKey(t *testing.T) {
	models := []string{"models"}
	for _, tc := range []struct{ name, src, value, want string }{
		{"missing file", "", `"a/b"`, "[models]\ndefault = \"a/b\"\n"},
		{"appends table", "[ui]\ntheme = \"dark\"\n", `"a/b"`, "[ui]\ntheme = \"dark\"\n\n[models]\ndefault = \"a/b\"\n"},
		{"replaces key keeps indent and neighbours",
			"# top\n[models]\n  default = \"old/m\"\nother = 1\n\n[ui]\ndefault = \"not me\"\n", `"a/b"`,
			"# top\n[models]\n  default = \"a/b\"\nother = 1\n\n[ui]\ndefault = \"not me\"\n"},
		{"inserts into existing table", "[models]\n# pick one\nfast = \"x/y\"\n\n[ui]\n", `"a/b"`,
			"[models]\ndefault = \"a/b\"\n# pick one\nfast = \"x/y\"\n\n[ui]\n"},
		{"removes key", "[models]\ndefault = \"a/b\"\nfast = \"x/y\"\n", "", "[models]\nfast = \"x/y\"\n"},
		{"remove from absent table is a no-op", "[ui]\n", "", "[ui]\n"},
		{"key prefix is not a match", "[models]\ndefault_x = 1\n", `"a/b"`, "[models]\ndefault = \"a/b\"\ndefault_x = 1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := setKey(tc.src, models, "default", tc.value); got != tc.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}
