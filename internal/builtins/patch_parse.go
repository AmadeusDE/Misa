// Behaviour modelled on toybox toys/posix/patch.c,
// landley/toybox@6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
// Native Go implementation; see docs/CREDITS.md and THIRD_PARTY_NOTICES.md.
package builtins

import (
	"fmt"
	"strings"
)

// patchLine is one line of a target file; nl records whether it ended with a
// newline (the last line of a file without one keeps nl == false).
type patchLine struct {
	text string
	nl   bool
}

// patchHunkLine is one body line of a hunk: ' ', '-' or '+'.
type patchHunkLine struct {
	kind byte
	text string
	noNL bool // followed by "\ No newline at end of file"
}

type patchHunk struct {
	oldStart, oldCount int
	newStart, newCount int
	lines              []patchHunkLine
	raw                []string // header + body lines verbatim, for the .rej file
}

type patchFile struct {
	oldName, newName     string // header names before -p stripping, /dev/null kept
	oldHeader, newHeader string
	hunks                []patchHunk
}

// parsePatch reads the whole patch stream. Files are applied only after this
// returns, so a malformed patch writes nothing.
func parsePatch(data []byte, opts *patchOptions, source string) ([]patchFile, error) {
	if source == "" || source == "-" {
		source = "<stdin>"
	}

	text := string(data)
	var raw []string
	if text != "" {
		raw = strings.Split(text, "\n")
		if strings.HasSuffix(text, "\n") {
			raw = raw[:len(raw)-1]
		}
	}

	var files []patchFile
	var cur *patchFile
	var hunk *patchHunk
	oldRemain, newRemain := 0, 0

	flushHunk := func() {
		if hunk != nil && cur != nil {
			cur.hunks = append(cur.hunks, *hunk)
		}
		hunk = nil
	}
	flushFile := func() {
		flushHunk()
		if cur != nil && len(cur.hunks) > 0 {
			files = append(files, *cur)
		}
		cur = nil
	}

	for idx := 0; idx < len(raw); idx++ {
		line := raw[idx]
		if strings.HasSuffix(line, "\r") {
			if !opts.dosNotice {
				opts.dosNotice = true
				if !opts.quiet {
					fmt.Fprintln(opts.stderr, "Removing DOS newlines")
				}
			}
			line = line[:len(line)-1]
		}
		lineNo := idx + 1

		if hunk != nil {
			switch {
			case line == `\ No newline at end of file`:
				if len(hunk.lines) == 0 {
					return nil, fmt.Errorf("%s:%d: malformed hunk line: %s", source, lineNo, line)
				}
				hunk.lines[len(hunk.lines)-1].noNL = true
				continue
			case strings.HasPrefix(line, `\`):
				return nil, fmt.Errorf("%s:%d: malformed hunk line: %s", source, lineNo, line)
			case oldRemain == 0 && newRemain == 0:
				// The hunk is complete; hand this line back to the outer state.
				flushHunk()
				idx--
				continue
			}

			if line == "" {
				line = " " // damaged patches drop the space of an empty context line
			}
			if line[0] != ' ' && line[0] != '-' && line[0] != '+' {
				return nil, patchTruncatedHunk(source, lineNo, oldRemain, newRemain)
			}
			hunk.lines = append(hunk.lines, patchHunkLine{kind: line[0], text: line[1:]})
			hunk.raw = append(hunk.raw, line)
			if line[0] != '+' {
				oldRemain--
			}
			if line[0] != '-' {
				newRemain--
			}
			if oldRemain < 0 || newRemain < 0 {
				return nil, patchTruncatedHunk(source, lineNo, oldRemain, newRemain)
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "--- "):
			flushFile()
			cur = &patchFile{oldHeader: line}
			name, err := patchHeaderName(line)
			if err != nil {
				return nil, err
			}
			cur.oldName = name
		case strings.HasPrefix(line, "+++ "):
			if cur == nil {
				cur = &patchFile{}
			}
			name, err := patchHeaderName(line)
			if err != nil {
				return nil, err
			}
			cur.newName = name
			cur.newHeader = line
		case strings.HasPrefix(line, "@@"):
			if cur == nil {
				cur = &patchFile{}
			}
			h, err := patchParseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			hunk = &h
			oldRemain, newRemain = h.oldCount, h.newCount
		case strings.HasPrefix(line, "*** "), strings.HasPrefix(line, "********"):
			return nil, fmt.Errorf("context and ed format diffs are not supported; use a unified diff (diff -u)")
		case strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, " differ"):
			return nil, fmt.Errorf("binary diffs are not supported")
		case strings.HasPrefix(line, "GIT binary patch"):
			return nil, fmt.Errorf("binary diffs are not supported")
		case patchEdCommand(line):
			return nil, fmt.Errorf("context and ed format diffs are not supported; use a unified diff (diff -u)")
		}
	}

	if hunk != nil && (oldRemain != 0 || newRemain != 0) {
		return nil, patchTruncatedHunk(source, len(raw), oldRemain, newRemain)
	}
	flushFile()

	if opts.reverse {
		for i := range files {
			patchReverseFile(&files[i])
		}
	}
	return files, nil
}

func patchTruncatedHunk(source string, lineNo, oldRemain, newRemain int) error {
	return fmt.Errorf("%s:%d: malformed hunk (expected %d more old / %d more new lines)",
		source, lineNo, max(0, oldRemain), max(0, newRemain))
}

// patchReverseFile turns a file section around for -R: the applier only ever
// runs forward, so names, ranges and body kinds are swapped here.
func patchReverseFile(pf *patchFile) {
	pf.oldName, pf.newName = pf.newName, pf.oldName
	pf.oldHeader, pf.newHeader = pf.newHeader, pf.oldHeader
	for i := range pf.hunks {
		h := &pf.hunks[i]
		h.oldStart, h.newStart = h.newStart, h.oldStart
		h.oldCount, h.newCount = h.newCount, h.oldCount
		for j := range h.lines {
			switch h.lines[j].kind {
			case '+':
				h.lines[j].kind = '-'
			case '-':
				h.lines[j].kind = '+'
			}
		}
	}
}

// patchHeaderName parses the name after "--- " or "+++ ", trimming a trailing
// timestamp and honouring toybox's "date <= epoch means /dev/null" rule.
func patchHeaderName(line string) (string, error) {
	rest := line[4:]
	if tab := strings.IndexByte(rest, '\t'); tab >= 0 {
		name, tail := rest[:tab], rest[tab+1:]
		if year := patchLeadingInt(tail); year > 1900 && year <= 1970 {
			return "/dev/null", nil
		}
		return patchUnquoteName(name)
	}
	return patchUnquoteName(rest)
}

// patchLeadingInt reads the leading decimal integer of s, skipping blanks.
func patchLeadingInt(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	start, n := i, 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}
	if i == start {
		return 0
	}
	return n
}

// patchUnquoteName decodes the C-style quoting diff uses for awkward names.
func patchUnquoteName(s string) (string, error) {
	if !strings.HasPrefix(s, `"`) {
		return s, nil
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return b.String(), nil
		}
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			return "", fmt.Errorf("bad %s", s)
		}
		e := s[i]
		if e >= '0' && e <= '7' {
			val, n := 0, 0
			for n < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
				val = val*8 + int(s[i]-'0')
				i++
				n++
			}
			b.WriteByte(byte(val))
			continue
		}
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			b.WriteByte(e)
		}
		i++
	}
	return "", fmt.Errorf("bad %s", s)
}

