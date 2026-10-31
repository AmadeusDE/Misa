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
)

//go:embed variants
var builtinFS embed.FS

var builtinFiles = []string{"chess.yaml", "double.yaml", "bigboard.yaml", "doublebig.yaml", "amazon.yaml", "nocastling.yaml", "capablanca.yaml", "berolina.yaml", "stalematewins.yaml", "selfcapture.yaml", "kingofthehill.yaml", "grand.yaml", "threecheck.yaml", "racingkings.yaml", "antichess.yaml", "atomic.yaml", "chess960.yaml", "crazyhouse.yaml", "xiangqi.yaml", "horde.yaml", "fog.yaml", "duck.yaml", "shogi.yaml"}

// PieceType is one kind of piece a variant uses. Type numbers in Piece index
// Variant.Pieces from 1, so the zero Piece is an empty square.
type PieceType struct {
	Letter          byte // upper case; White's pieces print in upper case, Black's in lower
	Name            string
	Betza           string
	Glyph           string // one terminal cell; the letter is used when empty
	Royal           bool   // may not be left attacked; goals and check limits count royals
	Pawn            bool   // promotes, takes and is taken en passant, resets the move clock
	DropForbidRanks []int  // side-relative ranks on which this piece cannot be dropped
	DropOnePerFile  bool   // only one unpromoted piece of this type per file
	DropNoMate      bool   // dropping this type may not deliver immediate checkmate
	// A piece may carry its own promotion rule; without one a pawn uses the
	// variant's Promotion and PromotionZone.
	Promotion          []uint8 // types this piece becomes, or nil
	PromotionZone      int     // side-relative ranks at the far end that promote
	PromotionOptional  bool    // the mover may decline while inside the zone
	PromotionOnExit    bool    // leaving the zone also promotes (a shogi piece)
	PromotionMandatory int     // ranks inside which the mover must promote, or 0
	promotionDefined   bool    // the file gave this piece its own promotion rule
	Demote             uint8   // original type of a promoted piece, or 0
	Value              int     // centipawns for the computer player; estimated when the file leaves it out

	zone        []bool
	moveZones   []moveZone
	royalAttack []Component
	moves       []Component
	tracksMoved bool
	reach       [2]reachTable
}

// Castle is one castling move, given by White's squares; Black's mirror them
// across the middle rank.
type Castle struct {
	King, Rook, KingTo, RookTo int
	RookSide                   string
	RookType                   uint8
}

// Explosion describes the capture blast and types immune to collateral damage.
type Explosion struct {
	Radius int
	Immune []uint8
}

type Variant struct {
	ID, Name, Description string
	Files, Ranks          int
	SelfCapture           bool
	Drops                 bool // captures go to a pocket, which may be dropped back
	Pieces                []*PieceType
	Castling              []Castle
	Promotion             []uint8 // piece types a pawn may become
	PawnDoubleRanks       []int   // side-relative 1-based ranks; nil keeps Betza rules
	EPExemptRanks         []int   // side-relative ranks whose double steps do not allow en passant
	PromotionZone         int     // the last n ranks promote
	PromotionOptional     bool    // a pawn may decline to promote inside the zone
	PromotionCapturedOnly bool    // a type is available only once its setup piece is gone
	MoveLimit             int     // moves by each side without a capture or pawn move before a draw
	Repetition            int     // occurrences of a position that draw, three by default
	Blocker               bool    // a neutral, uncapturable duck must relocate after each move
	Stalemate             StalemateRule
	CheckLimit            int        // checks a side must give to win, or 0 for no such rule
	Goal                  []int      // absolute squares whose arrival by a royal wins
	NoChecks              bool       // a move that leaves the other side's royals attacked is illegal
	GoalReplyDraw         bool       // the first side's goal arrival is drawn when the other can arrive in reply
	ForcedCapture         bool       // a legal capture may not be declined
	InverseMaterial       bool       // the evaluation prefers having fewer, less valuable pieces
	Fog                   bool       // show only the squares a side can see; the rules lose checks
	Explosion             *Explosion // captures explode, destroying nearby non-immune pieces
	Extinction            []uint8    // loss when no pieces of any listed type remain
	Setup                 string
	Source                string

	shuffle      *shuffleSetup
	start        []Piece
	startHands   [2][]int // pockets the setup starts with
	startPieces  [2][]int // per side, how many of each type the setup holds
	startBlocker int      // the duck's setup square, or -1
	zobrist      zobrist
}

