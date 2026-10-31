// Behaviour modelled on toybox toys/posix/patch.c,
// landley/toybox@6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
// Native Go implementation; see docs/CREDITS.md and THIRD_PARTY_NOTICES.md.
package builtins

import (
	"fmt"
	"strings"
)

// patchSplitLines splits file content into lines, remembering which ones ended
// with a newline.
func patchSplitLines(data []byte) []patchLine {
	var out []patchLine
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			out = append(out, patchLine{text: string(data[start:i]), nl: true})
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, patchLine{text: string(data[start:]), nl: false})
	}
	return out
}

// patchJoinLines is the inverse of patchSplitLines.
func patchJoinLines(lines []patchLine) []byte {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text)
		if l.nl {
			b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

// patchLineEq compares an expected hunk line with a file line. A line marked
// "\ No newline at end of file" must meet a file line that has no newline, and
// an unmarked one a file line that has one: without that, a hunk whose old side
// ends unterminated would also match a terminated last line and duplicate it.
func patchLineEq(file patchLine, hunk patchHunkLine, loose bool) bool {
	if file.nl == hunk.noNL {
		return false
	}
	if !loose {
		return file.text == hunk.text
	}
	return patchLooseEq(file.text, hunk.text)
}

// patchLooseEq compares ignoring all spaces and tabs (toybox loosecmp).
func patchLooseEq(a, b string) bool {
	i, j := 0, 0
	for {
		for i < len(a) && (a[i] == ' ' || a[i] == '\t') {
			i++
		}
		for j < len(b) && (b[j] == ' ' || b[j] == '\t') {
			j++
		}
		if i >= len(a) || j >= len(b) {
			return i >= len(a) && j >= len(b)
		}
		if a[i] != b[j] {
			return false
		}
		i++
		j++
	}
}

// patchHunkMatch reports whether oldSide fits the file at index i with da
// leading and db trailing context lines dropped (fuzz).
func patchHunkMatch(lines []patchLine, oldSide []patchHunkLine, i, da, db int, loose bool) bool {
	lo, hi := da, len(oldSide)-db
	if hi < lo {
		hi = lo
	}
	if i < 0 || i+lo < 0 || i+hi > len(lines) {
		return false
	}
	for k := lo; k < hi; k++ {
		if !patchLineEq(lines[i+k], oldSide[k], loose) {
			return false
		}
	}
	return true
}

// patchFuzzAllowed applies toybox's policy: fuzzing needs at least two
// substantial lines of matched text, -F overrides, otherwise the default is
// one less than the number of leading context lines.
func patchFuzzAllowed(oldSide []patchHunkLine, leading int, opts patchOptions) int {
	substantial := 0
	for _, hl := range oldSide {
		if len(strings.TrimSpace(hl.text)) >= 2 {
			substantial++
		}
	}
	if substantial < 2 {
		return 0
	}
	if opts.fuzzSet {
		return opts.fuzz
	}
	if leading > 1 {
		return leading - 1
	}
	return 0
}

// applyPatchHunks runs the hunks in order over lines. It returns the patched
// result, the hunks that did not fit (failed), and every hunk that was not
// applied to the result (unapplied): once one hunk fails, the later hunks of
// that file cannot be trusted, and the file is never written anyway, so they
// belong in the reject file too.
func applyPatchHunks(lines []patchLine, hunks []patchHunk, opts patchOptions) (out []patchLine, failed, unapplied []int) {
	out = make([]patchLine, 0, len(lines))
	pos := 0

	for hi := range hunks {
		h := &hunks[hi]
		if len(failed) > 0 {
			unapplied = append(unapplied, hi)
			continue
		}

		var oldSide, newSide []patchHunkLine
		leading, trailing := 0, 0
		seenChange := false
		for _, hl := range h.lines {
			if hl.kind == ' ' || hl.kind == '-' {
				oldSide = append(oldSide, hl)
			}
			if hl.kind == ' ' || hl.kind == '+' {
				newSide = append(newSide, hl)
			}
			if hl.kind == ' ' {
				if !seenChange {
					leading++
				}
				trailing++
			} else {
				seenChange = true
				trailing = 0
			}
		}

		expected := h.oldStart - 1
		if h.oldCount == 0 {
			// A zero-length old side inserts before oldStart.
			expected = h.oldStart
		}
		expected = min(max(expected, 0), len(lines))

		allowed := patchFuzzAllowed(oldSide, leading, opts)

		found, usedFuzz, da, db := -1, 0, 0, 0
		for f := 0; f <= allowed && found < 0; f++ {
			fda, fdb := min(f, leading), min(f, trailing)
			// Prefer the declared position, then scan forward from pos.
			if expected >= pos && patchHunkMatch(lines, oldSide, expected, fda, fdb, opts.loose) {
				found, usedFuzz, da, db = expected, f, fda, fdb
				break
			}
			for i := pos; i <= len(lines); i++ {
				if i == expected {
					continue
				}
				if patchHunkMatch(lines, oldSide, i, fda, fdb, opts.loose) {
					found, usedFuzz, da, db = i, f, fda, fdb
					break
				}
			}
		}

		if found < 0 {
			failed = append(failed, hi)
			unapplied = append(unapplied, hi)
			if patchHunkMatch(lines, newSide, expected, 0, 0, opts.loose) {
				fmt.Fprintf(opts.stderr, "Possibly reversed hunk %d at %d\n", hi+1, expected+1)
			}
			if opts.debug > 1 {
				fmt.Fprintf(opts.stdout, "Hunk #%d: no match (expected line %d, fuzz %d)\n", hi+1, expected+1, allowed)
			}
			continue
		}

		if opts.verbose > 0 {
			if offset := found - expected; offset != 0 {
				fmt.Fprintf(opts.stdout, "Hunk #%d succeeded at %d (offset %d lines).\n", hi+1, found+1, offset)
			} else {
				fmt.Fprintf(opts.stdout, "Hunk #%d succeeded at %d.\n", hi+1, found+1)
			}
		}
		if opts.debug > 1 {
			fmt.Fprintf(opts.stdout, "Hunk #%d: matched line %d, fuzz %d\n", hi+1, found+1, usedFuzz)
		}

		out = append(out, lines[pos:found]...)
		oldIdx := found
		for _, hl := range h.lines {
			switch hl.kind {
			case ' ':
				if oldIdx >= len(lines) {
					break
				}
				line := lines[oldIdx]
				// Context lines inside the fuzz zone are copied from the file
				// untouched; matched ones take the hunk's newline state.
				if oldIdx >= found+da && oldIdx < found+len(oldSide)-db && hl.noNL {
					line.nl = false
				}
				out = append(out, line)
				oldIdx++
			case '-':
				oldIdx++
			case '+':
				out = append(out, patchLine{text: hl.text, nl: !hl.noNL})
			}
		}
		pos = found + len(oldSide)
	}

	return append(out, lines[pos:]...), failed, unapplied
}
