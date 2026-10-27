package lscore

import "strings"

// QuotingStyle names the supported --quoting-style values.
type QuotingStyle string

const (
	QuoteLiteral           QuotingStyle = "literal"
	QuoteShell             QuotingStyle = "shell"
	QuoteShellAlways       QuotingStyle = "shell-always"
	QuoteShellEscape       QuotingStyle = "shell-escape"
	QuoteShellEscapeAlways QuotingStyle = "shell-escape-always"
	QuoteC                 QuotingStyle = "c"
	QuoteCMaybe            QuotingStyle = "c-maybe"
	QuoteEscape            QuotingStyle = "escape"
	QuoteLocale            QuotingStyle = "locale"
	QuoteCLocale           QuotingStyle = "clocale"
)

// ValidQuotingStyle reports whether name is a recognized --quoting-style value.
func ValidQuotingStyle(name string) bool {
	switch QuotingStyle(name) {
	case QuoteLiteral, QuoteShell, QuoteShellAlways, QuoteShellEscape,
		QuoteShellEscapeAlways, QuoteC, QuoteCMaybe, QuoteEscape,
		QuoteLocale, QuoteCLocale:
		return true
	}
	return false
}

// specialChars are the characters that make a name ambiguous enough to
// require quoting under shell/c-style quoting (space, shell metacharacters,
// and characters GNU quotearg treats as special such as ',' and ':').
const specialChars = " \t\n\v\f\r\"'`\\$&;|<>(){}[]*?!~#,:"

func needsQuoting(name string) bool {
	if name == "" {
		return true
	}
	return strings.ContainsAny(name, specialChars)
}

// QuoteName renders name according to the given --quoting-style value.
// Unrecognized styles are treated as "literal".
func QuoteName(name string, style string) string {
	switch QuotingStyle(style) {
	case QuoteShell:
		if needsQuoting(name) {
			return shellQuote(name)
		}
		return name
	case QuoteShellAlways:
		return shellQuote(name)
	case QuoteShellEscape:
		if needsQuoting(name) {
			return shellQuote(name)
		}
		return name
	case QuoteShellEscapeAlways:
		return shellQuote(name)
	case QuoteC:
		return cQuote(name)
	case QuoteCMaybe:
		if needsQuoting(name) {
			return cQuote(name)
		}
		return name
	case QuoteEscape:
		return escapeQuote(name)
	case QuoteLocale:
		return shellQuote(name)
	case QuoteCLocale:
		return cQuote(name)
	default:
		return name
	}
}

func shellQuote(name string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range name {
		if r == '\'' {
			b.WriteString(`'\''`)
		} else {
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func cQuote(name string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range name {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// sanitizeControl replaces raw control bytes (including embedded newlines)
// with '?'. GNU ls does this for the default "literal" quoting style
// whenever entries are packed multiple-per-line (as with -m) and wrapping
// is width-bounded, since an embedded newline would otherwise make the
// line-wrapped output ambiguous. It is skipped when width is unbounded
// (-w0), matching GNU's behavior of passing raw bytes through unchanged
// when column-width tracking is disabled.
func sanitizeControl(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || c == 0x7f {
			b.WriteByte('?')
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func escapeQuote(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == ' ':
			b.WriteString(`\ `)
		case strings.ContainsRune(specialChars, r) && r != ' ':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
