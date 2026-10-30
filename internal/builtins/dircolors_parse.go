// Database parsing for the dircolors builtin.
//
// Adapted from uutils coreutils (MIT license):
//   src/uu/dircolors/src/dircolors.rs (purify, split_two, fnmatch, escape,
//   parse, append_entry)
// pinned at commit fee61ff5d4f87e0f912a52e639942b5f7ad58663.
// See docs/CREDITS.md and THIRD_PARTY_NOTICES.md.

package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// readDatabaseLines collects the lines of the colour database: from stdin
// for "-", otherwise from path. The error text mirrors the reference's
// "path: message" diagnostics.
func readDatabaseLines(ctx *Context, name, path string) ([]string, error) {
	var r io.Reader
	if name == "-" {
		if ctx.Stdin == nil {
			return nil, fmt.Errorf("%s: no such file or directory", name)
		}
		r = ctx.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", name, fmtOsErr(err))
		}
		defer f.Close()
		r = f
	}

	var lines []string
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			lines = append(lines, line)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return lines, nil
}

// dircolorsPurify drops a trailing comment and surrounding whitespace. A
// '#' starts a comment only at the start of a line or after whitespace.
func dircolorsPurify(line string) string {
	for n := 0; n < len(line); n++ {
		if line[n] != '#' {
			continue
		}
		if n == 0 {
			return ""
		}
		if c := line[n-1]; c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' {
			return strings.TrimSpace(line[:n-1])
		}
	}
	return strings.TrimSpace(line)
}

// dircolorsSplitTwo splits a line into its keyword and the rest of the line.
// A line with no whitespace at all yields an empty value.
func dircolorsSplitTwo(line string) (string, string) {
	isSpace := func(c byte) bool {
		return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
	}
	b := -1
	for i := 0; i < len(line); i++ {
		if isSpace(line[i]) {
			b = i
			break
		}
	}
	if b < 0 {
		return "", ""
	}
	e := -1
	for i := b; i < len(line); i++ {
		if !isSpace(line[i]) {
			e = i
			break
		}
	}
	if e < 0 {
		return line[:b], ""
	}
	return line[:b], line[e:]
}

// dircolorsEscape makes a value safe between single quotes and escapes ':'
// so a colour value cannot look like an entry separator.
func dircolorsEscape(s string) string {
	var b strings.Builder
	prev := byte(' ')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'':
			b.WriteString(`'\''`)
		case c == ':' && prev != '\\':
			b.WriteString(`\:`)
		default:
			b.WriteByte(c)
		}
		prev = c
	}
	return b.String()
}

// dircolorsParse turns database lines into the shell/display commands for
// format f. It returns the output and, on failure, the diagnostic text.
func dircolorsParse(lines []string, f dircolorsFmt, file string, ctx *Context) (string, string) {
	prefix, suffix := dircolorsColors(f)

	term, termFound := lookupEnv(ctx, "TERM")
	if !termFound {
		term = "none"
	}
	colorterm, _ := lookupEnv(ctx, "COLORTERM")

	// Parse states, as in the reference: entries before the first matching
	// TERM/COLORTERM line are global, entries after a non-matching one are
	// skipped until a match, and a match keeps later TERM lines from
	// cancelling the rest of the file.
	const (
		stGlobal = iota
		stMatched
		stContinue
		stPass
	)
	state := stGlobal
	sawColortermMatch := false

	var out strings.Builder
	out.WriteString(prefix)

	for i, raw := range lines {
		line := dircolorsPurify(raw)
		if line == "" {
			continue
		}
		line = dircolorsEscape(line)

		key, val := dircolorsSplitTwo(line)
		if val == "" {
			return "", fmt.Sprintf("%s:%d: invalid line;  missing second token", file, i+1)
		}

		lower := strings.ToLower(key)
		switch lower {
		case "term":
			if dircolorsGlobMatch(val, term) {
				state = stMatched
			} else if state == stGlobal {
				state = stPass
			}
		case "colorterm":
			matched := false
			if val == "?*" {
				matched = colorterm != ""
			} else {
				matched = dircolorsGlobMatch(val, colorterm)
			}
			if matched {
				state = stMatched
				sawColortermMatch = true
			} else if !sawColortermMatch && state == stGlobal {
				state = stPass
			}
		default:
			if state == stMatched {
				state = stContinue
			}
			if state == stPass {
				continue
			}
			entry, msg := dircolorsEntry(f, key, lower, val)
			if msg != "" {
				return "", msg
			}
			out.WriteString(entry)
		}
	}

	if f == dcDisplay {
		s := out.String()
		if strings.HasSuffix(s, "\n") {
			s = s[:len(s)-1]
		}
		return s + suffix, ""
	}
	return out.String() + suffix, ""
}

