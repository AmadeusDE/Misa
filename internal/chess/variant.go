package chess

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"

	"github.com/AmadeusDE/Misa/internal/hostpath"
)

//go:embed variants
var builtinFS embed.FS

var builtinFiles = []string{"chess.yaml", "double.yaml", "bigboard.yaml", "doublebig.yaml", "amazon.yaml", "nocastling.yaml", "capablanca.yaml", "berolina.yaml", "stalematewins.yaml", "selfcapture.yaml"}

// PieceType is one kind of piece a variant uses. Type numbers in Piece index
// Variant.Pieces from 1, so the zero Piece is an empty square.
type PieceType struct {
	Letter byte // upper case; White's pieces print in upper case, Black's in lower
	Name   string
	Betza  string
	Glyph  string // one terminal cell; the letter is used when empty
	Royal  bool   // may not be left attacked, and castles
	Pawn   bool   // promotes, takes and is taken en passant, resets the move clock
	Value  int    // centipawns for the computer player; estimated when the file leaves it out

	moves       []Component
	tracksMoved bool
	reach       [2]reachTable
}

// Castle is one castling move, given by White's squares; Black's mirror them
// across the middle rank.
type Castle struct{ King, Rook, KingTo, RookTo int }

type Variant struct {
	ID, Name, Description string
	Files, Ranks          int
	SelfCapture           bool
	Pieces                []*PieceType
	Castling              []Castle
	Promotion             []uint8 // piece types a pawn may become
	PromotionZone         int     // the last n ranks promote
	MoveLimit             int     // moves by each side without a capture or pawn move before a draw
	Stalemate             StalemateRule
	Setup                 string
	Source                string

	start   []Piece
	zobrist zobrist
}

type pieceDef struct {
	Letter string `yaml:"letter"`
	Name   string `yaml:"name"`
	Betza  string `yaml:"betza"`
	Glyph  string `yaml:"glyph"`
	Royal  bool   `yaml:"royal"`
	Pawn   bool   `yaml:"pawn"`
	Value  int    `yaml:"value"`
}

type castleDef struct {
	King   string `yaml:"king"`
	Rook   string `yaml:"rook"`
	KingTo string `yaml:"king_to"`
	RookTo string `yaml:"rook_to"`
}

// StalemateRule is what happens to a side with no legal move that is not in
// check, from the point of view of the side that delivered the stalemate.
type StalemateRule string

const (
	StalemateDraw StalemateRule = "draw"
	StalemateWin  StalemateRule = "win"  // the side delivering stalemate wins
	StalemateLoss StalemateRule = "loss" // the side delivering stalemate loses
)

// variantDef is a variant file. With extends, unset fields come from the named
// variant and pieces add to or replace (by letter) the parent's.
type variantDef struct {
	ID            string        `yaml:"id"`
	Name          string        `yaml:"name"`
	Description   string        `yaml:"description"`
	Extends       string        `yaml:"extends"`
	Files         int           `yaml:"files"`
	Ranks         int           `yaml:"ranks"`
	Pieces        []pieceDef    `yaml:"pieces"`
	Setup         string        `yaml:"setup"`
	Castling      []castleDef   `yaml:"castling"`
	Promotion     []string      `yaml:"promotion"`
	PromotionZone int           `yaml:"promotion_zone"`
	MoveLimit     int           `yaml:"move_limit"`
	Stalemate     StalemateRule `yaml:"stalemate"`
	SelfCapture   *bool         `yaml:"self_capture"`

	source string
}

type source struct {
	name string
	data []byte
}

// Builtins returns the variants compiled into Misa.
func Builtins() ([]*Variant, []error) {
	return load(builtinSources())
}

// Load returns the built-in variants plus those in chess/*.yaml next to the
// executable and in ~/.config/misa/chess/. A file whose id matches an earlier
// variant replaces it, and may extend the one it replaces.
func Load() ([]*Variant, []error) {
	return load(append(builtinSources(), externalSources()...))
}

// LoadFile adds one variant file to the built-in and external variants and
// returns the variant it defines.
func LoadFile(path string) (*Variant, []*Variant, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var def variantDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, nil, fmt.Errorf("%s: %v", path, err)
	}
	vs, errs := load(append(append(builtinSources(), externalSources()...), source{path, data}))
	for _, err := range errs {
		if strings.HasPrefix(err.Error(), path+":") {
			return nil, vs, err
		}
	}
	v, err := Find(vs, def.ID)
	return v, vs, err
}

