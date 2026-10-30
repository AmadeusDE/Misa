// Package cardui supplies original compact card-table presentation and interaction.
package cardui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/cards"
)

type Pile struct {
	Name   string
	Cards  []cards.Card
	Hidden int
	Marked map[int]bool
	Prefix map[int]string
}

// Table is a presentation boundary, not a shared rules engine. Hidden cards must be masked.
type Table interface {
	Piles() []Pile
	Status() string
	Act(from, row, to int) bool
	Undo() bool
	Redo() bool
	Hint() string
	Auto() bool
	Won() bool
	Help() string
}

// Computer returns a decision closure over an immutable, restricted observation.
type Computer interface {
	Work() func() any
	Accept(any)
}
type Options struct {
	Seed  uint64
	Theme string
	ASCII bool
	Rule  int
}
type Factory func(uint64) Table
type target struct {
	x, y, w   int
	pile, row int
	key       string
}
type result struct {
	owner *Game
	gen   uint64
	value any
}
type tick struct {
	owner *Game
	gen   uint64
}
type Game struct {
	table                  Table
	factory                Factory
	title, id              string
	opts                   Options
	width, height          int
	focus, row, helpOffset int
	selected, selectedRow  int
	note, confirm          string
	help, quit, pending    bool
	started, ended         time.Time
	gen                    uint64
	cancel                 context.CancelFunc
}

