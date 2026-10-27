package mines

import (
	"math/rand/v2"
	"time"

	tea "charm.land/bubbletea/v2"
)

type Level struct {
	Name  string
	W, H  int
	Mines int
}

var Levels = []Level{
	{"Beginner", 9, 9, 10},
	{"Intermediate", 16, 16, 40},
	{"Expert", 30, 16, 99},
}

// Game plays a Board with keys and mouse; it satisfies procmon.Viewer so Misa
// can show it as an in-process screen.
type Game struct {
	level          Level
	board          *Board
	cx, cy         int
	width, height  int
	started, ended time.Time
	gen            int
	quit           bool
	now            func() time.Time
	rng            *rand.Rand
}

type tickMsg struct{ gen int }

func New(level Level) *Game {
	g := &Game{now: time.Now, rng: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
	g.newGame(level)
	return g
}

func (g *Game) newGame(level Level) {
	g.level = level
	g.board = NewBoard(level.W, level.H, level.Mines, g.rng)
	g.cx, g.cy = level.W/2, level.H/2
	g.started, g.ended = time.Time{}, time.Time{}
	g.gen++
}

func (g *Game) Init() tea.Cmd      { return nil }
func (g *Game) Resize(w, h int)    { g.width, g.height = w, h }
func (g *Game) IsQuit() bool       { return g.quit }
func (g *Game) Board() *Board      { return g.board }
func (g *Game) Cursor() (x, y int) { return g.cx, g.cy }

func (g *Game) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.Resize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		return g.key(msg.String())
	case tea.MouseClickMsg:
		return g.click(msg.Mouse())
	case tickMsg:
		if msg.gen == g.gen && g.board.State() == Playing {
			return g.tick()
		}
	}
	return nil
}

func (g *Game) key(k string) tea.Cmd {
	switch k {
	case "q", "esc", "ctrl+c":
		g.quit = true
	case "up", "k":
		g.move(0, -1)
	case "down", "j":
		g.move(0, 1)
	case "left", "h":
		g.move(-1, 0)
	case "right", "l":
		g.move(1, 0)
	case "space", "enter":
		return g.open(g.cx, g.cy)
	case "f", "m":
		return g.act(func() { g.board.ToggleFlag(g.cx, g.cy) })
	case "n", "r", "f2":
		g.newGame(g.level)
	case "1", "2", "3":
		g.newGame(Levels[k[0]-'1'])
	}
	return nil
}

func (g *Game) click(m tea.Mouse) tea.Cmd {
	target, x, y := g.layout().hit(m.X, m.Y)
	switch {
	case target == hitFace && m.Button == tea.MouseLeft:
		g.newGame(g.level)
	case target != hitCell:
	case m.Button == tea.MouseLeft:
		g.cx, g.cy = x, y
		return g.open(x, y)
	case m.Button == tea.MouseRight:
		g.cx, g.cy = x, y
		return g.act(func() { g.board.ToggleFlag(x, y) })
	case m.Button == tea.MouseMiddle:
		g.cx, g.cy = x, y
		return g.act(func() { g.board.Chord(x, y) })
	}
	return nil
}

func (g *Game) move(dx, dy int) {
	g.cx = max(0, min(g.cx+dx, g.board.Width()-1))
	g.cy = max(0, min(g.cy+dy, g.board.Height()-1))
}

// open reveals a hidden cell, or chords a revealed number.
func (g *Game) open(x, y int) tea.Cmd {
	if g.board.Revealed(x, y) {
		return g.act(func() { g.board.Chord(x, y) })
	}
	return g.act(func() { g.board.Reveal(x, y) })
}

// act runs a move and keeps the clock: it starts with the first reveal and
// stops when the game ends.
func (g *Game) act(move func()) tea.Cmd {
	wasStarted := g.board.Started()
	move()
	if !wasStarted && g.board.Started() {
		g.started = g.now()
	}
	if g.board.State() != Playing {
		if g.ended.IsZero() {
			g.ended = g.now()
		}
		return nil
	}
	if !wasStarted && g.board.Started() {
		return g.tick()
	}
	return nil
}

func (g *Game) tick() tea.Cmd {
	gen := g.gen
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{gen} })
}

func (g *Game) Elapsed() time.Duration {
	switch {
	case g.started.IsZero():
		return 0
	case !g.ended.IsZero():
		return g.ended.Sub(g.started)
	}
	return g.now().Sub(g.started)
}
