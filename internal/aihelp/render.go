package aihelp

import (
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// IsTTY reports whether w is a terminal that should get styled output: not NO_COLOR, not TERM=dumb.
func IsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Width is the column count of w when it is a terminal, else COLUMNS, else 80.
func Width(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
			return cols
		}
	}
	if cols, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && cols > 0 {
		return cols
	}
	return 80
}

// Markdown renders markdown for a terminal of the given width.
func Markdown(md string, width int) string {
	style := "dark"
	if lightBackground() {
		style = "light"
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(min(width, 100)))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return trimPadding(strings.Trim(out, "\n"))
}

// reTrailing matches the blanks glamour pads each line with, and the style codes around them.
var reTrailing = regexp.MustCompile(`(?:\x1b\[[0-9;]*m| )+$`)

// trimPadding drops that padding, so copied text carries no trailing spaces; a line that had styling gets one
// reset back.
func trimPadding(rendered string) string {
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if tail := reTrailing.FindString(line); tail != "" {
			line = line[:len(line)-len(tail)]
			if line != "" && strings.Contains(tail, "\x1b") {
				line += ansi.ResetStyle
			}
			lines[i] = line
		}
	}
	return strings.Join(lines, "\n")
}

// lightBackground reads COLORFGBG ("fg;bg"), which many terminals set; dark is the default.
func lightBackground() bool {
	parts := strings.Split(os.Getenv("COLORFGBG"), ";")
	bg, err := strconv.Atoi(parts[len(parts)-1])
	return err == nil && (bg == 7 || bg == 15)
}

// Printer writes a streamed answer. Plain printers pass text through; styled ones hold text back until a block
// (up to a blank line, never inside a code fence) is complete and write it rendered.
type Printer struct {
	w      io.Writer
	render func(string) string // nil: pass through
	buf    strings.Builder
	wrote  bool
}

// NewPrinter styles when styled is set, rendering for the given width.
func NewPrinter(w io.Writer, styled bool, width int) *Printer {
	p := &Printer{w: w}
	if styled {
		p.render = func(md string) string { return Markdown(md, width) }
	}
	return p
}

func (p *Printer) styled() bool { return p.render != nil }

// Write adds streamed text.
func (p *Printer) Write(s string) {
	if p.render == nil {
		io.WriteString(p.w, s)
		return
	}
	p.buf.WriteString(s)
	for {
		block, rest, ok := nextBlock(p.buf.String())
		if !ok {
			return
		}
		p.emit(block)
		p.buf.Reset()
		p.buf.WriteString(rest)
	}
}

// Close writes what is left and ends the output with a newline.
func (p *Printer) Close() {
	if p.render == nil {
		return
	}
	p.emit(p.buf.String())
	p.buf.Reset()
	if p.wrote {
		io.WriteString(p.w, "\n")
	}
}

func (p *Printer) emit(block string) {
	if strings.TrimSpace(block) == "" {
		return
	}
	if p.wrote {
		io.WriteString(p.w, "\n\n")
	}
	io.WriteString(p.w, p.render(strings.Trim(block, "\n")))
	p.wrote = true
}

// nextBlock splits off the first complete block: text up to a blank line outside a code fence.
func nextBlock(s string) (block, rest string, ok bool) {
	fenced := false
	pos := 0
	for pos < len(s) {
		nl := strings.IndexByte(s[pos:], '\n')
		if nl < 0 {
			return "", s, false
		}
		line := s[pos : pos+nl]
		pos += nl + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if !fenced && trimmed == "" && strings.TrimSpace(s[:pos-nl-1]) != "" {
			return strings.TrimRight(s[:pos-nl-1], "\n"), s[pos:], true
		}
	}
	return "", s, false
}