type pieceDef struct {
	Letter          string        `yaml:"letter"`
	Name            string        `yaml:"name"`
	Betza           string        `yaml:"betza"`
	Glyph           string        `yaml:"glyph"`
	Royal           bool          `yaml:"royal"`
	Pawn            bool          `yaml:"pawn"`
	Value           int           `yaml:"value"`
	DropForbidRanks []int         `yaml:"drop_forbid_ranks"`
	Demote          string        `yaml:"demote"`
	Zone            []string      `yaml:"zone"`
	MoveZones       []moveZoneDef `yaml:"move_zones"`
	RoyalAttack     *string       `yaml:"royal_attack"`
	DropOnePerFile  bool          `yaml:"drop_one_per_file"`
	DropNoMate      bool          `yaml:"drop_no_mate"`
	// A piece's own promotion rule; promotion and promotes_to are the same
	// field written two ways and may not be combined.
	Promotion          []string `yaml:"promotion"`
	PromotesTo         string   `yaml:"promotes_to"`
	PromotionZone      int      `yaml:"promotion_zone"`
	PromotionOptional  bool     `yaml:"promotion_optional"`
	PromotionOnExit    bool     `yaml:"promotion_on_exit"`
	PromotionMandatory int      `yaml:"promotion_mandatory"`
}

type castleDef struct {
	King      string `yaml:"king"`
	Rook      string `yaml:"rook"`
	KingTo    string `yaml:"king_to"`
	RookTo    string `yaml:"rook_to"`
	RookSide  string `yaml:"rook_side"`
	RookPiece string `yaml:"rook_piece"`
}

// StalemateRule is what happens to a side with no legal move that is not in
// check, from the point of view of the side that delivered the stalemate.
type StalemateRule string

const (
	StalemateDraw StalemateRule = "draw"
	StalemateWin  StalemateRule = "win"  // the side delivering stalemate wins
	StalemateLoss StalemateRule = "loss" // the side delivering stalemate loses
)

type explosionDef struct {
	Radius int      `yaml:"radius"`
	Immune []string `yaml:"immune"`
}

// extinctionDef accepts either a sequence of letters or the scalar all or *.
type extinctionDef struct {
	All     bool
	Letters []string
}

func (e *extinctionDef) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" && (strings.EqualFold(n.Value, "all") || n.Value == "*") {
		e.All = true
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("extinction: expected a list of pieces or all")
	}
	return n.Decode(&e.Letters)
}

