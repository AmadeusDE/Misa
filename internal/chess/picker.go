package chess

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// variantPicker holds a selection in the filtered list, not in variants.
type variantPicker struct {
	filter   string
	selected int
}

func (g *Game) openVariantPicker() {
	p := &variantPicker{}
	if i := slices.Index(g.variants, g.match.Variant()); i >= 0 {
		p.selected = i
	}
	g.picker = p
}

func (g *Game) pickerMatches() []int {
	var matches []int
	filter := strings.ToLower(g.picker.filter)
	for i, v := range g.variants {
		if strings.Contains(strings.ToLower(v.Name), filter) || strings.Contains(strings.ToLower(v.ID), filter) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (g *Game) pickerKey(k string) {
	p := g.picker
	switch k {
	case "ctrl+c":
		g.quit = true
	case "esc":
		g.picker = nil
	case "enter":
		g.choosePicker(p.selected)
	case "up", "ctrl+p":
		p.selected = max(0, p.selected-1)
	case "down", "ctrl+n":
		p.selected = min(max(0, len(g.pickerMatches())-1), p.selected+1)
	case "backspace":
		if p.filter != "" {
			_, size := utf8.DecodeLastRuneInString(p.filter)
			p.filter = p.filter[:len(p.filter)-size]
			p.selected = 0
		}
	default:
		if p.filter == "" && (k == "j" || k == "k") {
			if k == "j" {
				p.selected = min(max(0, len(g.pickerMatches())-1), p.selected+1)
			} else {
				p.selected = max(0, p.selected-1)
			}
			return
		}
		if k == "space" {
			k = " "
		}
		r, size := utf8.DecodeRuneInString(k)
		if size == len(k) && unicode.IsPrint(r) {
			g.pickerText(k)
		}
	}
}

// choosePicker accepts a row in the filtered list, leaving the game intact
// when it is already the current variant.
func (g *Game) choosePicker(row int) {
	matches := g.pickerMatches()
	if row < 0 || row >= len(matches) {
		return
	}
	g.setVariant(matches[row])
	g.picker = nil
}

// pickerText accepts text events containing one or more printable Unicode runes.
func (g *Game) pickerText(text string) {
	if text == "" || !utf8.ValidString(text) {
		return
	}
	for _, r := range text {
		if !unicode.IsPrint(r) {
			return
		}
	}
	g.picker.filter += text
	g.picker.selected = 0
}

// pickerLayout gives the list's screen coordinates for both rendering and clicks.
type pickerLayout struct {
	left, top, width, height int
	header, filter, help     bool
	listY, listRows, first   int
}

func (g *Game) pickerLayout() pickerLayout {
	// The box is twenty rows tall unless the list needs more: header, filter
	// and help row plus a row per variant, as far as the terminal allows.
	w, h := min(g.width, 60), min(g.height, max(20, len(g.variants)+5))
	l := pickerLayout{width: w, height: h, left: max(0, (g.width-w)/2), top: max(0, (g.height-h)/2)}
	if w < 2 || h < 2 {
		return l
	}
	inner := h - 2
	l.header = inner >= 3
	l.filter = inner >= 2
	l.help = inner >= 5
	l.listRows = inner
	if l.header {
		l.listRows--
	}
	if l.filter {
		l.listRows--
	}
	if l.help {
		l.listRows--
	}
	l.listY = l.top + 1
	if l.header {
		l.listY++
	}
	if l.filter {
		l.listY++
	}
	if l.listRows > 0 {
		l.first = max(0, g.picker.selected-l.listRows+1)
	}
	return l
}

func (g *Game) pickerClick(m tea.Mouse) {
	if m.Button != tea.MouseLeft {
		return
	}
	l := g.pickerLayout()
	row := m.Y - l.listY
	matches := g.pickerMatches()
	if m.X > l.left && m.X < l.left+l.width-1 && row >= 0 && row < l.listRows && l.first+row < len(matches) {
		g.choosePicker(l.first + row)
	}
}

func (g *Game) pickerView() string {
	l := g.pickerLayout()
	if l.width < 2 || l.height < 2 {
		return ansi.Truncate("Variants", l.width, "")
	}
	inner := l.width - 2
	border := lipgloss.NewStyle().Foreground(borderPlaying)
	// Truncate by terminal cells, then pad before styling so every row fits.
	row := func(text string, style lipgloss.Style) string {
		text = ansi.Truncate(text, inner, "")
		text += strings.Repeat(" ", max(0, inner-lipgloss.Width(text)))
		return border.Render("│") + style.Render(text) + border.Render("│")
	}
	lines := make([]string, 0, l.height)
	lines = append(lines, border.Render("╭"+strings.Repeat("─", inner)+"╮"))
	if l.header {
		lines = append(lines, row(" Chess variants", headerSt))
	}
	if l.filter {
		lines = append(lines, row(" Search: "+g.picker.filter, statusSt))
	}
	matches := g.pickerMatches()
	for i := 0; i < l.listRows; i++ {
		index := l.first + i
		text, style := "", statusSt
		if len(matches) == 0 && i == 0 {
			text = " No matching variants"
		}
		if index < len(matches) {
			v := g.variants[matches[index]]
			text = " " + v.Name + " (" + v.ID + ")"
			if index == g.picker.selected {
				style = choiceSt
			}
		}
		lines = append(lines, row(text, style))
	}
	if l.help {
		lines = append(lines, row(" type filter  ↑/↓ (j/k empty)  enter switch  esc cancel", helpSt))
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", inner)+"╯"))
	pad := strings.Repeat(" ", l.left)
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Repeat("\n", l.top) + strings.Join(lines, "\n")
}
