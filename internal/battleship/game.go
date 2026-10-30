package battleship

import (
	"math/rand/v2"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Options configures a new game.
type Options struct {
	// Random lays the player's fleet out at random and starts at the firing
	// phase, as `battleship -r` does.
	Random bool
}

// Phase is the part of a game being played.
type Phase int

const (
	Placing Phase = iota // the player is laying out a fleet
	Firing               // shots are traded with the computer
	Over                 // the game is decided
)

const (
	// replyDelay keeps the player's shot on screen before the computer answers.
	replyDelay = 450 * time.Millisecond
	// clockTick refreshes the elapsed seconds in the header.
	clockTick = time.Second
)

// tickMsg is the game's timer: it brings the computer's reply after a shot and
// refreshes the clock between shots. gen drops a timer left over from a game
// that has since been replaced, as mines does.
type tickMsg struct{ gen int }

// Game plays one single-player game: the player places a fleet, then trades
// shots with the computer. It satisfies procmon.Viewer, so Misa can show it as
// an in-process screen.
type Game struct {
	opts          Options
	player, enemy *Board
	ai            *AI
	phase         Phase
	state         State
	cx, cy        int
	horizontal    bool
	shots         int
	note          string
	pending       bool // the computer's reply is on its way

	width, height  int
	started, ended time.Time
	gen            int
	quit           bool
	now            func() time.Time
	rng            *rand.Rand
}

// New deals a game: the player's fleet is placed by hand unless opts.Random,
// and the computer's is always random.
func New(opts Options) *Game {
	g := &Game{now: time.Now, rng: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
	g.newGame(opts)
	return g
}

// newGame deals a fresh game with the same options. Bumping gen drops any
// reply or clock tick still on its way to the old one.
func (g *Game) newGame(opts Options) {
	g.opts = opts
	g.player = NewBoard(Size, Fleet, g.rng)
	g.enemy = NewBoard(Size, Fleet, g.rng)
	g.enemy.Randomize()
	g.ai = NewAI(g.rng)
	g.state = Playing
	g.phase = Placing
	g.cx, g.cy = 0, 0
	g.horizontal = true
	g.shots = 0
	g.note = ""
	g.pending = false
	g.started, g.ended = time.Time{}, time.Time{}
	g.gen++
	if opts.Random {
		g.player.Randomize()
		g.toFiring()
	}
}

// toFiring moves to the shooting phase with the cursor on the enemy board.
func (g *Game) toFiring() {
	g.phase = Firing
	g.cx, g.cy = Size/2, Size/2
}

func (g *Game) Init() tea.Cmd      { return nil }
func (g *Game) Resize(w, h int)    { g.width, g.height = w, h }
func (g *Game) IsQuit() bool       { return g.quit }
func (g *Game) Player() *Board     { return g.player }
func (g *Game) Enemy() *Board      { return g.enemy }
func (g *Game) Phase() Phase       { return g.phase }
func (g *Game) State() State       { return g.state }
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
		return g.tick(msg)
	}
	return nil
}

func (g *Game) key(k string) tea.Cmd {
	if k != "space" && k != "enter" {
		g.note = ""
	}
	switch k {
	case "q", "esc", "ctrl+c":
		g.quit = true
	case "n", "f2":
		g.newGame(g.opts)
	case "up", "k":
		g.move(0, -1)
	case "down", "j":
		g.move(0, 1)
	case "left", "h":
		g.move(-1, 0)
	case "right", "l":
		g.move(1, 0)
	case "r":
		g.horizontal = !g.horizontal
	case "f":
		return g.randomFleet()
	case "space", "enter":
		return g.act(g.cx, g.cy)
	}
	return nil
}

func (g *Game) click(m tea.Mouse) tea.Cmd {
	if m.Button != tea.MouseLeft {
		return nil
	}
	target, x, y := g.layout().hit(m.X, m.Y)
	switch {
	case target == hitPlayer && g.phase == Placing:
		g.cx, g.cy = x, y
		return g.act(x, y)
	case target == hitEnemy && g.phase == Firing:
		g.cx, g.cy = x, y
		return g.act(x, y)
	}
	return nil
}

func (g *Game) move(dx, dy int) {
	g.cx = max(0, min(g.cx+dx, Size-1))
	g.cy = max(0, min(g.cy+dy, Size-1))
}

