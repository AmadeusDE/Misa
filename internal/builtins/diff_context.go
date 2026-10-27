// Behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"fmt"
	"strings"
)

// diffContextRange formats a 1-based line range, or the empty-side
// placeholder used when a hunk has no lines on one side.
func diffContextRange(n0, n1 int) string {
	if n0 == n1 {
		return diffRange(n0-1, n0)
	}
	return diffRange(n0, n1)
}

// diffContextMarks maps each edit of a hunk to its two-byte line marker:
// "! " inside replacements, "- " or "+ " for one-sided changes, "  " for context.
func diffContextMarks(edits []diffEdit) []string {
	marks := make([]string, len(edits))
	for i := 0; i < len(edits); {
		if edits[i].kind == ' ' {
			i++
			continue
		}
		j, del, ins := i, false, false
		for j < len(edits) && edits[j].kind != ' ' {
			del = del || edits[j].kind == '-'
			ins = ins || edits[j].kind == '+'
			j++
		}
		mark := "- "
		if ins {
			mark = "+ "
		}
		if del && ins {
			mark = "! "
		}
		for ; i < j; i++ {
			marks[i] = mark
		}
	}
	return marks
}

func diffContext(a, b []diffLine, edits []diffEdit, o diffOptions, headers [2]string) string {
	hunks := diffVisibleHunks(a, b, edits, o)
	if len(hunks) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "*** %s\n--- %s\n", headers[0], headers[1])
	for _, h := range hunks {
		slice := edits[h[0]:h[1]]
		marks := diffContextMarks(slice)
		first := slice[0]
		na, nb, del, ins := 0, 0, false, false
		for _, e := range slice {
			switch e.kind {
			case ' ':
				na, nb = na+1, nb+1
			case '-':
				na, del = na+1, true
			case '+':
				nb, ins = nb+1, true
			}
		}
		fmt.Fprintf(&out, "***************%s\n*** %s ****\n", diffFunction(a, first.a, o), diffContextRange(first.a, first.a+na))
		if del {
			for i, e := range slice {
				if e.kind == '+' {
					continue
				}
				mark := "  "
				if e.kind == '-' {
					mark = marks[i]
				}
				diffWriteLine(&out, mark, a[e.a], o)
			}
		}
		fmt.Fprintf(&out, "--- %s ----\n", diffContextRange(first.b, first.b+nb))
		if ins {
			for i, e := range slice {
				if e.kind == '-' {
					continue
				}
				mark := "  "
				if e.kind == '+' {
					mark = marks[i]
				}
				diffWriteLine(&out, mark, b[e.b], o)
			}
		}
	}
	return out.String()
}
