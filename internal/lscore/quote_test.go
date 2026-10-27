package lscore

import "testing"

func TestQuoteNameStyles(t *testing.T) {
	cases := []struct {
		name  string
		style string
		want  string
	}{
		{"com,ma", "literal", "com,ma"},
		{"com,ma", "shell", "'com,ma'"},
		{"com,ma", "shell-always", "'com,ma'"},
		{"com,ma", "shell-escape", "'com,ma'"},
		{"com,ma", "shell-escape-always", "'com,ma'"},
		{"com,ma", "c", `"com,ma"`},
		{"com,ma", "c-maybe", `"com,ma"`},
		{"com,ma", "escape", `com\,ma`},
		{"com,ma", "locale", "'com,ma'"},
		{"com,ma", "clocale", `"com,ma"`},
		{"plain", "shell", "plain"},
		{"plain", "c-maybe", "plain"},
		{"needs quoting", "shell-escape", "'needs quoting'"},
		{"it's", "shell-always", `'it'\''s'`},
	}
	for _, c := range cases {
		got := QuoteName(c.name, c.style)
		if got != c.want {
			t.Errorf("QuoteName(%q, %q) = %q, want %q", c.name, c.style, got, c.want)
		}
	}
}

func TestQuoteNameUnknownStyleIsLiteral(t *testing.T) {
	if got := QuoteName("a b", "bogus"); got != "a b" {
		t.Errorf("QuoteName with unknown style = %q, want literal passthrough", got)
	}
}

func TestSanitizeControl(t *testing.T) {
	got := sanitizeControl("n\nl")
	if got != "n?l" {
		t.Errorf("sanitizeControl(%q) = %q, want %q", "n\nl", got, "n?l")
	}
}