// variantDef is a variant file. With extends, unset fields come from the named
// variant and pieces add to or replace (by letter) the parent's.
type variantDef struct {
	ID            string      `yaml:"id"`
	Name          string      `yaml:"name"`
	Description   string      `yaml:"description"`
	Drops         *bool       `yaml:"drops"`
	Extends       string      `yaml:"extends"`
	Files         int         `yaml:"files"`
	Ranks         int         `yaml:"ranks"`
	Pieces        []pieceDef  `yaml:"pieces"`
	Setup         string      `yaml:"setup"`
	Castling      []castleDef `yaml:"castling"`
	Promotion     []string    `yaml:"promotion"`
	PromotionZone int         `yaml:"promotion_zone"`
	// The two promotion rules are pointers so an inheriting variant can tell
	// "unset" (keep the parent's) from "false". no_checks, goal_reply_draw and
	// fog are pointers for the same reason.
	PromotionOptional     *bool          `yaml:"promotion_optional"`
	PromotionCapturedOnly *bool          `yaml:"promotion_captured_only"`
	PawnDoubleRanks       []int          `yaml:"double_step_ranks"`
	EPExemptRanks         []int          `yaml:"ep_exempt_ranks"`
	MoveLimit             int            `yaml:"move_limit"`
	CheckLimit            int            `yaml:"check_limit"`
	Stalemate             StalemateRule  `yaml:"stalemate"`
	SelfCapture           *bool          `yaml:"self_capture"`
	Goal                  []string       `yaml:"goal"`
	NoChecks              *bool          `yaml:"no_checks"`
	GoalReplyDraw         *bool          `yaml:"goal_reply_draw"`
	ForcedCapture         *bool          `yaml:"forced_capture"`
	Fog                   *bool          `yaml:"fog"`
	Material              string         `yaml:"material"`
	Explosion             *explosionDef  `yaml:"explosion"`
	Extinction            *extinctionDef `yaml:"extinction"`
	Blocker               *bool          `yaml:"blocker"`
	Repetition            int            `yaml:"repetition"`

	Shuffle *shuffleDef `yaml:"shuffle"`
	source  string
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
	var dirs []string
	if execPath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(execPath), subdir))
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(cfgDir, "misa", subdir))
	}
	return dirs
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
	if child.Drops != nil {
		d.Drops = child.Drops
	}
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
	if child.Shuffle != nil {
		d.Shuffle = child.Shuffle
	}
	if child.Castling != nil {
		d.Castling = child.Castling
	}
	if child.Promotion != nil {
		d.Promotion = child.Promotion
	}
	if child.PawnDoubleRanks != nil {
		d.PawnDoubleRanks = child.PawnDoubleRanks
	}
	if child.EPExemptRanks != nil {
		d.EPExemptRanks = child.EPExemptRanks
	}
	if child.PromotionZone != 0 {
		d.PromotionZone = child.PromotionZone
	}
	if child.PromotionOptional != nil {
		d.PromotionOptional = child.PromotionOptional
	}
	if child.PromotionCapturedOnly != nil {
		d.PromotionCapturedOnly = child.PromotionCapturedOnly
	}
	if child.MoveLimit != 0 {
		d.MoveLimit = child.MoveLimit
	}
	if child.Stalemate != "" {
		d.Stalemate = child.Stalemate
	}
	if child.Repetition != 0 {
		d.Repetition = child.Repetition
	}
	if child.SelfCapture != nil {
		d.SelfCapture = child.SelfCapture
	}
	if child.CheckLimit != 0 {
		d.CheckLimit = child.CheckLimit
	}
	if child.Fog != nil {
		d.Fog = child.Fog
	}
	if child.Goal != nil {
		d.Goal = child.Goal
	}
	if child.NoChecks != nil {
		d.NoChecks = child.NoChecks
	}
	if child.GoalReplyDraw != nil {
		d.GoalReplyDraw = child.GoalReplyDraw
	}
	if child.ForcedCapture != nil {
		d.ForcedCapture = child.ForcedCapture
	}
	if child.Material != "" {
		d.Material = child.Material
	}
	if child.Explosion != nil {
		d.Explosion = child.Explosion
	}
	if child.Extinction != nil {
		d.Extinction = child.Extinction
	}
	if child.Blocker != nil {
		d.Blocker = child.Blocker
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
		PromotionZone: d.PromotionZone, MoveLimit: d.MoveLimit, CheckLimit: d.CheckLimit,
		Repetition:            d.Repetition,
		Stalemate:             d.Stalemate,
		SelfCapture:           d.SelfCapture != nil && *d.SelfCapture,
		PromotionOptional:     d.PromotionOptional != nil && *d.PromotionOptional,
		PromotionCapturedOnly: d.PromotionCapturedOnly != nil && *d.PromotionCapturedOnly,
	}
	if d.ForcedCapture != nil {
		v.ForcedCapture = *d.ForcedCapture
	}
	switch d.Material {
	case "", "normal":
	case "inverse":
		v.InverseMaterial = true
	default:
		return nil, fmt.Errorf("material must be normal or inverse")
	}
	if d.Drops != nil {
		v.Drops = *d.Drops
	}
	if d.Fog != nil {
		v.Fog = *d.Fog
	}
	if d.Blocker != nil {
		v.Blocker = *d.Blocker
	}
	v.startBlocker = -1
	if v.Repetition == 0 {
		v.Repetition = 3
	}
	if v.Repetition < 2 {
		return nil, fmt.Errorf("repetition must be at least 2, not %d", v.Repetition)
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
	if v.CheckLimit < 0 {
		return nil, fmt.Errorf("check limit must not be negative, not %d", v.CheckLimit)
	}
	for name, ranks := range map[string][]int{"double_step_ranks": d.PawnDoubleRanks, "ep_exempt_ranks": d.EPExemptRanks} {
		seen := map[int]bool{}
		for _, rank := range ranks {
			if rank < 1 || rank > v.Ranks || seen[rank] {
				return nil, fmt.Errorf("%s: invalid or duplicate rank %d", name, rank)
			}
			seen[rank] = true
		}
	}
	v.PawnDoubleRanks = slices.Clone(d.PawnDoubleRanks)
	v.EPExemptRanks = slices.Clone(d.EPExemptRanks)
	if err := v.parseGoal(d.Goal); err != nil {
		return nil, err
	}
	if d.NoChecks != nil {
		v.NoChecks = *d.NoChecks
	}
	if d.GoalReplyDraw != nil {
		v.GoalReplyDraw = *d.GoalReplyDraw
	}
	if v.GoalReplyDraw && len(v.Goal) == 0 {
		return nil, fmt.Errorf("goal_reply_draw needs goal squares")
	}
	if len(d.Pieces) == 0 || len(d.Pieces) > 255 {
		return nil, fmt.Errorf("needs 1-255 piece types")
	}
	for _, p := range d.Pieces {
		t, err := buildPiece(p)
		if err != nil {
			return nil, err
		}
		if err := v.buildZones(t, p); err != nil {
			return nil, fmt.Errorf("piece %c: %v", t.Letter, err)
		}
		if v.TypeOf(t.Letter) != 0 {
			return nil, fmt.Errorf("piece letter %c used twice", t.Letter)
		}
		v.Pieces = append(v.Pieces, t)
	}
	if err := v.setCaptureRules(d); err != nil {
		return nil, err
	}
	for i, p := range d.Pieces {
		t := v.Pieces[i]
		if p.PromotionZone < 0 || p.PromotionZone > v.Ranks || p.PromotionMandatory < 0 || p.PromotionMandatory > v.Ranks {
			return nil, fmt.Errorf("piece %c: invalid promotion ranks", t.Letter)
		}
		t.PromotionZone, t.PromotionMandatory = p.PromotionZone, p.PromotionMandatory
		if t.PromotionZone == 0 {
			t.PromotionZone = v.PromotionZone
		}
		t.PromotionOptional, t.PromotionOnExit = p.PromotionOptional, p.PromotionOnExit
		t.DropOnePerFile, t.DropNoMate = p.DropOnePerFile, p.DropNoMate
		if p.PromotesTo != "" {
			if p.Promotion != nil {
				return nil, fmt.Errorf("piece %c: specify only one of promotion and promotes_to", t.Letter)
			}
			p.Promotion = []string{p.PromotesTo}
		}
		t.promotionDefined = p.Promotion != nil
		for _, letter := range p.Promotion {
			if len(letter) != 1 || v.TypeOf(letter[0]) == 0 {
				return nil, fmt.Errorf("piece %c: unknown promotion piece %q", t.Letter, letter)
			}
			t.Promotion = append(t.Promotion, v.TypeOf(letter[0]))
		}
		for _, rank := range p.DropForbidRanks {
			if rank < 1 || rank > v.Ranks {
				return nil, fmt.Errorf("piece %c: invalid forbidden rank %d", v.Pieces[i].Letter, rank)
			}
		}
		v.Pieces[i].DropForbidRanks = slices.Clone(p.DropForbidRanks)
		if p.Demote != "" {
			if len(p.Demote) != 1 || v.TypeOf(p.Demote[0]) == 0 {
				return nil, fmt.Errorf("piece %c: unknown demote piece %q", v.Pieces[i].Letter, p.Demote)
			}
			v.Pieces[i].Demote = v.TypeOf(p.Demote[0])
		}
	}
	placement, hands, err := v.splitFENHands(d.Setup)
	if err != nil {
		return nil, fmt.Errorf("setup: %v", err)
	}
	v.startHands = hands
	start, blocker, err := v.parsePlacementBlocker(placement)
	if err != nil {
		return nil, fmt.Errorf("setup: %v", err)
	}
	v.start = start
	v.startBlocker = blocker
	if d.Shuffle != nil {
		if err := v.buildShuffle(d.Shuffle); err != nil {
			return nil, fmt.Errorf("shuffle: %v", err)
		}
	}
	v.countStart()
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

func (v *Variant) setCaptureRules(d variantDef) error {
	parse := func(field string, letters []string) ([]uint8, error) {
		var result []uint8
		for _, l := range letters {
			if len(l) != 1 || v.TypeOf(l[0]) == 0 {
				return nil, fmt.Errorf("%s: unknown piece %q", field, l)
			}
			t := v.TypeOf(l[0])
			if slices.Contains(result, t) {
				return nil, fmt.Errorf("%s: piece %q listed twice", field, l)
			}
			result = append(result, t)
		}
		return result, nil
	}
	if d.Explosion != nil {
		if d.Explosion.Radius < 0 {
			return fmt.Errorf("explosion: radius must be nonnegative")
		}
		immune, err := parse("explosion immune", d.Explosion.Immune)
		if err != nil {
			return err
		}
		if d.Explosion.Radius > 0 {
			v.Explosion = &Explosion{Radius: d.Explosion.Radius, Immune: immune}
		}
	}
	if d.Extinction != nil {
		if d.Extinction.All {
			for i := range v.Pieces {
				v.Extinction = append(v.Extinction, uint8(i+1))
			}
		} else {
			var err error
			v.Extinction, err = parse("extinction", d.Extinction.Letters)
			if err != nil {
				return err
			}
		}
	}
	return nil
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

func (v *Variant) parseGoal(squares []string) error {
	seen := make(map[int]bool, len(squares))
	for _, name := range squares {
		sq, err := v.ParseSquare(name)
		if err != nil {
			return fmt.Errorf("goal: %v", err)
		}
		if seen[sq] {
			return fmt.Errorf("goal: square %s listed twice", name)
		}
		seen[sq] = true
		v.Goal = append(v.Goal, sq)
	}
	return nil
}

func (v *Variant) parseCastle(c castleDef) (Castle, error) {
	if c.RookSide != "" || c.RookPiece != "" {
		if c.King != "" || c.Rook != "" {
			return Castle{}, fmt.Errorf("dynamic castle cannot specify origins")
		}
		if c.RookSide != "left" && c.RookSide != "right" {
			return Castle{}, fmt.Errorf("rook_side must be left or right")
		}
		if len(c.RookPiece) != 1 || v.TypeOf(c.RookPiece[0]) == 0 {
			return Castle{}, fmt.Errorf("unknown rook_piece %q", c.RookPiece)
		}
		kt, err := v.ParseSquare(c.KingTo)
		if err != nil {
			return Castle{}, err
		}
		rt, err := v.ParseSquare(c.RookTo)
		if err != nil {
			return Castle{}, err
		}
		if kt/v.Files != rt/v.Files || kt == rt {
			return Castle{}, fmt.Errorf("destinations must be distinct squares on one rank")
		}
		return Castle{KingTo: kt, RookTo: rt, RookSide: c.RookSide, RookType: v.TypeOf(c.RookPiece[0])}, nil
	}
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
	return Castle{King: sq[0], Rook: sq[1], KingTo: sq[2], RookTo: sq[3]}, nil
}

// countStart records how many of each piece type each side's setup holds.
// Captured-only promotion compares the board against it.
func (v *Variant) countStart() {
	for _, c := range []Color{White, Black} {
		v.startPieces[c] = make([]int, len(v.Pieces)+1)
	}
	for _, pc := range v.start {
		if pc.Type != 0 {
			v.startPieces[pc.Color][pc.Type]++
		}
	}
}

// startCount returns how many of type t side c has in the variant's setup.
func (v *Variant) startCount(c Color, t uint8) int {
	if c > Black || int(t) >= len(v.startPieces[c]) {
		return 0
	}
	return v.startPieces[c][t]
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
		t.tracksMoved = t.Royal || slices.ContainsFunc(t.moves, func(c Component) bool { return c.Initial }) || slices.ContainsFunc(t.royalAttack, func(c Component) bool { return c.Initial })
		for _, z := range t.moveZones {
			t.tracksMoved = t.tracksMoved || slices.ContainsFunc(z.moves, func(c Component) bool { return c.Initial })
		}
	}
	for _, c := range v.Castling {
		if c.RookSide != "" {
			v.Pieces[c.RookType-1].tracksMoved = true
			continue
		}
		for _, color := range []Color{White, Black} {
			for _, sq := range []int{c.King, c.Rook} {
				if p := v.start[v.orient(sq, color)]; p.Type != 0 {
					v.Type(p).tracksMoved = true
				}
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
// (upper case White), numbers of any length are runs of empty squares, and a
// blocker variant's * marks its duck.
func (v *Variant) parsePlacement(s string) ([]Piece, error) {
	board, _, err := v.parsePlacementBlocker(s)
	return board, err
}

// parsePlacementBlocker is parsePlacement told about the one thing a setup
// needs that a board cannot hold: the square of a variant's neutral blocker,
// written * and returned as an index, or -1 when the setup has none.
func (v *Variant) parsePlacementBlocker(s string) ([]Piece, int, error) {
	blocker := -1
	rows := strings.Split(s, "/")
	if len(rows) != v.Ranks {
		return nil, -1, fmt.Errorf("%d ranks, want %d", len(rows), v.Ranks)
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
			case c == '*':
				if !v.Blocker || blocker >= 0 || x >= v.Files {
					return nil, -1, fmt.Errorf("invalid blocker placement")
				}
				blocker = v.Square(x, y)
				x++
			case isLetter(c):
				t := v.TypeOf(c)
				if t == 0 {
					return nil, -1, fmt.Errorf("unknown piece %c", c)
				}
				if x < v.Files {
					color := White
					if c >= 'a' {
						color = Black
					}
					pc := Piece{Type: t, Color: color}
					if j+1 < len(row) && row[j+1] == '~' {
						if !v.Drops || v.Pieces[t-1].Demote == 0 {
							return nil, -1, fmt.Errorf("piece %c cannot be marked promoted", c)
						}
						pc.Promoted = v.Pieces[t-1].Demote
						j++
					}
					board[v.Square(x, y)] = pc
				}
				x++
			default:
				return nil, -1, fmt.Errorf("unexpected %q", c)
			}
			j++
		}
		if x != v.Files {
			return nil, -1, fmt.Errorf("rank %d has %d squares, want %d", y+1, x, v.Files)
		}
	}
	return board, blocker, nil
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
