package chess

import (
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

const DefaultThinkTime = 3 * time.Second

// Game plays a Match at one keyboard, either side optionally played by the
// computer; it satisfies procmon.Viewer so Misa can show it as an in-process
// screen.
type Game struct {
	variants      []*Variant
	match         *Match
	cx, cy        int    // cursor square as file and rank
	sel           int    // selected square, or -1
	drop          uint8  // selected pocket type, or 0
	promo         []Move // promotion choices waiting for an answer
	flipped       bool
	letters       bool
	note          string
	width, height int
	quit          bool

	computer  [2]bool // which sides the computer plays
	level     Level
	seed      uint64
	thinkTime time.Duration
	engines   map[*Variant]*Engine
	thinking  bool
	gen       int // bumped to drop the result of an abandoned search
	stop      *atomic.Bool
}

// thoughtMsg carries a finished search back to the game.
type thoughtMsg struct {
	gen    int
	result Result
}

// New starts a game of v, the computer playing Black; variants are the ones
// v cycles through.
func New(variants []*Variant, v *Variant) *Game {
	g := &Game{variants: variants, seed: uint64(time.Now().UnixNano()), engines: map[*Variant]*Engine{}}
	g.SetComputer(false, true)
	g.newGame(v)
	return g
}

// SetComputer picks the sides the computer plays; the board turns so a lone
// human player sits at the bottom.
func (g *Game) SetComputer(white, black bool) {
	g.cancel()
	g.computer = [2]bool{white, black}
	g.sel, g.drop, g.promo = -1, 0, nil
	g.flipped = white && !black
}

func (g *Game) SetThinkTime(d time.Duration) {
	g.cancel()
	g.thinkTime = d
}

func (g *Game) Level() Level { return g.level }

// SetLevel cancels obsolete work. The next Init or Update starts the new
// search, just as for SetComputer. Hotseat retains the selected level.
func (g *Game) SetLevel(level Level) {
	g.cancel()
	g.level = level
}

func (g *Game) SetSeed(seed uint64) {
	g.cancel()
	g.seed = seed
}

// Load replaces the starting position, e.g. one read with ParseFEN.
func (g *Game) Load(p *Position) {
	g.cancel()
	g.match.Load(p)
	g.reset()
}

func (g *Game) newGame(v *Variant) {
	g.cancel()
	g.match = NewMatch(v)
	g.reset()
	g.note = v.Description
}

func (g *Game) reset() {
	v := g.match.Variant()
	g.cx, g.cy = (v.Files-1)/2, min(1, v.Ranks-1)
	if g.match.Turn() == Black {
		g.cy = max(0, v.Ranks-2)
	}
	g.sel, g.drop, g.promo, g.note = -1, 0, nil, ""
}

func (g *Game) Init() tea.Cmd   { return g.think() }
func (g *Game) Resize(w, h int) { g.width, g.height = w, h }
func (g *Game) IsQuit() bool    { return g.quit }
func (g *Game) Match() *Match   { return g.match }

func (g *Game) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.Resize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		k := msg.String()
		// Text may omit modifiers in String; modified printable text must not
		// accidentally select a pocket (or trigger a plain shortcut).
		if msg.Mod&^tea.ModShift != 0 && len(k) == 1 {
			break
		}
		g.key(k)
	case tea.MouseClickMsg:
		g.click(msg.Mouse())
	case thoughtMsg:
		g.thought(msg)
	}
	if g.quit {
		g.cancel()
		return nil
	}
	return g.think()
}

