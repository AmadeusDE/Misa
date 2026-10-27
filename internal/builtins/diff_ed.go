// Behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"errors"
	"fmt"
	"strings"
)

func diffEd(a, b []diffLine, edits []diffEdit) (string, error) {
	if len(a) > 0 && !a[len(a)-1].newline || len(b) > 0 && !b[len(b)-1].newline {
		return "", errors.New("No newline at end of file")
	}
	// GNU diff -e emits hunks last-to-first using original line numbers, so
	// each command stays valid after the later ones have been applied.
	var out strings.Builder
	changes := diffChanges(edits)
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		na, nb := c.a1-c.a0, c.b1-c.b0
		start, end := c.a0+1, c.a1
		rng := fmt.Sprintf("%d", start)
		if na > 1 {
			rng = fmt.Sprintf("%d,%d", start, end)
		}
		switch {
		case na == 0:
			fmt.Fprintf(&out, "%da\n", c.a0)
		case nb == 0:
			fmt.Fprintf(&out, "%sd\n", rng)
		default:
			fmt.Fprintf(&out, "%sc\n", rng)
		}
		dotLast := false
		for _, line := range b[c.b0:c.b1] {
			dotLast = line.text == "."
			if dotLast {
				// Append a literal dot via a one-character substitution.
				out.WriteString("..\n.\ns/.//\na\n")
			} else {
				out.WriteString(line.text)
				out.WriteByte('\n')
			}
		}
		if dotLast {
			// GNU ends the hunk right after the substitution, without
			// reopening append mode.
			trimmed := strings.TrimSuffix(out.String(), "a\n")
			out.Reset()
			out.WriteString(trimmed)
		} else if nb > 0 {
			out.WriteString(".\n")
		}
	}
	return out.String(), nil
}