func New(title, id string, opts Options, f Factory) *Game {
	g := &Game{title: title, id: id, opts: opts, factory: f, width: 80, height: 24, selected: -1}
	g.reset(opts.Seed)
	return g
}
func (g *Game) reset(seed uint64) {
	g.stop()
	g.gen++
	g.opts.Seed = seed
	g.table = g.factory(seed)
	g.focus = 0
	g.row = 0
	g.helpOffset = 0
	g.selected = -1
	g.note = ""
	g.confirm = ""
	g.help = false
	g.started = time.Now()
	g.ended = time.Time{}
}
func (g *Game) stop() {
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	g.pending = false
}
func (g *Game) Close()          { g.stop(); g.gen++; g.quit = true }
func (g *Game) IsQuit() bool    { return g.quit }
func (g *Game) Resize(w, h int) { g.width = w; g.height = h; g.selected = -1 }
func (g *Game) clock() tea.Cmd {
	owner, gen := g, g.gen
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tick{owner, gen} })
}
func (g *Game) Init() tea.Cmd { return tea.Batch(g.clock(), g.schedule()) }
func (g *Game) schedule() tea.Cmd {
	if g.pending || g.quit {
		return nil
	}
	c, ok := g.table.(Computer)
	if !ok {
		return nil
	}
	work := c.Work()
	if work == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.pending = true
	gen := g.gen
	return func() tea.Msg {
		timer := time.NewTimer(180 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		value := work()
		select {
		case <-ctx.Done():
			return nil
		default:
			return result{g, gen, value}
		}
	}
}
func (g *Game) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		g.Resize(m.Width, m.Height)
	case tick:
		if m.owner != g || m.gen != g.gen || g.quit {
			return nil
		}
		return g.clock()
	case result:
		if m.owner != g || m.gen != g.gen || g.quit {
			return nil
		}
		g.stop()
		if c, ok := g.table.(Computer); ok {
			c.Accept(m.value)
		}
	case tea.KeyPressMsg:
		if cmd := g.key(m.String()); cmd != nil {
			return cmd
		}
	case tea.MouseClickMsg:
		if m.Button != tea.MouseLeft || g.small() {
			return nil
		}
		_, targets := g.draw()
		for _, t := range targets {
			if m.X >= t.x && m.X < t.x+t.w && m.Y == t.y {
				if t.key != "" {
					if cmd := g.key(t.key); cmd != nil {
						return cmd
					}
					break
				}
				g.focus = t.pile
				g.row = t.row
				g.activate()
				break
			}
		}
	}
	g.clampFocus()
	if g.table.Won() && g.ended.IsZero() {
		g.ended = time.Now()
		g.note = "Deal complete! Misa salutes your play."
	} else if !g.table.Won() {
		g.ended = time.Time{}
	}
	return g.schedule()
}
func (g *Game) clampFocus() {
	piles := g.table.Piles()
	if len(piles) == 0 {
		return
	}
	g.focus = max(0, min(g.focus, len(piles)-1))
	g.row = max(0, min(g.row, max(0, len(piles[g.focus].Cards)-1)))
}
func (g *Game) small() bool { return g.width < 24 || g.height < 14 }
func (g *Game) key(k string) tea.Cmd {
	if g.confirm != "" {
		if k == "y" {
			action := g.confirm
			g.confirm = ""
			switch action {
			case "quit":
				g.Close()
			case "restart":
				g.reset(g.opts.Seed)
				return tea.Batch(g.clock(), g.schedule())
			case "new":
				seed, err := cards.FreshSeed()
				if err != nil {
					g.note = "Cannot obtain a fresh seed: " + err.Error()
				} else {
					g.reset(seed)
					return tea.Batch(g.clock(), g.schedule())
				}
			}
		} else if k == "n" || k == "esc" {
			g.confirm = ""
		}
		return nil
	}
	if k == "q" || k == "ctrl+c" {
		g.confirm = "quit"
		return nil
	}
	if k == "?" || k == "f1" {
		g.help = !g.help
		g.helpOffset = 0
		return nil
	}
	if g.help {
		switch k {
		case "esc":
			g.help = false
		case "down", "pgdown", "space":
			body, count := g.helpPage()
			g.helpOffset = min(g.helpOffset+1, max(0, len(body)-count))
		case "up", "pgup":
			g.helpOffset = max(0, g.helpOffset-1)
		}
		return nil
	}
	if k == "n" {
		g.confirm = "new"
		return nil
	}
	if k == "r" {
		g.confirm = "restart"
		return nil
	}
	if g.small() {
		return nil
	}
	piles := g.table.Piles()
	if len(piles) == 0 {
		return nil
	}
	g.focus = min(g.focus, len(piles)-1)
	switch k {
	case "esc":
		g.selected = -1
	case "left", "shift+tab":
		g.focus = (g.focus + len(piles) - 1) % len(piles)
		g.row = max(0, len(piles[g.focus].Cards)-1)
	case "right", "tab":
		g.focus = (g.focus + 1) % len(piles)
		g.row = max(0, len(piles[g.focus].Cards)-1)
	case "up":
		g.row = max(0, g.row-1)
	case "down":
		g.row = min(max(0, len(piles[g.focus].Cards)-1), g.row+1)
	case "enter", "space":
		g.activate()
	case "u":
		if g.table.Undo() {
			g.selected = -1
			g.note = "Undone (clock continues)"
		}
	case "ctrl+r", "U":
		if g.table.Redo() {
			g.selected = -1
			g.note = "Redone"
		}
	case "h":
		g.note = g.table.Hint()
	case "a":
		if g.table.Auto() {
			g.selected = -1
			g.note = "Auto action completed"
		} else {
			g.note = "No verified safe auto action available"
		}
	}
	return nil
}
func (g *Game) activate() {
	if g.pending {
		return
	}
	piles := g.table.Piles()
	if g.focus >= len(piles) {
		return
	}
	g.row = min(g.row, max(0, len(piles[g.focus].Cards)-1))
	if g.selected < 0 {
		if g.table.Act(-1, g.row, g.focus) {
			g.note = ""
			return
		}
		if _, direct := g.table.(Computer); direct {
			g.note = "Select a legal card or action"
			return
		}
		if len(piles[g.focus].Cards) > 0 && g.row >= piles[g.focus].Hidden {
			g.selected = g.focus
			g.selectedRow = g.row
		}
		return
	}
	if g.table.Act(g.selected, g.selectedRow, g.focus) {
		g.note = ""
		g.selected = -1
	} else {
		g.note = "Illegal action; Escape cancels selection"
		if g.selected == g.focus {
			g.selected = -1
		}
	}
}
func (g *Game) View() string { s, _ := g.draw(); return s }
func (g *Game) draw() (string, []target) {
	if g.small() {
		prompt := "q: quit r: restart"
		if g.confirm != "" {
			prompt = g.confirm + "? y/n"
		}
		return lipgloss.NewStyle().MaxWidth(max(1, g.width)).MaxHeight(max(1, g.height)).Render("Misa " + g.title + "\nResize: need 24×14\nState preserved\n" + prompt), nil
	}
	bg, title, accent := "#126546", "#245b9c", "#f2d273"
	if g.opts.Theme == "default" {
		bg = "#282828"
		title = "#504945"
	}
	felt := lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color("#ffffff"))
	line := func(s string) string { return felt.Width(g.width).MaxWidth(g.width).MaxHeight(1).Render(s) }
	titleStyle := lipgloss.NewStyle().Background(lipgloss.Color(title)).Foreground(lipgloss.Color("#ffffff")).Bold(true).Width(g.width).MaxWidth(g.width).MaxHeight(1)
	lines := []string{titleStyle.Render(" Misa · " + g.title), line(cards.DealID(g.id, g.opts.Seed, g.opts.Rule))}
	status := strings.Split(lipgloss.NewStyle().Width(g.width).Render(g.table.Status()), "\n")
	for _, s := range status[:min(len(status), 4)] {
		lines = append(lines, line(s))
	}
	var targets []target
	buttons := []struct{ label, key string }{{"N", "n"}, {"R", "r"}, {"?", "?"}, {"Q", "q"}, {"U", "u"}, {"Y", "U"}, {"H", "h"}, {"A", "a"}}
	toolbar := ""
	buttonStyle := lipgloss.NewStyle().Background(lipgloss.Color("#bdc3cd")).Foreground(lipgloss.Color("#172235")).Bold(true)
	x := 0
	for _, b := range buttons {
		label := "[" + b.label + "]"
		if x+len(label) > g.width {
			break
		}
		targets = append(targets, target{x: x, y: len(lines), w: len(label), key: b.key})
		toolbar += buttonStyle.Render(label)
		x += len(label)
	}
	lines = append(lines, line(toolbar))
	if g.confirm != "" {
		lines = append(lines, line(g.confirm+": abandon? y/n"))
		y := len(lines)
		lines = append(lines, line("[Yes] [No]"))
		return strings.Join(lines, "\n"), []target{{x: 0, y: y, w: 5, key: "y"}, {x: 6, y: y, w: 4, key: "n"}}
	}
	if g.help {
		body, count := g.helpPage()
		start := min(g.helpOffset, max(0, len(body)-count))
		for _, l := range body[start:min(len(body), start+count)] {
			lines = append(lines, line(l))
		}
		y := len(lines)
		lines = append(lines, line("[Up] [Down] [Close]"))
		return strings.Join(lines, "\n"), []target{{x: 0, y: y, w: 4, key: "up"}, {x: 5, y: y, w: 6, key: "down"}, {x: 12, y: y, w: 7, key: "esc"}}
	}
	navigation := ""
	x = 0
	for _, b := range []struct{ label, key string }{{"<", "left"}, {">", "right"}, {"^", "up"}, {"v", "down"}} {
		label := "[" + b.label + "]"
		targets = append(targets, target{x: x, y: len(lines), w: 3, key: b.key})
		navigation += buttonStyle.Render(label)
		x += 3
	}
	lines = append(lines, line(navigation+fmt.Sprintf(" pile %d card %d", g.focus+1, g.row+1)))
	piles := g.table.Piles()
	fit := max(1, g.width/8)
	first := 0
	if g.focus >= fit {
		first = g.focus - fit + 1
	}
	last := min(len(piles), first+fit)
	rows := g.height - len(lines) - 4
	gridY := len(lines) + 1
	starts := make([]int, len(piles))
	for i, p := range piles {
		starts[i] = max(0, len(p.Cards)-rows)
		if i == g.focus {
			starts[i] = min(starts[i], g.row)
			if g.row >= starts[i]+rows {
				starts[i] = g.row - rows + 1
			}
		}
	}
	header := ""
	for i := first; i < last; i++ {
		name := piles[i].Name
		if i == g.focus {
			name = ">" + name
		}
		header += lipgloss.NewStyle().Width(8).MaxWidth(8).Render(name)
	}
	lines = append(lines, line(header))
	for y := 0; y < rows; y++ {
		row := ""
		for i := first; i < last; i++ {
			p := piles[i]
			index := starts[i] + y
			label := ""
			style := lipgloss.NewStyle().Width(7).MaxWidth(7).MaxHeight(1).Background(lipgloss.Color(bg)).Foreground(lipgloss.Color("#ffffff"))
			if index < len(p.Cards) {
				c := p.Cards[index]
				label = "│" + c.Label(g.opts.ASCII) + "│"
				fg := "#172235"
				if c.Red() {
					fg = "#9d1830"
				}
				style = style.Background(lipgloss.Color("#f4f1e5")).Foreground(lipgloss.Color(fg))
				if index < p.Hidden {
					label = "│╳╱╳│"
					if g.opts.ASCII {
						label = "|/X/|"
					}
					style = style.Background(lipgloss.Color("#245b9c")).Foreground(lipgloss.Color("#ffffff"))
				}
				if p.Marked[index] {
					label = "*" + label
				}
				label = p.Prefix[index] + label
				if i == g.selected && index == g.selectedRow {
					label = "+" + label
					style = style.Background(lipgloss.Color(accent)).Foreground(lipgloss.Color("#111111"))
				}
				targets = append(targets, target{x: (i - first) * 8, y: gridY + y, w: 7, pile: i, row: index})
			} else if y == 0 && len(p.Cards) == 0 {
				label = "[ -- ]"
				targets = append(targets, target{x: (i - first) * 8, y: gridY + y, w: 7, pile: i, row: 0})
			}
			if i == g.focus && index == g.row {
				label = ">" + label
				style = style.Bold(true).Underline(true)
			}
			row += style.Render(label) + felt.Render(" ")
		}
		lines = append(lines, line(row))
	}
	elapsed := time.Since(g.started)
	if !g.ended.IsZero() {
		elapsed = g.ended.Sub(g.started)
	}
	lines = append(lines, line(fmt.Sprintf("Time %s · piles %d–%d/%d", elapsed.Truncate(time.Second), first+1, last, len(piles))), line(g.note), line("←↑↓→ Tab Enter · ? help · Esc cancel"))
	return strings.Join(lines, "\n"), targets
}

func (g *Game) helpPage() ([]string, int) {
	text := cards.DealID(g.id, g.opts.Seed, g.opts.Rule) + "\n" + g.table.Status() + "\nArrows/Tab: focus; Enter/Space: select/place\nEsc: cancel; u/U: undo/redo; h: hint; a: auto\nn: fresh seed; r: identical replay; q: quit\nButtons: N new, R restart, Q quit, U undo, Y redo, H hint, A auto\n\n" + g.table.Help()
	body := strings.Split(lipgloss.NewStyle().Width(max(1, g.width-2)).Render(text), "\n")
	status := strings.Split(lipgloss.NewStyle().Width(max(1, g.width)).Render(g.table.Status()), "\n")
	return body, max(1, g.height-4-min(len(status), 4))
}
