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
	promo         []Move // promotion choices waiting for an answer
	flipped       bool
	letters       bool
	note          string
	width, height int
	quit          bool
	picker        *variantPicker

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
// the variant picker offers, including user variants.
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
	g.sel, g.promo, g.note = -1, nil, ""
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
		if g.picker != nil && len([]rune(msg.Text)) > 1 && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			g.pickerText(msg.Text)
		} else {
			g.key(msg.String())
		}
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
	if g.picker != nil {
		g.pickerKey(k)
		return
	}
	if g.promo != nil {
		g.promotionKey(k)
		return
	}
	g.note = ""
	switch k {
	case "q", "ctrl+c":
		g.quit = true
	case "esc":
		if g.sel >= 0 {
			g.sel = -1
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
		g.openVariantPicker()
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
	c := upper(k[0])
	v := g.match.Variant()
	for _, mv := range g.promo {
		if mv.Promote != 0 && v.Pieces[mv.Promote-1].Letter == c {
			g.play(mv)
			return
		}
	}
	// Staying a pawn has no piece type of its own, so the pawn's letter picks
	// it, just as the glyph it wears in the prompt suggests.
	if keep, ok := g.keepMove(); ok && g.moverLetter(keep.From) == c {
		g.play(keep)
	}
}

// keepMove reports the choice that leaves the pawn as it is, for variants
// where promotion is optional.
func (g *Game) keepMove() (Move, bool) {
	for _, mv := range g.promo {
		if mv.Promote == 0 {
			return mv, true
		}
	}
	return Move{}, false
}

// moverLetter is the letter of the piece on sq, or 0 for an empty square.
func (g *Game) moverLetter(sq int) byte {
	pc := g.match.Position().Board[sq]
	if pc.Type == 0 {
		return 0
	}
	return g.match.Variant().Type(pc).Letter
}

func (g *Game) click(m tea.Mouse) {
	if g.picker != nil {
		g.pickerClick(m)
		return
	}
	l := g.layout()
	if g.promo != nil {
		if i := l.promoChoice(g, m.X, m.Y); i >= 0 && m.Button == tea.MouseLeft {
			g.play(g.promo[i])
		} else if m.Button == tea.MouseRight {
			g.promo = nil
		}
		return
	}
	sq, ok := l.square(g, m.X, m.Y)
	switch {
	case !ok || g.computerToMove():
	case m.Button == tea.MouseLeft:
		g.note = ""
		g.cx, g.cy = g.match.Variant().XY(sq)
		g.activate(sq)
	case m.Button == tea.MouseRight:
		g.sel = -1
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
	if g.sel >= 0 {
		if moves := g.entryMoves(g.sel, sq); len(moves) > 0 {
			if choices := promotionChoices(moves); len(choices) > 1 {
				g.promo = choices
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

// entryMoves gives castling precedence over an ordinary move to the same
// square, and also accepts the castling rook as a destination.
func (g *Game) entryMoves(from, to int) []Move {
	p := g.match.Position()
	for _, mv := range g.movesBetween(from, -1) {
		if mv.Castle >= 0 && (mv.To == to || p.Castle(mv.Castle, p.Turn).Rook == to) {
			return []Move{mv}
		}
	}
	return g.movesBetween(from, to)
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

// promotionChoices reduces the moves between two squares to one per promotion
// result: a variant that lets a pawn stay a pawn lists the plain move alongside
// the promoted ones, and only the distinct outcomes are worth offering.
func promotionChoices(moves []Move) []Move {
	out := make([]Move, 0, len(moves))
	var seen [256]bool
	for _, mv := range moves {
		if seen[mv.Promote] {
			continue
		}
		seen[mv.Promote] = true
		out = append(out, mv)
	}
	// Staying a pawn comes first: it is the plain move, the others add a piece.
	if i := slices.IndexFunc(out, func(mv Move) bool { return mv.Promote == 0 }); i > 0 {
		keep := out[i]
		copy(out[1:i+1], out[:i])
		out[0] = keep
	}
	return out
}

func (g *Game) play(mv Move) {
	g.match.Play(mv)
	g.sel, g.promo, g.note = -1, nil, ""
}

// undo takes back moves until a human is to move, so against the computer it
// takes back its reply along with your move.
func (g *Game) undo() {
	g.sel, g.promo = -1, nil
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
	g.sel, g.promo = -1, nil
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

func (g *Game) setVariant(i int) {
	if i >= 0 && i < len(g.variants) && g.variants[i] != g.match.Variant() {
		g.newGame(g.variants[i])
	}
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}
	return c
}
