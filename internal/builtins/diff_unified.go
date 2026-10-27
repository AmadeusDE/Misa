// Behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"fmt"
	"strings"
)

func diffUnifiedRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d,0", start)
	}
	if end == start+1 {
		return fmt.Sprint(end)
	}
	return fmt.Sprintf("%d,%d", start+1, end-start)
}

func diffHunks(edits []diffEdit, context int) [][2]int {
	var hunks [][2]int
	for i := 0; i < len(edits); {
		if edits[i].kind == ' ' {
			i++
			continue
		}
		lo := i - min(i, context)
		for i < len(edits) && edits[i].kind != ' ' {
			i++
		}
		hi := i + min(len(edits)-i, context)
		if len(hunks) > 0 && lo <= hunks[len(hunks)-1][1] {
			hunks[len(hunks)-1][1] = max(hi, hunks[len(hunks)-1][1])
		} else {
			hunks = append(hunks, [2]int{lo, hi})
		}
	}
	return hunks
}

func diffUnified(a, b []diffLine, edits []diffEdit, o diffOptions, headers [2]string) string {
	hunks := diffVisibleHunks(a, b, edits, o)
	if len(hunks) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", headers[0], headers[1])
	for _, h := range hunks {
		first := edits[h[0]]
		ae, be := first.a, first.b
		for _, e := range edits[h[0]:h[1]] {
			if e.kind != '+' {
				ae++
			}
			if e.kind != '-' {
				be++
			}
		}
		fmt.Fprintf(&out, "@@ -%s +%s @@%s\n", diffUnifiedRange(first.a, ae), diffUnifiedRange(first.b, be), diffFunction(a, first.a, o))
		for _, e := range edits[h[0]:h[1]] {
			switch e.kind {
			case ' ':
				diffWriteLine(&out, " ", a[e.a], o)
			case '-':
				diffWriteLine(&out, "-", a[e.a], o)
			case '+':
				diffWriteLine(&out, "+", b[e.b], o)
			}
		}
	}
	return out.String()
}
