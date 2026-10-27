// Behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"fmt"
	"strings"
)

func diffRange(start, end int) string {
	if end == start+1 {
		return fmt.Sprint(end)
	}
	return fmt.Sprintf("%d,%d", start+1, end)
}

func diffWriteLine(out *strings.Builder, prefix string, line diffLine, o diffOptions) {
	text := line.text
	if o.expandTabs {
		text = diffExpand(text, o.tabsize)
	}
	if o.initialTab {
		prefix = strings.TrimSuffix(prefix, " ") + "\t"
	}
	out.WriteString(prefix)
	out.WriteString(text)
	out.WriteByte('\n')
	if !line.newline {
		out.WriteString("\\ No newline at end of file\n")
	}
}

func diffNormal(a, b []diffLine, edits []diffEdit, o diffOptions) string {
	var out strings.Builder
	for _, c := range diffVisibleChanges(a, b, edits, o) {
		switch {
		case c.a0 == c.a1:
			fmt.Fprintf(&out, "%da%s\n", c.a0, diffRange(c.b0, c.b1))
		case c.b0 == c.b1:
			fmt.Fprintf(&out, "%sd%d\n", diffRange(c.a0, c.a1), c.b0)
		default:
			fmt.Fprintf(&out, "%sc%s\n", diffRange(c.a0, c.a1), diffRange(c.b0, c.b1))
		}
		for _, line := range a[c.a0:c.a1] {
			diffWriteLine(&out, "< ", line, o)
		}
		if c.a0 != c.a1 && c.b0 != c.b1 {
			out.WriteString("---\n")
		}
		for _, line := range b[c.b0:c.b1] {
			diffWriteLine(&out, "> ", line, o)
		}
	}
	return out.String()
}
