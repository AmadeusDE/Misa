package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
)

// Textual edits of ugh's config.toml. Users keep that file by hand, so these
// helpers touch only the tables they own and leave every other line, comment
// and blank as it was.

var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func tomlKey(k string) string {
	if bareKey.MatchString(k) {
		return k
	}
	return tomlString(k)
}

func tomlStrings(l []string) string {
	q := make([]string, len(l))
	for i, s := range l {
		q[i] = tomlString(s)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func tomlBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// renderProvider is the [providers.<name>] table, plus its headers table.
func renderProvider(name string, p config.Provider) string {
	var b strings.Builder
	b.WriteString("[providers." + tomlKey(name) + "]\n")
	b.WriteString("base_url = " + tomlString(p.BaseURL) + "\n")
	if p.KeyEnv != "" {
		b.WriteString("key_env = " + tomlString(p.KeyEnv) + "\n")
	}
	if p.SessionHeader {
		b.WriteString("session_header = true\n")
	}
	if len(p.NoThinking) > 0 {
		b.WriteString("no_thinking = " + tomlStrings(p.NoThinking) + "\n")
	}
	if p.Usage != "" {
		b.WriteString("usage = " + tomlString(p.Usage) + "\n")
	}
	if len(p.Headers) > 0 {
		b.WriteString("\n[providers." + tomlKey(name) + ".headers]\n")
		keys := make([]string, 0, len(p.Headers))
		for k := range p.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(tomlKey(k) + " = " + tomlString(p.Headers[k]) + "\n")
		}
	}
	return b.String()
}

// parseHeader returns the key path of a `[a.b]` or `[[a.b]]` line.
func parseHeader(line string) ([]string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "[") {
		return nil, false
	}
	array := strings.HasPrefix(s, "[[")
	s = strings.TrimPrefix(s, "[")
	if array {
		s = strings.TrimPrefix(s, "[")
	}
	var parts []string
	for {
		s = strings.TrimLeft(s, " \t")
		var part string
		switch {
		case s == "":
			return nil, false
		case s[0] == '"' || s[0] == '\'':
			end := closingQuote(s)
			if end < 0 {
				return nil, false
			}
			part = unquote(s[:end+1])
			s = s[end+1:]
		default:
			end := strings.IndexAny(s, ".] \t")
			if end <= 0 || !bareKey.MatchString(s[:end]) {
				return nil, false
			}
			part, s = s[:end], s[end:]
		}
		parts = append(parts, part)
		s = strings.TrimLeft(s, " \t")
		switch {
		case strings.HasPrefix(s, "."):
			s = s[1:]
		case strings.HasPrefix(s, "]"):
			s = strings.TrimPrefix(s[1:], "]")
			s = strings.TrimSpace(s)
			if s == "" || strings.HasPrefix(s, "#") {
				return parts, true
			}
			return nil, false
		default:
			return nil, false
		}
	}
}

func closingQuote(s string) int {
	q := s[0]
	for i := 1; i < len(s); i++ {
		if q == '"' && s[i] == '\\' {
			i++
			continue
		}
		if s[i] == q {
			return i
		}
	}
	return -1
}

func unquote(s string) string {
	inner := s[1 : len(s)-1]
	if s[0] == '\'' {
		return inner
	}
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(inner)
}

type tomlDoc struct {
	lines   []string
	headers []int      // line index of each table header
	paths   [][]string // key path of each header
}

func parseTomlDoc(src string) tomlDoc {
	d := tomlDoc{lines: strings.Split(strings.TrimSuffix(src, "\n"), "\n")}
	if src == "" {
		d.lines = nil
	}
	for i, l := range d.lines {
		if p, ok := parseHeader(l); ok {
			d.headers = append(d.headers, i)
			d.paths = append(d.paths, p)
		}
	}
	return d
}

func isComment(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "#") }

// attachedStart is the first line of header k including the comment lines
// directly above it, which belong to that table.
func (d tomlDoc) attachedStart(k int) int {
	floor := 0
	if k > 0 {
		floor = d.headers[k-1] + 1
	}
	i := d.headers[k]
	for i > floor && isComment(d.lines[i-1]) {
		i--
	}
	return i
}

func (d tomlDoc) regionEnd(k int) int {
	if k+1 < len(d.headers) {
		return d.attachedStart(k + 1)
	}
	return len(d.lines)
}

func hasPrefix(path, prefix []string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

func equalPath(a, b []string) bool { return len(a) == len(b) && hasPrefix(a, b) }

func (d tomlDoc) String() string {
	end := len(d.lines)
	for end > 0 && strings.TrimSpace(d.lines[end-1]) == "" {
		end--
	}
	if end == 0 {
		return ""
	}
	return strings.Join(d.lines[:end], "\n") + "\n"
}

// replaceTables swaps the tables whose path starts with prefix for text, in the
// place of the first one (appending when there is none). Empty text removes them.
func replaceTables(src string, prefix []string, text string) string {
	d := parseTomlDoc(src)
	var out []string
	cursor, placed := 0, false
	for k := range d.headers {
		if !hasPrefix(d.paths[k], prefix) {
			continue
		}
		start, end := d.attachedStart(k), d.regionEnd(k)
		if cursor < start {
			out = append(out, d.lines[cursor:start]...)
		}
		cursor = end
		if placed || text == "" {
			continue
		}
		placed = true
		out = append(out, d.lines[start:d.headers[k]]...)
		out = append(out, strings.Split(strings.TrimSuffix(text, "\n"), "\n")...)
		if end < len(d.lines) {
			out = append(out, "")
		}
	}
	out = append(out, d.lines[min(cursor, len(d.lines)):]...)
	if !placed && text != "" {
		out = trimTrailingBlank(out)
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, strings.Split(strings.TrimSuffix(text, "\n"), "\n")...)
	}
	return tomlDoc{lines: out}.String()
}

func trimTrailingBlank(l []string) []string {
	for len(l) > 0 && strings.TrimSpace(l[len(l)-1]) == "" {
		l = l[:len(l)-1]
	}
	return l
}

// setKey sets table.key to a TOML value. An empty value removes the key. A
// missing table is appended.
func setKey(src string, table []string, key, value string) string {
	d := parseTomlDoc(src)
	line := key + " = " + value
	for k := range d.headers {
		if !equalPath(d.paths[k], table) {
			continue
		}
		keyLine := regexp.MustCompile(`^(\s*)` + regexp.QuoteMeta(key) + `\s*=`)
		for i := d.headers[k] + 1; i < d.regionEnd(k); i++ {
			m := keyLine.FindStringSubmatch(d.lines[i])
			if m == nil {
				continue
			}
			if value == "" {
				d.lines = append(d.lines[:i], d.lines[i+1:]...)
			} else {
				d.lines[i] = m[1] + line
			}
			return d.String()
		}
		if value != "" {
			at := d.headers[k] + 1
			d.lines = append(d.lines[:at], append([]string{line}, d.lines[at:]...)...)
		}
		return d.String()
	}
	if value == "" {
		return src
	}
	names := make([]string, len(table))
	for i, t := range table {
		names[i] = tomlKey(t)
	}
	d.lines = trimTrailingBlank(d.lines)
	if len(d.lines) > 0 {
		d.lines = append(d.lines, "")
	}
	d.lines = append(d.lines, "["+strings.Join(names, ".")+"]", line)
	return d.String()
}