func (g *Game) key(k string) {
	if g.match.Variant().Drops && len(k) == len("shift+n") && k[:6] == "shift+" && k[6] >= 'a' && k[6] <= 'z' {
		k = string(upper(k[6]))
	}
	if g.promo != nil {
		g.promotionKey(k)
		return
	}
	g.note = ""
	if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' && g.selectDropLetter(k[0]) {
		return
	}
	switch k {
	case "q", "ctrl+c":
		g.quit = true
	case "esc":
		if g.sel >= 0 || g.drop != 0 {
			g.sel, g.drop = -1, 0
		} else {
			g.quit = true
		}
	case "up", "k":
		g.moveCursor(0, -1)
	case "down", "j":
		g.moveCursor(0, 1)
	case "left", "h":
		g.moveCursor(-1, 0)
	case "right", "l":
		g.moveCursor(1, 0)
	case "space", "enter":
		g.activate(g.match.Variant().Square(g.cx, g.cy))
	case "u", "backspace":
		g.undo()
	case "c":
		g.cycleComputer()
	case "d":
		g.SetLevel(g.level.next())
		g.note = "Computer level: " + g.level.String()
	case "f":
		g.flipped = !g.flipped
	case "t":
		g.letters = !g.letters
	case "n":
		g.newGame(g.match.Variant())
	case "v", "V":
		g.cycleVariant(map[string]int{"v": 1, "V": -1}[k])
	}
}

func (g *Game) promotionKey(k string) {
	if k == "esc" {
		g.promo = nil
		return
	}
	if len(k) != 1 {
		return
	}
	for _, mv := range g.promo {
		if g.match.Variant().Pieces[mv.Promote-1].Letter == upper(k[0]) {
			g.play(mv)
			return
		}
	}
}

func (g *Game) click(m tea.Mouse) {
	l := g.layout()
	if g.promo != nil {
		if i := l.promoChoice(g, m.X, m.Y); i >= 0 && m.Button == tea.MouseLeft {
			g.play(g.promo[i])
		} else if m.Button == tea.MouseRight {
			g.promo = nil
		}
		return
	}
	if m.Button == tea.MouseRight {
		g.sel, g.drop = -1, 0
		return
	}
	if m.Button == tea.MouseLeft {
		if typ := l.pocketChoice(g, m.X, m.Y); typ != 0 {
			g.selectDrop(typ)
			return
		}
	}
	sq, ok := l.square(g, m.X, m.Y)
	switch {
	case !ok || g.computerToMove():
	case m.Button == tea.MouseLeft:
		g.note = ""
		g.cx, g.cy = g.match.Variant().XY(sq)
		g.activate(sq)
	}
}

// moveCursor moves by screen direction, so up stays up on a flipped board.
func (g *Game) moveCursor(dx, dy int) {
	v := g.match.Variant()
	if !g.flipped {
		dy = -dy
	} else {
		dx = -dx
	}
	g.cx = max(0, min(g.cx+dx, v.Files-1))
	g.cy = max(0, min(g.cy+dy, v.Ranks-1))
}

// activate selects the piece on sq, or moves the selected piece there.
func (g *Game) activate(sq int) {
	if g.match.Over() || g.computerToMove() {
		return
	}
	if g.drop != 0 {
		for _, mv := range g.match.Legal() {
			if mv.Drop == g.drop && mv.To == sq {
				g.play(mv)
				return
			}
		}
		return // illegal destinations leave the pocket selection intact
	}
	if g.sel >= 0 {
		if moves := g.movesBetween(g.sel, sq); len(moves) > 0 {
			if len(moves) > 1 && moves[0].Promote != 0 {
				g.promo = moves
				return
			}
			g.play(moves[0])
			return
		}
	}
	if sq != g.sel && len(g.movesBetween(sq, -1)) > 0 {
		g.sel = sq
		return
	}
	g.sel = -1
}

// selectDrop selects an available type in the side-to-move's pocket.
func (g *Game) selectDrop(typ uint8) bool {
	p := g.match.Position()
	if !p.V.Drops || g.match.Over() || g.computerToMove() || int(typ) >= len(p.Hands[p.Turn]) || p.Hands[p.Turn][typ] <= 0 {
		return false
	}
	g.sel, g.drop, g.note = -1, typ, ""
	return true
}

func (g *Game) selectDropLetter(letter byte) bool {
	for i, pt := range g.match.Variant().Pieces {
		if pt.Letter == letter {
			return g.selectDrop(uint8(i + 1))
		}
	}
	return false
}

// movesBetween lists the legal moves from one square to another, or to
// anywhere when to is -1.
func (g *Game) movesBetween(from, to int) []Move {
	var out []Move
	for _, mv := range g.match.Legal() {
		if mv.From == from && (to < 0 || mv.To == to) {
			out = append(out, mv)
		}
	}
	return out
}