func builtinSources() []source {
	var srcs []source
	for _, name := range builtinFiles {
		data, err := builtinFS.ReadFile("variants/" + name)
		if err != nil {
			panic(err)
		}
		srcs = append(srcs, source{"builtin:" + name, data})
	}
	return srcs
}

func externalSources() []source {
	var srcs []source
	for _, dir := range searchDirs("chess") {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if data, err := os.ReadFile(path); err == nil {
				srcs = append(srcs, source{path, data})
			}
		}
	}
	return srcs
}

func searchDirs(subdir string) []string {
	return hostpath.SearchDirs(subdir)
}

// parseVariants loads the built-in variants plus extra files, for tests.
func parseVariants(extra map[string][]byte) ([]*Variant, error) {
	names := make([]string, 0, len(extra))
	for name := range extra {
		names = append(names, name)
	}
	sort.Strings(names)
	srcs := builtinSources()
	for _, name := range names {
		srcs = append(srcs, source{name, extra[name]})
	}
	vs, errs := load(srcs)
	return vs, errors.Join(errs...)
}

func load(srcs []source) ([]*Variant, []error) {
	var errs []error
	var defs []variantDef
	for _, src := range srcs {
		var d variantDef
		if err := yaml.Unmarshal(src.data, &d); err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", src.name, err))
			continue
		}
		if d.ID == "" {
			errs = append(errs, fmt.Errorf("%s: missing id", src.name))
			continue
		}
		d.source = src.name
		defs = append(defs, d)
	}

	built := make([]*Variant, len(defs))
	resolved := make([]*variantDef, len(defs))
	for i, d := range defs {
		if d.Extends != "" {
			parent := -1
			for j := i - 1; j >= 0; j-- {
				if resolved[j] != nil && strings.EqualFold(defs[j].ID, d.Extends) {
					parent = j
					break
				}
			}
			if parent < 0 {
				errs = append(errs, fmt.Errorf("%s: extends unknown variant %q", d.source, d.Extends))
				continue
			}
			d = merge(*resolved[parent], d)
		}
		resolved[i] = &d
		v, err := build(d)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", d.source, err))
			resolved[i] = nil
			continue
		}
		built[i] = v
	}

	var out []*Variant
	index := map[string]int{}
	for _, v := range built {
		if v == nil {
			continue
		}
		key := strings.ToLower(v.ID)
		if i, ok := index[key]; ok {
			out[i] = v
			continue
		}
		index[key] = len(out)
		out = append(out, v)
	}
	return out, errs
}

func merge(parent, child variantDef) variantDef {
	d := parent
	d.ID, d.Name, d.Description, d.source = child.ID, child.Name, child.Description, child.source
	if child.Files != 0 {
		d.Files = child.Files
	}
	if child.Ranks != 0 {
		d.Ranks = child.Ranks
	}
	if child.Setup != "" {
		d.Setup = child.Setup
	}
	if child.Castling != nil {
		d.Castling = child.Castling
	}
	if child.Promotion != nil {
		d.Promotion = child.Promotion
	}
	if child.PromotionZone != 0 {
		d.PromotionZone = child.PromotionZone
	}
	if child.MoveLimit != 0 {
		d.MoveLimit = child.MoveLimit
	}
	if child.Stalemate != "" {
		d.Stalemate = child.Stalemate
	}
	if child.SelfCapture != nil {
		d.SelfCapture = child.SelfCapture
	}
	d.Pieces = slices.Clone(parent.Pieces)
	for _, p := range child.Pieces {
		i := slices.IndexFunc(d.Pieces, func(q pieceDef) bool { return strings.EqualFold(q.Letter, p.Letter) })
		if i >= 0 {
			d.Pieces[i] = p
		} else {
			d.Pieces = append(d.Pieces, p)
		}
	}
	return d
}

