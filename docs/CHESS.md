# Chess

Play orthodox chess or a bundled/custom variant against the computer or another
person. By default the computer plays Black at **normal** level.

```sh
chess --level beginner
chess --level hard --think 5
chess double --level easy
chess --computer both --level expert
chess --computer none
chess --list
```

`--computer` accepts `white`, `black`, `both`, or `none`. One level applies to
every computer-controlled side. All levels support every variant, including
custom fairy pieces; no orthodox-specific rules or external engine are used.

## Stalemate rules

The bundled `stalematewins` variant (Stalemate-wins Chess) uses the
"stalemate = win" rule studied with AlphaZero: a player who has no legal move
and is not in check loses, so the player who delivered stalemate wins. See
Tomašev, Paquet, Hassabis and Kramnik, [*Assessing Game Balance with AlphaZero:
Exploring Alternative Rule Sets in Chess*](https://arxiv.org/abs/2009.04374)
(2020). The bundled `antichess` variant goes the other way: captures are
compulsory, a side with no legal move wins, material preference is reversed,
and the king is non-royal, so it can be captured and promotion may produce one.

Custom variant YAML can set `stalemate` to `draw`, `win`, or `loss`. The default
when the field is omitted is `draw` (orthodox chess); `win` means the side
delivering stalemate wins, and `loss` means the side delivering it loses.
`forced_capture: true` requires a capture whenever one is legal, and
`material: inverse` makes the evaluation prefer having fewer, less valuable
pieces; both are described under [Variant files](#variant-files).

| Level | Default time ceiling | Depth cap | Node cap | Root selection |
| --- | ---: | ---: | ---: | --- |
| beginner | 1 s | 1 | 4,000 | 35% chance of an alternative within 200 cp |
| easy | 1 s | 2 | 20,000 | 15% chance of an alternative within 75 cp |
| normal | 3 s | unrestricted* | none | best searched move |
| hard | 6 s | unrestricted* | none | best searched move |
| expert | 12 s | unrestricted* | none | best searched move |

\* The engine's existing safety ceiling is 48 completed iterations. Check
extensions and quiescence can look beyond a shallow profile's nominal depth.
These are resource/selection profiles, **not measured Elo ratings or proven
strength separations**. Initial partial measurements and remaining validation
are in [CHESS_CALIBRATION.md](CHESS_CALIBRATION.md).

`--think 1` through `--think 3600` replaces only the time ceiling. It never
removes beginner/easy depth or node caps or changes their selection policy.
Search stops at the first applicable limit, and may finish early on forced
moves, known mates, or when too little time remains for another iteration.
Main-search and quiescence entries both count as nodes; node limits have no
overshoot. Time is checked at root moves and every 64 recursive entries;
individual move-generation/evaluation operations cannot be interrupted.

Weak levels sample only candidates fully scored at the same completed depth,
using full search windows, and fall back to the best completed search if scoring
all candidates exceeds the budget. Alternatives can tie the best score. Known
mates are handled separately from centipawns; weakening never sacrifices a
known immediate win or chooses known forced mate against itself over a safe
scored alternative. Scores remain the engine's imperfect assessments, not an
oracle. Each weak search has fresh tables and locally seeded randomness.

## Bundled variants

`chess --list` shows every variant available. The embedded ones are:

- **Chess** (`chess`): orthodox chess.
- **Amazon Chess** (`chess amazon`): 8×8 orthodox chess with queens replaced by Amazons, moving as queen or knight. Castling, en passant, and checkmate/stalemate follow orthodox rules; promotion is to Amazon, rook, bishop, or knight. The Amazon's value (1200 cp) is approximate. This queen-replacement variant follows the published Amazon piece definition at [Chess Variants](https://www.chessvariants.com/diffmove.dir/amazone.html) and [Wikipedia](https://en.wikipedia.org/wiki/Amazon_(chess)).

- **No-castling Chess** (`chess nocastling`): orthodox chess with castling disabled.
  It is based on the AlphaZero rule-change variant described by Tomašev et al.,
  [Assessing Game Balance with AlphaZero](https://arxiv.org/abs/2009.04374) (2020).

- **Capablanca Chess** (`capablanca`): 10×8, with Archbishop (bishop + knight)
  and Chancellor (rook + knight). Kings castle three squares; pawns move as in
  chess and promote to Q, C, A, R, B, or N. Checkmate wins, stalemate draws,
  and the 50-move rule applies. Rules:
  [Wikipedia](https://en.wikipedia.org/wiki/Capablanca_chess),
  [Chess Variants](https://www.chessvariants.com/large.dir/capablanca.html).

- **Berolina Chess** (`berolina`): otherwise orthodox rules, but pawns move
  diagonally without capturing and capture straight ahead. Their initial
  double-step is diagonal, and en passant is captured straight ahead on the
  square they pass. Rules:
  [Wikipedia](https://en.wikipedia.org/wiki/Berolina_chess),
  [Chess Variants](https://www.chessvariants.com/dif-pawn.dir/berolina.html).

- **Stalemate-wins Chess** (`chess stalematewins`): orthodox chess except that
  the side delivering stalemate wins; see
  [Stalemate rules](#stalemate-rules).

- **Self-capture Chess** (`selfcapture`): orthodox chess where a player may
  capture their own pieces with normal capture movement, except the king.
  Self-capture Chess follows the self-capture rule set of Tomašev, Paquet,
  Hassabis and Kramnik, *Assessing Game Balance with AlphaZero: Exploring
  Alternative Rule Sets in Chess* (2020),
  [arXiv:2009.04374](https://arxiv.org/abs/2009.04374). Only friendly non-royal
  pieces can be taken; a piece's ordinary capture moves apply (so a pawn
  self-captures diagonally, never forward), and all other rules, including
  castling, promotion, en passant and checkmate, are orthodox. The engine
  exposes the rule as the variant YAML field `self_capture: true`, which
  `extends:` inherits like any other field (a child may switch it off with
  `self_capture: false`).

- **Double Chess** (`double`): Julian Hayward, 1916: two armies side by side on
  16×12, pawns may first move up to four squares, kings castle within their own
  half, checkmating either enemy king wins.

- **Betza's Big Board** (`bigboard`, listed as "Chess on a Really Big Board"):
  Ralph Betza, 1996: sixteen pieces a side on 16×16, pawns may first move as
  far as the middle, castling rooks move as far as in chess.

- **Double Big Board** (`doublebig`): two Big Board armies per side on 32×16;
  each king castles within its own half.

- **King of the Hill** (`kingofthehill`): 8×8, orthodox chess where a royal
  reaching `d4`, `d5`, `e4` or `e5` wins at once; otherwise plain chess, so mate,
  stalemate, repetition and move-limit endings still apply. See
  [Variant files](#variant-files) for the `goal` key.

- **Grand Chess** (`grand`): Christian Freeling, 1984: ten pieces a side on
  10×10 with a marshal (rook + knight) and a cardinal (bishop + knight), no
  castling. Pawns promote from their eighth rank on, may decline inside the
  zone, and only to a piece their side has captured (see
  [Variant files](#variant-files)). Rules:
  [Wikipedia](https://en.wikipedia.org/wiki/Grand_chess),
  [Chess Variants](https://www.chessvariants.com/large.dir/freeling.html).

- **Three-Check** (`threecheck`): 8×8, orthodox chess where giving check
  three times wins at once, the checkmating third check included; the usual
  mate, stalemate, repetition and move-limit endings still apply, and if the
  limit-th check is also mate the game is won either way. See
  [Variant files](#variant-files) for the `check_limit` key.

- **Racing Kings** (`racingkings`): V. R. Parton, 1961: 8×8 with no pawns, no
  castling and no promotion, both armies on ranks 1 and 2 (Black on the a–d
  files, White on the e–h files), and neither side may give check. The whole
  eighth rank is a goal for both colours: the first king there wins, except
  that White's arrival is only pending while Black can answer on the eighth
  rank, which draws. See [Variant files](#variant-files) for the `goal`,
  `no_checks` and `goal_reply_draw` keys.

Because checking is illegal, no move in Racing Kings can ever be mate:
stalemate, repetition and the move limit are what end a race that no king wins.
The bare-kings material rule does not apply while a variant has goal squares
either, since a lone king can still reach the eighth rank.

- **Antichess** (`antichess`): orthodox chess except that a capture is
  compulsory whenever one is legal, and a side with no legal move at all wins,
  so a player wins by being stalemated or by losing every piece. The king is a
  non-royal piece that can be captured, and pawns may promote to it, so
  checkmate plays no part. Material preference is reversed (`material:
  inverse`), and the computer sheds pieces instead of hoarding them. See
  [Variant files](#variant-files) for the `forced_capture`, `material` and
  `stalemate` keys.

- **Atomic Chess** (`atomic`): 8×8, orthodox chess except that a capture
  explodes: it removes the capturing piece, the captured piece and every
  adjacent non-pawn piece of either colour. Kings move as `mK` and so never
  capture, a move that would explode its own king is illegal, and exploding the
  enemy king wins at once, even when the moving side would otherwise be in
  check. See [Explosions and extinction](#explosions-and-extinction) for the
  `explosion` and `extinction` keys, the
  [Lichess rules](https://lichess.org/variant/atomic) and
  [Wikipedia](https://en.wikipedia.org/wiki/Atomic_chess).

- **Chess960** (`chess960`): Bobby Fischer's shuffle chess: the back rank is
  drawn at random from the 960 arrangements with bishops on opposite colours
  and the king between the rooks, Black mirrors White, and castling follows the
  position instead of fixed squares (see [Chess960](#chess960)). Rules:
  [FIDE Laws of Chess, Chess960 rules](https://handbook.fide.com/chapter/E012023),
  [Wikipedia](https://en.wikipedia.org/wiki/Fischer_random_chess).

`chess --list` prints them and `chess <id|name>` starts one; `v`/`V` opens the
variant picker during play. A variant file in `chess/*.yaml` next to the binary
or in `~/.config/misa/chess/` joins the list, and a file whose `id` matches an
earlier variant replaces it.

## During play

- Arrows or `h/j/k/l`: move the cursor; space/enter or click: select/move. In a
  self-capture variant, activating a friendly piece that the selected piece can
  legally capture plays that self-capture; click the selected piece to deselect
  first if you meant to switch pieces.
- `d`: cycle beginner → easy → normal → hard → expert → beginner.
- `c`: cycle computer sides; `u`/backspace: undo to a human turn.
- `n`: new game; `v`/`V`: open the variant picker. Type to filter by name or id;
  ↑/↓ or `j`/`k` (when the filter is empty) to move, enter to switch, esc to cancel.
- `f`: flip; `t`: toggle piece letters; `q`: quit.

In the GUI (`misa --gui`), standard piece glyphs come from the system font when
it has them; otherwise Misa falls back to its embedded "Misa Chess Glyphs" font.
That font covers only the Western chess symbols U+2654–U+265F. Shogi and
Xiangqi pieces render as one-cell letters, so they don't need it; any future
variant that uses other symbols needs a system font that has them.

The header shows, for example, `Chess · hard · Black to move · thinking…`.
Changing difficulty cancels the old search and starts the new policy without
applying obsolete results. The chosen level survives new games, variant
changes, and hotseat, but is not persisted after quitting. Analysis after a
computer move reports that move's score, including deliberate alternatives;
an unanalysed legal fallback is labelled instead of showing a fabricated score.

## Variant files

Every rule comes from YAML: pieces are Betza strings, `setup` is FEN, `castling`
lists White's four squares (Black's mirror them), and `promotion` lists the
pieces a pawn may become. A file with `extends: <id>` inherits every unset field
from that variant and merges pieces by letter; `castling: []` drops inherited
castling. A later file with an existing id replaces it. Files are read from the
embedded set, then `chess/*.yaml` next to the binary, then
`~/.config/misa/chess/`.

`goal` names **absolute** squares, not White's squares: a royal piece arriving
on any of them wins immediately, for either side, and that arrival outranks a
checkmate or draw that the same move would otherwise produce. Unlike
`castling`, the list is not mirrored for Black, so `kingofthehill.yaml` (which
extends `chess`) needs only its four centre squares:

```yaml
id: kingofthehill
name: King of the Hill
extends: chess
goal: [d4, d5, e4, e5]
```

Each entry accepts a square label as for `castling` (e.g. `d4`). A variant
without `goal` has no goal squares: it can still be won by checkmate or, when
the variant configures it, extinction; otherwise it ends in stalemate,
repetition or the move limit. Grand Chess, for one, sets no goal.

Promotion is governed by four keys:

- `promotion` — the piece types a pawn may become, in prompt order.
- `promotion_zone` (default 1) — how many ranks from the far edge promote.
- `promotion_optional: true` — inside the zone a pawn may decline promotion and
  stay a pawn; on the last rank promotion is still mandatory.
- `promotion_captured_only: true` — a pawn may only promote to a piece type its
  side has fewer of on the board than in the starting setup, that is, only to a
  piece that has been captured. If no such piece is available the pawn cannot
  promote, and therefore cannot move onto the last rank, where promotion is
  forced.

Grand Chess sets `promotion_zone: 3` with both flags: its pawns promote from
their eighth rank on and may wait for a captured piece, and a side never owns
two queens or two marshals.

Where a variant lets a pawn decline, the prompt lists the pawn first: press its
letter (`p`) or click it to leave the pawn as it is. Long prompt lists fall back
to glyphs on a narrow terminal.

`check_limit: N` wins the game for a side that gives check N times. It is not
tied to any id: 0 or no field turns the rule off, `extends` inherits it, and a
child file overrides it with a positive value (a negative one is rejected).
Every move that leaves the opponent in check counts, including the checkmating
one; reaching the limit outranks a checkmate or draw rule the same move would
produce, exactly as a `goal` arrival does. For example:

```yaml
id: threecheck
name: Three-Check
extends: chess
check_limit: 3
```

The header shows the checks left (`checks left: White 3, Black 0`) and
names the winner once a side reaches the limit.

A FEN for such a variant also takes a checks field anywhere after the
side-to-move field: `+2+1` counts the checks White and Black have already
given, while Lichess's `1+2` style gives the checks each side has left. A
variant without a check limit ignores the field, so existing FENs are
unchanged.

Two flags refine goal variants. `no_checks: true` makes a move that leaves the
opponent's royal piece attacked illegal, for both colours, so a checking move is
never a legal move at all: the move list, the notation, the search and the UI
all agree that it does not exist, and checkmate can therefore never occur.
`goal_reply_draw: true` makes the arrival provisional for the side that moves
first. White reaching a goal square is not yet a win while Black has a legal
move that also puts a royal on a goal square; if Black plays it the game ends as
`draw by the goal reply`, and if Black cannot, the arrival wins. A goal arrival
by Black always wins at once. Both flags default to `false`, are inherited
through `extends` like every other field, and a child file may set them back to
`false`. `goal_reply_draw` without goal squares would do nothing, so it is a
load error. A variant with goal squares is also never drawn by the bare-kings
material rule, because a lone king can still reach a goal.

`racingkings` uses both flags; its piece list is the ordinary one, so only the
shape of the variant is shown here:

```yaml
id: racingkings
name: Racing Kings
files: 8
ranks: 8
setup: 8/8/8/8/8/8/krbnNBRK/qrbnNBRQ   # Black a-d, White e-h, ranks 1-2
castling: []
promotion: []
goal: [a8, b8, c8, d8, e8, f8, g8, h8]  # the eighth rank, both colours
no_checks: true
goal_reply_draw: true
```

`forced_capture: true` makes every legal capture compulsory: the move list
holds only captures whenever one exists, en passant and capture-promotions
included, and a capture that would leave the mover's own royal in check does
not restrict the quiet moves. It defaults to `false` and is inherited through
`extends` like the other flags. `material:` decides how the evaluation weighs
material: `normal` (the default) prefers more and more valuable pieces, and
`inverse` prefers fewer, so the engine seeks to give its own pieces away. Both
keys are what makes antichess play like antichess:

```yaml
id: antichess
name: Antichess
extends: chess
pieces:
  - {letter: K, name: King, betza: K, royal: false, value: 300, glyph: ♚}
castling: []
promotion: [Q, R, B, N, K]
forced_capture: true
material: inverse
stalemate: loss
```

Dropping `royal` from the king leaves a variant with no royal piece at all.
Nothing can then be in check or mate, the bare-kings material draw does not
apply, and every ending comes from `stalemate:`, which antichess sets to `loss`:
the side that cannot move wins, whether it lost its last piece or is
simply stuck.

## Explosions and extinction

Variants can configure these mechanisms independently, for example:

```yaml
explosion: {radius: 1, immune: [P]}
extinction: [K]
```

A capture explodes around its destination, removing pieces within the configured
Chebyshev radius (a square neighbourhood). Listed immune piece types survive the
blast, but the capturer and captured piece are always removed, even if immune.
Radius 0 disables explosions, including an inherited configuration. With
explosions enabled, a royal within that radius of an enemy royal is connected
and is not considered in check. A legal move must preserve all of the mover's
royals and either extinguish the opponent or leave the mover out of check.

A side loses when none of its listed extinction types remain. Extinguishing the
opponent wins immediately, before goal, check limit, checkmate, stalemate or
draw adjudication, and it is the `extinction` status the header, Match.Decisive
and the search treat as decisive. If both sides become extinct at once, the side
that just moved wins (losing a royal still makes that move illegal). The scalar
`all` or `"*"` expands to all piece types, useful for horde-style armies. An
empty list (`extinction: []`) clears inherited extinction. An omitted
`explosion` inherits the parent's configuration; an explicit `explosion`
mapping replaces it as a whole. Other unspecified variant fields are inherited
from the parent. The computer disables null-move pruning when explosions are
enabled, since passing is not a safe tactical shortcut for these positions.

### Atomic

A capture removes the capturer, captured piece and all adjacent nonpawns of
either side. En passant explodes around the capturing pawn’s destination, not
the captured pawn’s square. Kings use `mK` and cannot capture. A move that
explodes its own royal is illegal; exploding the enemy king wins immediately,
even if the moving king would otherwise be in check. Connected royals suppress
check. Normal castling, en passant, promotion, checkmate, stalemate, repetition
and move-limit rules remain in effect. See [Lichess Atomic rules](https://lichess.org/variant/atomic)
and [Wikipedia: Atomic chess](https://en.wikipedia.org/wiki/Atomic_chess).

## Custom board dimensions

Variants support 1–702 files (labels `a` through `zz`) and 1–99 ranks.

Large boards can be expensive to load and play: reach tables, move generation,
and search consume more memory and time as the board grows.

## Chess960

Chess960 shuffles the back rank at the start of each new game, with bishops on
opposite colors and the king between the rooks. Black mirrors White's back rank;
the header shows the original White back rank in piece letters. Pawns start as
in orthodox chess. There are 960 possible arrangements.

Castling leaves the king on c1 and rook on d1 (queenside), or king on g1 and
rook on f1 (kingside), mirrored for Black. The king and rook must retain their
castling rights; the relevant paths must be clear except for the castling king
and rook, and the king cannot start in, cross, or finish in check. Depending on
the starting arrangement, either piece may already be on its destination, or
the king and rook may swap squares. Select the king then its destination **or**
the castling rook, using mouse or keyboard. If an ordinary king move shares a
castle's destination, that entry castles; the rook is an unambiguous entry.
Chess960 uses the engine's orthodox win and draw rules, with no special changes.

Rules references: [FIDE Laws of Chess, Chess960 rules](https://handbook.fide.com/chapter/E012023)
and [Wikipedia: Fischer random chess](https://en.wikipedia.org/wiki/Fischer_random_chess).

## YAML setup and castling mechanisms

A variant may shuffle a full White rank, mirrored on Black's corresponding
rank, with constraints expressed entirely in YAML:

```yaml
shuffle:
  rank: 1
  opposite_color: B
  between: {royal: K, pieces: R}
castling:
  - {rook_side: right, rook_piece: R, king_to: g1, rook_to: f1}
  - {rook_side: left, rook_piece: R, king_to: c1, rook_to: d1}
```

`rank` is one-based. The setup must contain two `opposite_color` pieces, one
`between.royal` (a royal type), and two `between.pieces` on that rank; these
three types must differ. Other pieces retain their counts but can occupy any
remaining files. The two opposite-color pieces occupy different square colors,
and the royal stands between the surrounding pair. The original setup must
have a full White rank and an identical, full Black mirror. Invalid or
impossible constraints are load errors. Enumeration is bounded to 10 files
and 1,000,000 arrangements to limit custom-variant loading costs.

Each new match, including `n` and a variant change through the `v`/`V` picker,
draws uniformly from the valid arrangements; consecutive draws can coincide. The engine's
`NewPositionWithRand` and `NewMatchWithRand` accept a seeded `math/rand/v2.Rand`
for reproducible draws (nil uses the default RNG). Chess960's enumeration uses
Scharnagl numbering, with orthodox chess at index 518. Variants without
`shuffle` retain their fixed setup unchanged.

Dynamic castling entries replace fixed `king`/`rook` origins with `rook_side:
left|right` and `rook_piece`, locating the royal and that side's outermost rook
of the named type on the destination rank in the actual starting position.
The origins and rights remain attached to the position through play, undo and
search; moving either piece or capturing the rook permanently removes the
right. Existing four-square castling YAML continues to work unchanged. All
squares from king origin to destination and rook origin to destination must be
empty except for the two castling pieces. Rook attacks do not prevent castling;
king attacks before, during, or after the move do.

`Position.FEN()` emits dynamic castling rights as Shredder-FEN rook-file letters
(uppercase for White, lowercase for Black); the parser also accepts `KQkq`,
meaning the outermost rook on each side. File letters identify the exact rook
when other rooks share that side. Supply explicit rights when loading a
non-template setup; `-` removes all rights. Fixed castling entries serialize
with their existing `KQkq` convention. En passant, the halfmove clock and, for
a variant with a `check_limit`, the checks each side has given also round-trip
for ordinary chess; FEN cannot express arbitrary fairy-piece moved flags or
multiple crossed en-passant squares.
