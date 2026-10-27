// Tab-expansion behaviour adapted from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type diffLine struct {
	text    string
	newline bool
}
type diffEdit struct {
	kind byte
	a, b int
}
type diffChange struct{ a0, a1, b0, b1 int }

func diffLines(data []byte) []diffLine {
	if len(data) == 0 {
		return nil
	}
	parts := strings.Split(string(data), "\n")
	lines := make([]diffLine, 0, len(parts))
	for i, text := range parts {
		if i == len(parts)-1 {
			if text != "" {
				lines = append(lines, diffLine{text, false})
			}
		} else {
			lines = append(lines, diffLine{text, true})
		}
	}
	return lines
}

func diffKey(line diffLine, o diffOptions) diffLine {
	if o.ignoreTabs && !o.ignoreTrailing {
		line.text = diffExpand(line.text, o.tabsize)
	}
	if o.ignoreTrailing {
		line.newline = true
		line.text = strings.TrimRightFunc(line.text, unicode.IsSpace)
	}
	if o.ignoreAllSpace {
		line.text = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, line.text)
	} else if o.ignoreSpace {
		var out strings.Builder
		space := false
		for _, r := range line.text {
			if unicode.IsSpace(r) {
				space = true
				continue
			}
			if space {
				out.WriteByte(' ')
				space = false
			}
			out.WriteRune(r)
		}
		line.text = out.String()
	}
	if o.ignoreCase {
		line.text = strings.ToLower(line.text)
	}
	return line
}

func diffEqual(a, b []diffLine, o diffOptions) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if diffKey(a[i], o) != diffKey(b[i], o) {
			return false
		}
	}
	return true
}

func diffMyers(a, b []diffLine, o diffOptions) []diffEdit {
	ak, bk := make([]diffLine, len(a)), make([]diffLine, len(b))
	for i := range a {
		ak[i] = diffKey(a[i], o)
	}
	for i := range b {
		bk[i] = diffKey(b[i], o)
	}
	prefix := 0
	for prefix < len(a) && prefix < len(b) && ak[prefix] == bk[prefix] {
		prefix++
	}
	ae, be := len(a), len(b)
	for ae > prefix && be > prefix && ak[ae-1] == bk[be-1] {
		ae--
		be--
	}
	edits := make([]diffEdit, 0, len(a)+len(b))
	for i := 0; i < prefix; i++ {
		edits = append(edits, diffEdit{' ', i, i})
	}
	n, m := ae-prefix, be-prefix
	if n == 0 || m == 0 {
		for i := prefix; i < ae; i++ {
			edits = append(edits, diffEdit{'-', i, prefix})
		}
		for j := prefix; j < be; j++ {
			edits = append(edits, diffEdit{'+', ae, j})
		}
	} else {
		maxD := n + m
		offset := maxD + 1
		v := make([]int, 2*maxD+3)
		var trace [][]int
		done := false
		for d := 0; d <= maxD && !done; d++ {
			trace = append(trace, append([]int(nil), v[offset-d-1:offset+d+2]...))
			for k := -d; k <= d; k += 2 {
				idx := offset + k
				var x int
				if k == -d || (k != d && v[idx-1] < v[idx+1]) {
					x = v[idx+1]
				} else {
					x = v[idx-1] + 1
				}
				y := x - k
				for x < n && y < m && ak[prefix+x] == bk[prefix+y] {
					x++
					y++
				}
				v[idx] = x
				if x >= n && y >= m {
					done = true
					break
				}
			}
		}
		x, y := n, m
		var reversed []byte
		for d := len(trace) - 1; d >= 0; d-- {
			prev := trace[d]
			base := d + 1
			k := x - y
			var pk int
			if k == -d || (k != d && prev[base+k-1] < prev[base+k+1]) {
				pk = k + 1
			} else {
				pk = k - 1
			}
			px := prev[base+pk]
			py := px - pk
			for x > px && y > py {
				reversed = append(reversed, ' ')
				x--
				y--
			}
			if d > 0 {
				if x == px {
					reversed = append(reversed, '+')
					y--
				} else {
					reversed = append(reversed, '-')
					x--
				}
			}
		}
		// Keep each replacement's deletions before its insertions.
		var kinds []byte
		for i := len(reversed) - 1; i >= 0; {
			if reversed[i] == ' ' {
				kinds = append(kinds, ' ')
				i--
				continue
			}
			minus, plus := 0, 0
			for i >= 0 && reversed[i] != ' ' {
				if reversed[i] == '-' {
					minus++
				} else {
					plus++
				}
				i--
			}
			for j := 0; j < minus; j++ {
				kinds = append(kinds, '-')
			}
			for j := 0; j < plus; j++ {
				kinds = append(kinds, '+')
			}
		}
		x, y = prefix, prefix
		for _, kind := range kinds {
			edits = append(edits, diffEdit{kind, x, y})
			if kind != '+' {
				x++
			}
			if kind != '-' {
				y++
			}
		}
	}
	for i, j := ae, be; i < len(a); i, j = i+1, j+1 {
		edits = append(edits, diffEdit{' ', i, j})
	}
	return edits
}

func diffChanges(edits []diffEdit) []diffChange {
	var changes []diffChange
	for i := 0; i < len(edits); {
		if edits[i].kind == ' ' {
			i++
			continue
		}
		c := diffChange{a0: edits[i].a, a1: edits[i].a, b0: edits[i].b, b1: edits[i].b}
		for i < len(edits) && edits[i].kind != ' ' {
			if edits[i].kind == '-' {
				c.a1++
			} else {
				c.b1++
			}
			i++
		}
		changes = append(changes, c)
	}
	return changes
}

func diffExpand(text string, tabsize int) string {
	if tabsize <= 0 {
		tabsize = 8
	}
	var out strings.Builder
	parts := strings.Split(text, "\t")
	for i, part := range parts {
		out.WriteString(part)
		if i < len(parts)-1 {
			out.WriteString(strings.Repeat(" ", tabsize-diffDisplayWidth(part)%tabsize))
		}
	}
	return out.String()
}

func diffDisplayWidth(text string) int {
	if !utf8.ValidString(text) {
		return len(text)
	}
	width := 0
	joined := false
	regional := false
	for _, r := range text {
		if r == 0x200d {
			joined = true
			continue
		}
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) || r >= 0x1f3fb && r <= 0x1f3ff {
			continue
		}
		if joined {
			joined = false
			continue
		}
		if r >= 0x1f1e6 && r <= 0x1f1ff {
			if regional {
				regional = false
				continue
			}
			regional = true
		} else {
			regional = false
		}
		if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xfe10 && r <= 0xfe6f || r >= 0xff00 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 || r >= 0x1f000 && r <= 0x1faff || r >= 0x20000 && r <= 0x3fffd) {
			width += 2
		} else {
			width++
		}
	}
	return width
}