// dircolorsEntry renders one database entry (extension pattern or file type).
func dircolorsEntry(f dircolorsFmt, key, lower, val string) (string, string) {
	if strings.HasPrefix(key, ".") || strings.HasPrefix(key, "*") {
		pattern := key
		if strings.HasPrefix(pattern, ".") {
			pattern = "*" + pattern
		}
		if f == dcDisplay {
			return fmt.Sprintf("\x1b[%sm%s\t%s\x1b[0m\n", val, pattern, val), ""
		}
		return pattern + "=" + val + ":", ""
	}

	switch lower {
	case "options", "color", "eightbit":
		// Honoured by the slackware dircolors only; recognized but ignored.
		return "", ""
	}
	code, ok := dircolorsAttrCodes[lower]
	if !ok {
		return "", "unrecognized keyword " + key
	}
	if f == dcDisplay {
		return fmt.Sprintf("\x1b[%sm%s\t%s\x1b[0m\n", val, code, val), ""
	}
	return code + "=" + val + ":", ""
}

// dircolorsGlobMatch matches name against a TERM/COLORTERM glob pattern.
// It supports '*', '?', and '[...]' classes with '!' or '^' negation and
// ranges; an invalid pattern never matches, as in the reference.
func dircolorsGlobMatch(pattern, name string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if dircolorsGlobMatch(pattern, name[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(name) == 0 {
				return false
			}
			pattern, name = pattern[1:], name[1:]
		case '[':
			matched, rest, ok := dircolorsGlobClass(pattern)
			if !ok || len(name) == 0 {
				return false
			}
			if !matched(name[0]) {
				return false
			}
			pattern, name = rest, name[1:]
		default:
			if len(name) == 0 || name[0] != pattern[0] {
				return false
			}
			pattern, name = pattern[1:], name[1:]
		}
	}
	return len(name) == 0
}

// dircolorsGlobClass parses a leading "[...]" class and reports a matcher
// plus the pattern remainder.
func dircolorsGlobClass(pattern string) (func(byte) bool, string, bool) {
	i := 1
	negated := false
	if i < len(pattern) && (pattern[i] == '!' || pattern[i] == '^') {
		negated = true
		i++
	}
	var set []byte // pairs of (lo, hi)
	first := true
	for i < len(pattern) {
		c := pattern[i]
		if c == ']' && !first {
			i++
			match := func(b byte) bool {
				found := false
				for k := 0; k+1 < len(set); k += 2 {
					if b >= set[k] && b <= set[k+1] {
						found = true
						break
					}
				}
				if negated {
					return !found
				}
				return found
			}
			return match, pattern[i:], true
		}
		first = false
		lo, hi := c, c
		if i+2 < len(pattern) && pattern[i+1] == '-' && pattern[i+2] != ']' {
			hi = pattern[i+2]
			i += 3
		} else {
			i++
		}
		if hi < lo {
			lo, hi = hi, lo
		}
		set = append(set, lo, hi)
	}
	return nil, "", false
}
