// Side-by-side formatting ported from uutils diffutils (MIT/Apache-2.0).
// Source: src/side_diff.rs (https://github.com/uutils/diffutils).
package builtins

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const diffGutterWidthMin = 3

type diffSideConfig struct {
	halfWidth, columnTwoOffset, tabSize, separatorPos int
	expanded                                          bool
}

func diffNewSideConfig(width, tabSize int, expanded bool) diffSideConfig {
	if tabSize <= 0 {
		tabSize = 8
	}
	w, gutter := width, tabSize+diffGutterWidthMin
	off := (w >> 1) + (gutter >> 1) + (w & gutter & 1)
	off -= off % tabSize
	half := max(0, min(off-diffGutterWidthMin, w-off))
	c2o := w
	if half != 0 {
		c2o = off
	}
	return diffSideConfig{half, c2o, tabSize, (half + c2o - 1) >> 1, expanded}
}

var diffWideRanges = [...][2]rune{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF},
	{0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF},
	{0xA960, 0xA97F}, {0xAC00, 0xD7A3}, {0xF900, 0xFAFF},
	{0xFE10, 0xFE19}, {0xFE30, 0xFE6F}, {0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6}, {0x1B000, 0x1B001}, {0x1F200, 0x1F251},
	{0x1F300, 0x1F64F}, {0x1F900, 0x1F9FF}, {0x20000, 0x2FFFD},
	{0x30000, 0x3FFFD},
}

func diffRuneWidth(r rune) int {
	switch {
	case r == 0, unicode.IsControl(r):
		return 0
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r), unicode.Is(unicode.Cf, r):
		return 0
	}
	for _, rg := range diffWideRanges {
		if r >= rg[0] && r <= rg[1] {
			return 2
		}
	}
	return 1
}

// diffFormatTabsAndSpaces writes the whitespace between display columns from and to.
func diffFormatTabsAndSpaces(from, to int, c diffSideConfig, buf *strings.Builder) {
	if from > to {
		return
	}
	if c.expanded {
		buf.WriteString(strings.Repeat(" ", to-from))
		return
	}
	current := from
	for current+(c.tabSize-current%c.tabSize) <= to {
		buf.WriteByte('\t')
		current += c.tabSize - current%c.tabSize
	}
	buf.WriteString(strings.Repeat(" ", to-current))
}

// diffProcessHalfLine writes one side of a line, padded to maxWidth display columns.
func diffProcessHalfLine(s string, maxWidth int, isRight, wsGutter bool, c diffSideConfig, buf *strings.Builder) {
	gutter := 1
	if wsGutter {
		gutter = diffGutterWidthMin
	}
	if s == "" {
		if !isRight {
			diffFormatTabsAndSpaces(0, maxWidth+gutter, c, buf)
		}
		return
	}
	if maxWidth > c.halfWidth || maxWidth > c.columnTwoOffset && !isRight {
		return
	}
	width := 0
loop:
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		cw := diffRuneWidth(r)
		if width+cw > maxWidth {
			break
		}
		i += size
		switch r {
		case '\t':
			if adv := c.tabSize - width%c.tabSize; width+adv <= maxWidth {
				if c.expanded {
					buf.WriteString(strings.Repeat(" ", adv))
				} else {
					buf.WriteByte('\t')
				}
				width += adv
			}
		case '\n':
			break loop
		case '\r':
			buf.WriteByte('\r')
			diffFormatTabsAndSpaces(0, c.columnTwoOffset, c, buf)
			width = 0
		default:
			buf.WriteString(s[i-size : i])
			width += cw
		}
	}
	if !isRight {
		diffFormatTabsAndSpaces(width, maxWidth+gutter, c, buf)
	}
}

// diffPushOutput writes one side-by-side row: left half, gutter symbol, right half.
func diffPushOutput(left, right string, symbol byte, c diffSideConfig, buf *strings.Builder) {
	if left == "" && right == "" {
		buf.WriteByte('\n')
		return
	}
	wsGutter := symbol == ' '
	diffProcessHalfLine(left, c.halfWidth, false, wsGutter, c, buf)
	if symbol != ' ' {
		diffFormatTabsAndSpaces(c.halfWidth+1, c.separatorPos, c, buf)
		buf.WriteByte(symbol)
		if right != "" {
			diffFormatTabsAndSpaces(c.separatorPos+1, c.columnTwoOffset, c, buf)
		}
	}
	diffProcessHalfLine(right, c.halfWidth, true, wsGutter, c, buf)
	buf.WriteByte('\n')
}

