package builtins

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dlclark/regexp2"
)

func diffOptionValue(arg string) bool {
	if strings.HasPrefix(arg, "--") {
		if strings.Contains(arg, "=") {
			return false
		}
		def, ok := diffSpec.byLong[strings.ToLower(arg[2:])]
		if !ok {
			def, ok = diffSpec.matchLongPrefix(strings.ToLower(arg[2:]))
		}
		return ok && (def.kind == kindString || def.kind == kindNum)
	}
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	for i := 1; i < len(arg); i++ {
		def, ok := diffSpec.byShort[arg[i]]
		if ok && (def.kind == kindString || def.kind == kindNum) {
			return i == len(arg)-1
		}
	}
	return false
}

func diffCommandFlags(args []string) string {
	var options []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			options = append(options, arg)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}
		options = append(options, diffQuote(arg))
		if diffOptionValue(arg) && i+1 < len(args) {
			i++
			options = append(options, diffQuote(args[i]))
		}
	}
	if len(options) == 0 {
		return ""
	}
	return strings.Join(options, " ") + " "
}

func diffQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`;&|()<>*?[]=") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func diffNameQuote(s string) string {
	if strings.ContainsAny(s, " \t\n\r\v\f\"") {
		return strconv.Quote(s)
	}
	return s
}

type diffPattern struct {
	linear       *regexp.Regexp
	backtracking *regexp2.Regexp
}

func (p diffPattern) MatchString(text string) bool {
	if p.linear != nil {
		return p.linear.MatchString(text)
	}
	matched, _ := p.backtracking.MatchString(text)
	return matched
}

func diffRegexp(pattern string) (diffPattern, error) {
	var out strings.Builder
	bracket := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && i+1 < len(pattern) {
			i++
			c = pattern[i]
			if !bracket && (c == '<' || c == '>') {
				if c == '<' {
					out.WriteString(`(?<!\w)(?=\w)`)
				} else {
					out.WriteString(`(?<=\w)(?!\w)`)
				}
			} else if !bracket && strings.ContainsRune("()+?{}|", rune(c)) {
				out.WriteByte(c)
			} else {
				out.WriteByte('\\')
				out.WriteByte(c)
			}
			continue
		}
		if c == '[' {
			bracket = true
		} else if c == ']' {
			bracket = false
		}
		if !bracket && strings.ContainsRune("()+?{}|", rune(c)) {
			out.WriteByte('\\')
		}
		out.WriteByte(c)
	}
	re, err := regexp.Compile(out.String())
	if err == nil {
		return diffPattern{linear: re}, nil
	}
	fallback, err := regexp2.Compile(out.String(), regexp2.RE2)
	if err != nil {
		return diffPattern{}, fmt.Errorf("invalid regular expression: %s", pattern)
	}
	fallback.MatchTimeout = time.Second
	return diffPattern{backtracking: fallback}, nil
}

func diffIgnored(c diffChange, a, b []diffLine, o diffOptions) bool {
	if !o.ignoreBlank && len(o.ignoreMatching) == 0 {
		return false
	}
	for _, lines := range [][]diffLine{a[c.a0:c.a1], b[c.b0:c.b1]} {
		for _, line := range lines {
			blank := line.text == ""
			if o.ignoreSpace || o.ignoreAllSpace || o.ignoreTrailing {
				blank = strings.TrimSpace(line.text) == ""
			}
			if o.ignoreBlank && blank {
				continue
			}
			matched := false
			for _, re := range o.ignoreMatching {
				if re.MatchString(line.text) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
	}
	return true
}

func diffVisibleChanges(a, b []diffLine, edits []diffEdit, o diffOptions) []diffChange {
	var visible []diffChange
	for _, c := range diffChanges(edits) {
		if !diffIgnored(c, a, b, o) {
			visible = append(visible, c)
		}
	}
	return visible
}

func diffVisibleHunks(a, b []diffLine, edits []diffEdit, o diffOptions) [][2]int {
	var visible [][2]int
	for _, h := range diffHunks(edits, o.context) {
		if len(diffVisibleChanges(a, b, edits[h[0]:h[1]], o)) > 0 {
			visible = append(visible, h)
		}
	}
	return visible
}

func diffFunction(a []diffLine, before int, o diffOptions) string {
	for i := before - 1; i >= 0; i-- {
		for _, re := range o.functions {
			if re.MatchString(a[i].text) {
				text := strings.TrimRightFunc(a[i].text, unicode.IsSpace)
				if len(text) > 40 {
					text = text[:40]
				}
				return " " + text
			}
		}
	}
	return ""
}

func diffRCS(a, b []diffLine, edits []diffEdit, o diffOptions) string {
	var out strings.Builder
	for _, c := range diffVisibleChanges(a, b, edits, o) {
		if c.a1 > c.a0 {
			fmt.Fprintf(&out, "d%d %d\n", c.a0+1, c.a1-c.a0)
		}
		if c.b1 > c.b0 {
			fmt.Fprintf(&out, "a%d %d\n", c.a1, c.b1-c.b0)
			for _, line := range b[c.b0:c.b1] {
				text := line.text
				if o.expandTabs {
					text = diffExpand(text, o.tabsize)
				}
				out.WriteString(text)
				if line.newline {
					out.WriteByte('\n')
				}
			}
		}
	}
	return out.String()
}