// patchParseHunkHeader parses "@@ -old[,count] +new[,count] @@[ text]".
func patchParseHunkHeader(line string) (patchHunk, error) {
	bad := func() (patchHunk, error) {
		return patchHunk{}, fmt.Errorf("malformed hunk header: %s", line)
	}
	rest := strings.TrimPrefix(line, "@@ -")
	i := 0
	readNum := func() (int, bool) {
		start, n := i, 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			n = n*10 + int(rest[i]-'0')
			i++
		}
		return n, i > start
	}

	var h patchHunk
	var ok bool
	if h.oldStart, ok = readNum(); !ok {
		return bad()
	}
	h.oldCount = 1
	if i < len(rest) && rest[i] == ',' {
		i++
		if h.oldCount, ok = readNum(); !ok {
			return bad()
		}
	}
	if i+1 >= len(rest) || rest[i] != ' ' || rest[i+1] != '+' {
		return bad()
	}
	i += 2
	if h.newStart, ok = readNum(); !ok {
		return bad()
	}
	h.newCount = 1
	if i < len(rest) && rest[i] == ',' {
		i++
		if h.newCount, ok = readNum(); !ok {
			return bad()
		}
	}
	if !strings.HasPrefix(rest[i:], " @@") {
		return bad()
	}
	h.raw = []string{line}
	return h, nil
}

// patchEdCommand recognises "3c3"-style normal/ed diff commands.
func patchEdCommand(line string) bool {
	i, n := 0, len(line)
	digits := func() bool {
		start := i
		for i < n && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		return i > start
	}
	if !digits() {
		return false
	}
	if i < n && line[i] == ',' {
		i++
		if !digits() {
			return false
		}
	}
	if i >= n || (line[i] != 'a' && line[i] != 'c' && line[i] != 'd') {
		return false
	}
	i++
	if !digits() {
		return false
	}
	return i == n
}