func diffRawLine(line diffLine) string {
	if line.newline {
		return line.text + "\n"
	}
	return line.text
}

// diffGNUSide mirrors GNU diffutils side.c (print_1sdiff_line and friends).
type diffGNUSide struct {
	hw, c2o, tabsize int
	expand           bool
	buf              *strings.Builder
}

func diffNewGNUSide(width, tabsize int, expand bool, buf *strings.Builder) *diffGNUSide {
	if tabsize <= 0 {
		tabsize = 8
	}
	t := tabsize
	if expand {
		t = 1
	}
	off := (width + t + diffGutterWidthMin) / (2 * t) * t
	hw := max(0, min(off-diffGutterWidthMin, width-off))
	c2o := width
	if hw != 0 {
		c2o = off
	}
	return &diffGNUSide{hw, c2o, tabsize, expand, buf}
}

func (g *diffGNUSide) tabFromTo(from, to int) int {
	if !g.expand {
		for tab := from + g.tabsize - from%g.tabsize; tab <= to; tab += g.tabsize {
			g.buf.WriteByte('\t')
			from = tab
		}
	}
	for ; from < to; from++ {
		g.buf.WriteByte(' ')
	}
	return to
}

func (g *diffGNUSide) halfLine(s string, indent, bound int) int {
	in, out := 0, 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		raw := s[i : i+size]
		i += size
		switch r {
		case '\n':
			return out
		case '\t':
			spaces := g.tabsize - in%g.tabsize
			if in == out {
				stop := out + spaces
				if g.expand {
					stop = min(stop, bound)
					for ; out < stop; out++ {
						g.buf.WriteByte(' ')
					}
				} else if stop < bound {
					out = stop
					g.buf.WriteByte('\t')
				}
			}
			in += spaces
		case '\r':
			g.buf.WriteByte('\r')
			g.tabFromTo(0, indent)
			in, out = 0, 0
		case '\b':
			if in != 0 {
				in--
				if in < bound {
					if out <= in {
						for ; out < in; out++ {
							g.buf.WriteByte(' ')
						}
					} else {
						out = in
						g.buf.WriteByte('\b')
					}
				}
			}
		default:
			w := 1
			if r != utf8.RuneError || size != 1 {
				w = diffRuneWidth(r)
			}
			if in+w <= bound {
				in += w
				out = in
				g.buf.WriteString(raw)
			} else {
				in += w
			}
		}
	}
	return out
}

func (g *diffGNUSide) row(left *diffLine, sep byte, right *diffLine) {
	col, nl := 0, false
	if left != nil {
		nl = nl || left.newline
		col = g.halfLine(left.text, 0, g.hw)
	}
	if sep != ' ' {
		col = g.tabFromTo(col, (g.hw+g.c2o-1)/2) + 1
		if sep == '|' && left.newline != right.newline {
			if nl {
				sep = '/'
			} else {
				sep = '\\'
			}
		}
		g.buf.WriteByte(sep)
	}
	if right != nil {
		nl = nl || right.newline
		if right.text != "" {
			col = g.tabFromTo(col, g.c2o)
			g.halfLine(right.text, col, g.hw)
		}
	}
	if nl {
		g.buf.WriteByte('\n')
	}
}

func diffSide(a, b []diffLine, edits []diffEdit, opts diffOptions) string {
	var buf strings.Builder
	g := diffNewGNUSide(opts.width, opts.tabsize, opts.expandTabs, &buf)
	for i := 0; i < len(edits); {
		if edits[i].kind == ' ' {
			e := edits[i]
			i++
			if opts.suppressCommon {
				continue
			}
			if opts.leftColumn {
				g.row(&a[e.a], '(', nil)
			} else {
				g.row(&a[e.a], ' ', &b[e.b])
			}
			continue
		}
		// One hunk of consecutive deletions/insertions; GNU pairs the first
		// min(dels, adds) rows as changes ("|").
		var dels, adds []diffEdit
		for ; i < len(edits) && edits[i].kind != ' '; i++ {
			if edits[i].kind == '-' {
				dels = append(dels, edits[i])
			} else {
				adds = append(adds, edits[i])
			}
		}
		for k := 0; k < max(len(dels), len(adds)); k++ {
			switch {
			case k < len(dels) && k < len(adds):
				g.row(&a[dels[k].a], '|', &b[adds[k].b])
			case k < len(dels):
				g.row(&a[dels[k].a], '<', nil)
			default:
				g.row(nil, '>', &b[adds[k].b])
			}
		}
	}
	return buf.String()
}