// act does whatever the cursor's board asks for in the current phase.
func (g *Game) act(x, y int) tea.Cmd {
	switch g.phase {
	case Placing:
		return g.place(x, y)
	case Firing:
		return g.fire(x, y)
	}
	return nil
}

// place lays the next pending ship with its bow under the cursor.
func (g *Game) place(x, y int) tea.Cmd {
	pending := g.player.Pending()
	if len(pending) == 0 {
		g.toFiring()
		return g.settle()
	}
	spec := pending[0]
	if !g.player.Place(spec.Len, x, y, g.horizontal) {
		g.note = spec.Name + " doesn't fit there"
		return nil
	}
	g.note = ""
	if len(g.player.Pending()) == 0 {
		g.toFiring()
	}
	return g.settle()
}

// randomFleet lays the player's whole fleet out at random and starts firing,
// the same as `battleship -r`.
func (g *Game) randomFleet() tea.Cmd {
	if g.phase != Placing {
		return nil
	}
	g.player.Randomize()
	g.note = ""
	g.toFiring()
	return g.settle()
}

// fire shoots at the enemy board. A fresh shot leaves the computer a reply on a
// timer; a repeated one only sets a note.
func (g *Game) fire(x, y int) tea.Cmd {
	if g.state != Playing || g.phase != Firing {
		return nil
	}
	if g.pending {
		g.note = "enemy fires…"
		return nil
	}
	shot := g.enemy.Fire(x, y)
	if shot == Unfired {
		g.note = "already fired there"
		return nil
	}
	g.shots++
	g.note = g.shotNote(x, y, shot)
	// settle's clock tick is superseded by the reply timer, which re-arms it.
	g.settle()
	if g.phase == Over {
		return nil
	}
	g.pending = true
	gen := g.gen
	return tea.Tick(replyDelay, func(time.Time) tea.Msg { return tickMsg{gen} })
}

// shotNote says what the player's shot did.
func (g *Game) shotNote(x, y int, shot Shot) string {
	if shot == Miss {
		return "miss"
	}
	if ship := g.enemy.ShipAt(x, y); ship != nil && ship.Sunk() {
		return "you sank the " + ship.Name + "!"
	}
	return "hit!"
}

// tick runs a timer that belongs to the game in play: the computer's reply when
// a shot is waiting for one, and the next clock refresh either way.
func (g *Game) tick(msg tickMsg) tea.Cmd {
	if msg.gen != g.gen {
		return nil
	}
	if g.pending {
		g.reply()
	}
	return g.clock()
}

// reply lets the computer shoot back at the player's fleet, once per shot.
func (g *Game) reply() {
	g.pending = false
	if g.state != Playing || g.phase != Firing {
		return
	}
	p, ok := g.ai.Fire(g.player)
	if !ok {
		return
	}
	g.note = g.replyNote(p)
	g.settle()
}

// replyNote says what the computer's shot did.
func (g *Game) replyNote(p Point) string {
	ship := g.player.ShipAt(p.X, p.Y)
	switch {
	case ship == nil:
		return "the computer missed"
	case ship.Sunk():
		return "the computer sank your " + ship.Name
	}
	return "the computer hit your " + ship.Name
}

// settle starts the clock, ends it when a fleet has gone down, and keeps it
// ticking while play goes on.
func (g *Game) settle() tea.Cmd {
	g.startClock()
	if g.decided() || g.pending {
		return nil
	}
	return g.clock()
}

// decided records the ending when one fleet is at the bottom.
func (g *Game) decided() bool {
	switch {
	case g.enemy.AllSunk():
		g.state = Won
	case g.player.AllSunk():
		g.state = Lost
	}
	if g.state == Playing {
		return false
	}
	g.phase = Over
	if g.ended.IsZero() {
		g.ended = g.now()
	}
	return true
}

func (g *Game) startClock() {
	if g.started.IsZero() {
		g.started = g.now()
	}
}

// clock asks for the next clock refresh while the game is on.
func (g *Game) clock() tea.Cmd {
	if g.started.IsZero() || g.state != Playing {
		return nil
	}
	gen := g.gen
	return tea.Tick(clockTick, func(time.Time) tea.Msg { return tickMsg{gen} })
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
