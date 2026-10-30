package mines

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// customSetup is separate from the live game so cancellation leaves the board
// intact. The mine count follows the percent, so the screen edits the board's
// mine density rather than a raw count.
type customSetup struct {
	level   Level
	percent int
	field   int
	paused  time.Time
}

// setupFields names the three editable rows.
var setupFields = [3]string{"Width", "Height", "Mines %"}

// setupDeltas are the steps offered on every row: the same set the keys use.
var setupDeltas = [...]int{-100, -10, -1, 1, 10, 100}

// setupButton is one on-screen ±N control with its column range on the line.
type setupButton struct {
	delta int
	x, w  int
}

// customLine is one rendered row of the custom screen and the buttons on it.
// field is the editable row it belongs to, or -1.
type customLine struct {
	text    string
	x       int
	field   int
	buttons []setupButton
}

func setupLabel(delta int) string {
	if delta > 0 {
		return fmt.Sprintf("[+%d]", delta)
	}
	return fmt.Sprintf("[%d]", delta)
}

func (g *Game) setupKey(k string) tea.Cmd {
	s := g.setup
	switch k {
	case "c", "esc":
		g.setup = nil
		if !g.started.IsZero() && g.ended.IsZero() {
			g.started = g.started.Add(g.now().Sub(s.paused))
			return g.tick()
		}
	case "enter":
		g.setup = nil
		g.newGame(s.level)
	case "up", "k":
		s.field = (s.field + 2) % 3
	case "down", "j":
		s.field = (s.field + 1) % 3
	default:
		if delta, ok := setupStep(k); ok {
			g.adjustSetup(s.field, delta)
		}
	}
	return nil
}

// setupStep maps a key to a step. The arrows and h/l change by one, the square
// brackets by ten, and the braces or the page keys by a hundred. A terminal
// that reports shift+[ without its text still lands on the hundred.
func setupStep(k string) (int, bool) {
	switch k {
	case "left", "h":
		return -1, true
	case "right", "l":
		return 1, true
	case "[", "shift+left":
		return -10, true
	case "]", "shift+right":
		return 10, true
	case "{", "shift+{", "shift+[", "pgdown":
		return -100, true
	case "}", "shift+}", "shift+]", "pgup":
		return 100, true
	}
	return 0, false
}

// adjustSetup steps one field and recomputes the mine count from the percent.
func (g *Game) adjustSetup(field, delta int) {
	s := g.setup
	switch field {
	case 0:
		s.level.W = max(MinWidth, min(MaxWidth, s.level.W+delta))
	case 1:
		s.level.H = max(MinHeight, min(MaxHeight, s.level.H+delta))
	default:
		s.percent = max(MinPercent, min(MaxPercent, s.percent+delta))
	}
	s.level.Mines = PercentMines(s.level.W, s.level.H, s.percent)
}

// setupClick applies a left click on one of the screen's step buttons.
func (g *Game) setupClick(m tea.Mouse) {
	if g.setup == nil || m.Button != tea.MouseLeft {
		return
	}
	lines, top := g.customLines()
	y := m.Y - top
	if y < 0 || y >= len(lines) || top+y >= g.height || lines[y].field < 0 {
		return
	}
	for _, b := range lines[y].buttons {
		if m.X >= b.x && m.X < b.x+b.w {
			g.adjustSetup(lines[y].field, b.delta)
			return
		}
	}
}

// setupRowText lays out one field row and reports where its buttons sit:
// [-100] [-10] [-1]  value  [+1] [+10] [+100].
func setupRowText(name string, value int, selected bool) (string, []setupButton) {
	marker := "  "
	if selected {
		marker = "> "
	}
	var sb strings.Builder
	var buttons []setupButton
	sb.WriteString(marker)
	sb.WriteString(fmt.Sprintf("%-8s", name))
	cell := func(delta int, gap string) {
		sb.WriteString(gap)
		if delta == 0 {
			sb.WriteString(fmt.Sprintf("%4d", value))
			return
		}
		label := setupLabel(delta)
		buttons = append(buttons, setupButton{delta: delta, x: lipgloss.Width(sb.String()), w: lipgloss.Width(label)})
		sb.WriteString(label)
	}
	cell(-100, "  ")
	cell(-10, " ")
	cell(-1, " ")
	cell(0, "  ")
	cell(1, "  ")
	cell(10, " ")
	cell(100, " ")
	return sb.String(), buttons
}

// customLines builds the screen's rows with their centred columns, so the view
// and the mouse see the same layout.
func (g *Game) customLines() ([]customLine, int) {
	s := g.setup
	var lines []customLine
	add := func(text string, x, field int, buttons []setupButton) {
		for i := range buttons {
			buttons[i].x += x
		}
		lines = append(lines, customLine{text: text, x: x, field: field, buttons: buttons})
	}
	centre := func(text string) int { return max(0, (g.width-lipgloss.Width(text))/2) }
	add("Custom game", centre("Custom game"), -1, nil)
	add("", 0, -1, nil)
	// The field rows are centred as one block so their columns line up, even
	// though the percent row carries an extra mine count.
	values := [3]int{s.level.W, s.level.H, s.percent}
	rows := make([]struct {
		text    string
		buttons []setupButton
	}, len(setupFields))
	block := 0
	for i, name := range setupFields {
		text, buttons := setupRowText(name, values[i], i == s.field)
		if i == 2 {
			text += fmt.Sprintf("  (= %d mines)", s.level.Mines)
		}
		rows[i].text, rows[i].buttons = text, buttons
		block = max(block, lipgloss.Width(text))
	}
	indent := max(0, (g.width-block)/2)
	for i, row := range rows {
		add(row.text, indent, i, row.buttons)
	}
	add("", 0, -1, nil)
	l := boardLayout(s.level.W, s.level.H, g.width, g.height)
	switch {
	case l.fits:
		add("Board fits", centre("Board fits"), -1, nil)
	case l.viable:
		text := fmt.Sprintf("Board won't fit (needs %d×%d) — viewport scrolls", l.fullW(), l.fullH())
		add(text, centre(text), -1, nil)
	default:
		text := fmt.Sprintf("Terminal too small (needs %d×%d)", l.fullW(), l.fullH())
		add(text, centre(text), -1, nil)
	}
	add("", 0, -1, nil)
	for _, text := range []string{
		"↑/↓ select   ←/→ or h/l ±1",
		"[/] or shift+←/→ ±10   {/} or pgup/pgdn ±100",
		"enter start   c/esc cancel",
	} {
		add(text, centre(text), -1, nil)
	}
	return lines, max(0, (g.height-len(lines))/2)
}

func (g *Game) customView() string {
	lines, top := g.customLines()
	out := make([]string, top, g.height)
	for _, line := range lines {
		if len(out) >= g.height {
			break
		}
		out = append(out, strings.Repeat(" ", line.x)+truncate(line.text, max(0, g.width-line.x)))
	}
	for len(out) < g.height {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}