func build(d variantDef) (*Variant, error) {
	v := &Variant{
		ID: d.ID, Name: d.Name, Description: d.Description,
		Files: d.Files, Ranks: d.Ranks, Setup: d.Setup, Source: d.source,
		SelfCapture:   d.SelfCapture != nil && *d.SelfCapture,
		PromotionZone: d.PromotionZone, MoveLimit: d.MoveLimit, Stalemate: d.Stalemate,
	}
	if v.Name == "" {
		v.Name = v.ID
	}
	if v.PromotionZone == 0 {
		v.PromotionZone = 1
	}
	if v.MoveLimit == 0 {
		v.MoveLimit = 50
	}
	if v.Stalemate == "" {
		v.Stalemate = StalemateDraw
	}
	if v.Stalemate != StalemateDraw && v.Stalemate != StalemateWin && v.Stalemate != StalemateLoss {
		return nil, fmt.Errorf("stalemate must be draw, win, or loss")
	}
	// Two-letter file labels end at zz (702); large boards also incur substantial
	// reach-table, move generation, and search costs, so keep dimensions bounded.
	if v.Files < 1 || v.Files > 702 || v.Ranks < 1 || v.Ranks > 99 {
		return nil, fmt.Errorf("board must be 1-702 files by 1-99 ranks, not %d×%d", v.Files, v.Ranks)
	}
	if len(d.Pieces) == 0 || len(d.Pieces) > 255 {
		return nil, fmt.Errorf("needs 1-255 piece types")
	}
	for _, p := range d.Pieces {
		t, err := buildPiece(p)
		if err != nil {
			return nil, err
		}
		if v.TypeOf(t.Letter) != 0 {
			return nil, fmt.Errorf("piece letter %c used twice", t.Letter)
		}
		v.Pieces = append(v.Pieces, t)
	}
	start, err := v.parsePlacement(d.Setup)
	if err != nil {
		return nil, fmt.Errorf("setup: %v", err)
	}
	v.start = start
	for _, c := range d.Castling {
		castle, err := v.parseCastle(c)
		if err != nil {
			return nil, fmt.Errorf("castling: %v", err)
		}
		v.Castling = append(v.Castling, castle)
	}
	if err := v.setPromotion(d.Promotion); err != nil {
		return nil, err
	}
	v.markTrackedPieces()
	v.buildReach()
	v.estimateValues()
	v.zobrist = newZobrist(v)
	return v, nil
}

func buildPiece(p pieceDef) (*PieceType, error) {
	if len(p.Letter) != 1 || !isLetter(p.Letter[0]) {
		return nil, fmt.Errorf("piece letter %q must be one letter", p.Letter)
	}
	t := &PieceType{
		Letter: strings.ToUpper(p.Letter)[0], Name: p.Name, Betza: p.Betza,
		Glyph: p.Glyph, Royal: p.Royal, Pawn: p.Pawn, Value: p.Value,
	}
	if t.Name == "" {
		t.Name = string(t.Letter)
	}
	moves, err := ParseBetza(p.Betza)
	if err != nil {
		return nil, fmt.Errorf("piece %c: %v", t.Letter, err)
	}
	t.moves = moves
	if t.Glyph != "" && lipgloss.Width(t.Glyph) != 1 {
		return nil, fmt.Errorf("piece %c: glyph %q must be one cell wide", t.Letter, t.Glyph)
	}
	return t, nil
}

func (v *Variant) parseCastle(c castleDef) (Castle, error) {
	var sq [4]int
	for i, name := range []string{c.King, c.Rook, c.KingTo, c.RookTo} {
		s, err := v.ParseSquare(name)
		if err != nil {
			return Castle{}, err
		}
		sq[i] = s
	}
	rank := sq[0] / v.Files
	for _, s := range sq {
		if s/v.Files != rank {
			return Castle{}, fmt.Errorf("%s/%s: all four squares must share a rank", c.King, c.Rook)
		}
	}
	if sq[0] == sq[1] {
		return Castle{}, fmt.Errorf("king and rook on the same square %s", c.King)
	}
	return Castle{sq[0], sq[1], sq[2], sq[3]}, nil
}

func (v *Variant) setPromotion(letters []string) error {
	if letters == nil {
		for i, t := range v.Pieces {
			if !t.Royal && !t.Pawn {
				v.Promotion = append(v.Promotion, uint8(i+1))
			}
		}
		return nil
	}
	for _, l := range letters {
		if len(l) != 1 || v.TypeOf(l[0]) == 0 {
			return fmt.Errorf("promotion: unknown piece %q", l)
		}
		v.Promotion = append(v.Promotion, v.TypeOf(l[0]))
	}
	return nil
}