func (g *Game) play(mv Move) {
	g.match.Play(mv)
	g.sel, g.drop, g.promo, g.note = -1, 0, nil, ""
}

// undo takes back moves until a human is to move, so against the computer it
// takes back its reply along with your move.
func (g *Game) undo() {
	g.sel, g.drop, g.promo = -1, 0, nil
	played := len(g.match.Played())
	plies := 0
	for plies < played {
		plies++
		if turn := g.match.Turn() ^ Color(plies%2); !g.computer[turn] || g.computer[turn.Other()] {
			break
		}
	}
	if turn := g.match.Turn() ^ Color(plies%2); plies == 0 || (g.computer[turn] && !g.computer[turn.Other()]) {
		g.note = "Nothing to take back"
		return
	}
	g.cancel()
	for range plies {
		g.match.Undo()
	}
}

// cycleComputer steps through: computer plays Black, White, both, neither.
func (g *Game) cycleComputer() {
	modes := [][2]bool{{false, true}, {true, false}, {true, true}, {false, false}}
	i := slices.Index(modes, g.computer)
	next := modes[(i+1)%len(modes)]
	g.SetComputer(next[0], next[1])
	g.sel, g.drop, g.promo = -1, 0, nil
	g.note = "Computer plays " + g.opponent()
}

func (g *Game) opponent() string {
	switch g.computer {
	case [2]bool{true, true}:
		return "both sides"
	case [2]bool{true, false}:
		return "White"
	case [2]bool{false, true}:
		return "Black"
	}
	return "neither side: two players"
}

func (g *Game) computerToMove() bool { return g.computer[g.match.Turn()] }

func (g *Game) engine() *Engine {
	v := g.match.Variant()
	if g.engines[v] == nil {
		g.engines[v] = NewEngine(v)
	}
	return g.engines[v]
}

// think starts a search when the computer is to move and none is running.
// It runs as a command, off the UI goroutine.
func (g *Game) think() tea.Cmd {
	if g.thinking || g.quit || g.match.Over() || !g.computerToMove() {
		return nil
	}
	g.thinking = true
	g.gen++
	g.stop = &atomic.Bool{}
	gen, stop, engine := g.gen, g.stop, g.engine()
	root, past := g.match.Position(), g.match.Hashes()
	level, seed := g.level, g.seed^root.Hash()^uint64(len(g.match.Played()))
	limits := level.Limits(g.thinkTime)
	limits.Stop = stop
	return func() tea.Msg {
		return thoughtMsg{gen: gen, result: engine.ThinkLevel(root, past, level, limits, seed)}
	}
}

// cancel abandons a running search; its result will be ignored.
func (g *Game) cancel() {
	if g.stop != nil {
		g.stop.Store(true)
	}
	g.thinking = false
	g.gen++
}

func (g *Game) thought(msg thoughtMsg) {
	if msg.gen != g.gen {
		return
	}
	g.thinking = false
	r := msg.result
	if !r.Found || !slices.Contains(g.match.Legal(), r.Move) {
		return
	}
	mover := g.match.Turn()
	g.play(r.Move)
	if r.Depth == 0 {
		g.note = "Computer played " + g.match.Notation()[len(g.match.Notation())-1] + " · search stopped before analysis"
		return
	}
	g.note = fmt.Sprintf("Computer played %s · depth %d · %s", g.match.Notation()[len(g.match.Notation())-1], r.Depth, scoreText(r.Score, mover))
}

// scoreText shows a search score from White's side: +1.25, or who mates.
func scoreText(score int, mover Color) string {
	if IsMateScore(score) {
		n := MateIn(score)
		winner := mover
		if n < 0 {
			winner, n = mover.Other(), -n
		}
		return fmt.Sprintf("%s mates in %d", winner, n)
	}
	if mover == Black {
		score = -score
	}
	return fmt.Sprintf("%+.2f", float64(score)/100)
}

func (g *Game) cycleVariant(step int) {
	i := slices.Index(g.variants, g.match.Variant())
	if len(g.variants) == 0 {
		return
	}
	i = ((i+step)%len(g.variants) + len(g.variants)) % len(g.variants)
	g.newGame(g.variants[i])
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}