// markTrackedPieces notes which types need their moved flag kept when telling
// positions apart: pieces with first-move-only moves, and castling pieces.
func (v *Variant) markTrackedPieces() {
	for _, t := range v.Pieces {
		t.tracksMoved = t.Royal || slices.ContainsFunc(t.moves, func(c Component) bool { return c.Initial })
	}
	for _, c := range v.Castling {
		for _, color := range []Color{White, Black} {
			if p := v.start[v.orient(c.Rook, color)]; p.Type != 0 {
				v.Type(p).tracksMoved = true
			}
		}
	}
}

// TypeOf returns the type number for a piece letter in either case, or 0.
func (v *Variant) TypeOf(letter byte) uint8 {
	up := strings.ToUpper(string(letter))[0]
	for i, t := range v.Pieces {
		if t.Letter == up {
			return uint8(i + 1)
		}
	}
	return 0
}

func (v *Variant) Type(p Piece) *PieceType { return v.Pieces[p.Type-1] }

func (v *Variant) Size() int { return v.Files * v.Ranks }

func (v *Variant) XY(sq int) (x, y int) { return sq % v.Files, sq / v.Files }

func (v *Variant) Square(x, y int) int { return y*v.Files + x }

func (v *Variant) OnBoard(x, y int) bool { return x >= 0 && y >= 0 && x < v.Files && y < v.Ranks }

func (v *Variant) SquareName(sq int) string { return formatSquare(v, sq) }

func (v *Variant) ParseSquare(s string) (int, error) { return parseSquare(v, s) }

// orient maps one of White's squares to the given side: Black's mirror image.
func (v *Variant) orient(sq int, c Color) int {
	if c == White {
		return sq
	}
	x, y := v.XY(sq)
	return v.Square(x, v.Ranks-1-y)
}

// parsePlacement reads FEN-style ranks, top rank first: letters are pieces
// (upper case White), numbers of any length are runs of empty squares.
func (v *Variant) parsePlacement(s string) ([]Piece, error) {
	rows := strings.Split(s, "/")
	if len(rows) != v.Ranks {
		return nil, fmt.Errorf("%d ranks, want %d", len(rows), v.Ranks)
	}
	board := make([]Piece, v.Size())
	for i, row := range rows {
		y, x := v.Ranks-1-i, 0
		for j := 0; j < len(row); {
			c := row[j]
			switch {
			case isDigit(c):
				k := j
				for k < len(row) && isDigit(row[k]) {
					k++
				}
				n, _ := strconv.Atoi(row[j:k])
				x, j = x+n, k
				continue
			case isLetter(c):
				t := v.TypeOf(c)
				if t == 0 {
					return nil, fmt.Errorf("unknown piece %c", c)
				}
				if x < v.Files {
					color := White
					if c >= 'a' {
						color = Black
					}
					board[v.Square(x, y)] = Piece{Type: t, Color: color}
				}
				x++
			default:
				return nil, fmt.Errorf("unexpected %q", c)
			}
			j++
		}
		if x != v.Files {
			return nil, fmt.Errorf("rank %d has %d squares, want %d", y+1, x, v.Files)
		}
	}
	return board, nil
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// Find picks a variant by id or name, ignoring case, or by a prefix of either
// that matches only one variant.
func Find(vs []*Variant, name string) (*Variant, error) {
	name = strings.ToLower(name)
	for _, v := range vs {
		if strings.ToLower(v.ID) == name || strings.ToLower(v.Name) == name {
			return v, nil
		}
	}
	var found *Variant
	for _, v := range vs {
		if strings.HasPrefix(strings.ToLower(v.ID), name) || strings.HasPrefix(strings.ToLower(v.Name), name) {
			if found != nil && found != v {
				return nil, fmt.Errorf("%q matches both %s and %s", name, found.ID, v.ID)
			}
			found = v
		}
	}
	if found == nil {
		ids := make([]string, len(vs))
		for i, v := range vs {
			ids[i] = v.ID
		}
		return nil, fmt.Errorf("no variant %q (have %s)", name, strings.Join(ids, ", "))
	}
	return found, nil
}
