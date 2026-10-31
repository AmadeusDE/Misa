# Chess

Play orthodox chess or a bundled/custom variant against the computer or another
person. By default the computer plays Black at **normal** level.

```sh
chess --level beginner
chess --level hard --think 5
chess double --level easy
chess xiangqi
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

- **Crazyhouse** (`crazyhouse`): orthodox chess with pockets. A capture does not
  remove the captured piece but changes its colour and puts it in the captor's
  pocket; a captured promoted piece returns as a pawn. Instead of moving, a
  player may drop a pocket piece on an empty square, must leave their own king
  safe (a drop may give check or checkmate), and may not drop a pawn on its
  first or last rank. Castling, en passant, promotion, checkmate, stalemate
  draws, repetition and the 50-move rule work as in chess, and a dropped rook
  never restores castling rights. See [Playing with pockets](#playing-with-pockets)
  for the controls and [Custom drop rules](#custom-drop-rules) for the YAML.
  Rules: [Lichess](https://lichess.org/variant/crazyhouse),
  [Wikipedia](https://en.wikipedia.org/wiki/Crazyhouse).

- **Xiangqi** (`xiangqi`): Chinese chess on a 9×10 board, played on the
  intersections, with generals and advisors confined to the palace, elephants
  that may not cross the river, cannons capturing over exactly one piece,
  soldiers that gain sideways moves across the river, the flying-general rule,
  stalemate counting as a loss for the stalemated side, and perpetual check
  drawn by repetition (see [Xiangqi](#xiangqi)).

- **Horde** (`horde`): 8×8, White is a kingless horde of 36 pawns against
  Black's ordinary army. White wins by checkmating Black; Black wins by
  capturing every horde piece, promoted pieces included, which is the variant's
  `extinction: all` rule. White is never in check (it has no royal piece), it
  may still be stalemated, and stalemate stays a draw. Horde pawns may advance
  two squares from their first or second rank, but only a step from the second
  rank grants en passant (see
  [Pawn double steps and en passant](#pawn-double-steps-and-en-passant)).
  Black castles and promotes as in chess. Rules:
  [Lichess](https://lichess.org/variant/horde),
  [Wikipedia: Dunsany's chess](https://en.wikipedia.org/wiki/Dunsany%27s_chess).

- **Fog of War** (`fog`): 8×8 king-capture chess where each side sees only its
  own pieces and the squares they can reach. There is no check or checkmate and
  capturing the enemy king wins (see [Fog of War](#fog-of-war)).

- **Duck Chess** (`duck`): 8×8 king-capture chess with one neutral, uncapturable
  duck that must be placed on an empty square after every piece move; the duck
  blocks movement and starts off the board. Capturing the enemy king wins, and a
  side with no legal turn wins (see [Duck Chess](#duck-chess)).

- **Shogi** (`shogi`): Japanese chess on a 9×9 board with drops and per-piece
  promotion. Captured pieces change sides and join the captor's hand, and on a
  turn a player may drop a hand piece on an empty square instead of moving.
  Pawns, lances, knights, silvers and bishops and rooks promote inside the last
  three ranks or on leaving them, may decline while a plain move stays legal,
  and must promote where the unpromoted piece could never move again. Drops
  follow Shogi's restrictions: only one unpromoted pawn per file, no lance on
  the last rank and no knight on the last two, and no pawn drop that gives
  immediate checkmate. Fourfold repetition draws, and stalemate is a win for the
  side that delivered it.

`chess --list` prints them and `chess <id|name>` starts one; `v`/`V` opens the
variant picker during play. A variant file in `chess/*.yaml` next to the binary
or in `~/.config/misa/chess/` joins the list, and a file whose `id` matches an
earlier variant replaces it.

### Fog of War

The bundled `fog` variant enables fog with `fog: true`. Fog is an inherited YAML
boolean like other variant settings: omit it to inherit the parent's value, or
set `fog: false` to turn it off in a child variant. For example:

```yaml
id: myfog
name: My Fog Variant
extends: chess
fog: true
```

A side sees all of its own occupied squares, plus every square its own pieces
can legally move to or capture on. This includes pawn forward squares and an
en-passant target when available, but not empty pawn capture diagonals. In
variants with royal pieces, moves that expose a royal to attack are excluded.
Enemy pieces are shown only when they occupy a visible square; hidden pieces
are not shown as markers and their last-seen positions are not remembered. In a
computer game the human always sees their own side's view. In hotseat play the
view follows the side to move.

Fog uses king-capture rules rather than checkmate: there is no check, moving
into attack is legal, and capturing the enemy king wins. Castling is allowed
through attacked squares; en passant and promotion work as usual. The computer
searches the complete position, including pieces hidden from the human player.
It therefore has full information and does not play as a genuinely fog-limited
opponent.

### Duck Chess

Duck Chess starts from the standard chess position with the duck off the board.
Each turn is compound: a piece move, then the duck placed on a different empty
square. The duck blocks movement and cannot be captured. Select a piece and its
destination, then click or move the cursor to the duck's destination and confirm
to complete the turn; `esc` cancels the whole turn. Even a king capture includes
the duck placement.

Kings are not royal, so there is no check or checkmate and capturing the enemy
king wins. Castling is allowed without check restrictions, though the duck can
block its transit or destination squares. A side with no legal turn wins. En
passant, promotion, repetition and the move-limit draw otherwise apply.

The variant sets `blocker: true`, `extinction: [K]` and `stalemate: loss`. That
last value describes the side delivering stalemate: it loses, so the stalemated
side wins.

A setup or FEN marks the duck with `*`, for example
`rnbqkbnr/pppppppp/8/8/8/4P3/PPPP*PPP/RNBQKBNR b KQkq`. The duck is hashed, so
repetition and the transposition table see it. Move history appends `@square`
after the piece move, as in `e2-e4@e2`.

For affordable computer play the search samples at most eight evenly spaced
legal duck destinations for each piece move (and keeps a previous
transposition-table choice). Every legal piece move is still searched, king
captures included, but not every duck placement. Human move selection and the
legal-move list stay exhaustive.

### Xiangqi

Run `chess xiangqi`. The 9×10 board represents the intersections of a Chinese
chess board as squares; the river lies between ranks 5 and 6. White's home is
at rank 1 and Black's at rank 10. Pieces use letters rather than Unicode
Xiangqi glyphs: those glyphs generally occupy two terminal cells, while the
engine requires one-cell glyphs.

- **K, general:** one orthogonal step, confined to its own palace (files d–f,
  its own ranks 1–3). The generals may not face each other on an open file:
  the flying-general attack makes that position illegal.
- **A, advisor:** one diagonal step, confined to its own palace.
- **B, elephant:** two diagonal steps, blocked if the intervening point is
  occupied; it stays on its own half (own ranks 1–5).
- **N, horse:** a knight move blocked when its orthogonal first step (the leg)
  is occupied.
- **R, chariot:** moves and captures like a rook.
- **C, cannon:** moves like a rook without capturing; captures along a rank or
  file by jumping exactly one intervening piece (`mRcpR`).
- **P, soldier:** moves and captures one step forward; after crossing the river
  it can also move sideways, but never backwards. It does not promote.

There is no castling, en passant, or initial double step. Checkmate wins, and
a player with no legal move loses even if not in check (stalemate wins for
the opponent). Perpetual-check and perpetual-chase adjudication is out of
scope: the engine instead draws on the third occurrence of a position. It also
retains its normal move-limit draw (50 moves per side without a capture or pawn
move) and draw when only royal pieces remain. See [Wikipedia: Xiangqi](https://en.wikipedia.org/wiki/Xiangqi)
and [The Chess Variant Pages: Xiangqi](https://www.chessvariants.com/xiangqi.html).

**Xiangqi known gaps (follow-up):** under AXF/WXF rules the side that gives
perpetual check, or perpetual chase, loses the game (the exact rules differ
between rulesets and versions). Misa has no such adjudication, so such a
repetition is scored as an ordinary draw by repetition, like any other. The
engine can therefore repeat a position it ought to avoid, and it can treat a
repeating attack as a safe draw rather than a lost game. The current behaviour
is pinned by `TestXiangqiPerpetualCheckIsDrawForNow` in
`internal/chess/xiangqi_test.go`, so implementing adjudication has to be a
deliberate change that updates that test.

The variant assigns material values R=900, C=450, N=400, B=200, A=200 and
P=100; crossing the river does not add a soldier value bonus.

### Variant YAML mechanisms

These general per-piece fields express Xiangqi without extra piece types or
special UI notation:

- `zone: [d1, e1, ...]` restricts allowed destinations. Coordinates are given
  from White's perspective; Black mirrors ranks.
- `move_zones: [{zone: [...], betza: fWsW}]` replaces a piece's movement when
  its origin is in the zone. The piece keeps its identity; this is not a
  promotion and requires no promotion choice.
- `royal_attack: vR` adds attacks only against enemy royals, ignoring the
  piece's confinement. These attacks use the shared check and legality rules.

Zones list board squares, must be nonempty, and cannot repeat a square.
A piece's movement zones must not overlap; outside them its normal `betza`
applies. Omit `zone` for unrestricted destinations. Confinement limits landing
squares, not the intervening path; pieces still obey their usual blockers.

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

`blocker: true` gives the variant one neutral duck that both sides must relocate
after every piece move (see [Duck Chess](#duck-chess)); a setup may place it with
`*`. `stalemate:` selects the stalemate ending, as described under
[Stalemate rules](#stalemate-rules); `repetition: N` (default 3) is how many
occurrences of an identical position draw, and Shogi sets it to 4. Match
adjudication compares position hashes, pieces and pockets included, and does not
tell perpetual check from any other repetition.

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

Any piece can carry its own promotion rule instead of relying on the variant's
pawn promotion, which is what Shogi needs. The piece-level keys are the same
names with the same meanings:

- `promotes_to: T` — the single type this piece becomes; `promotion: [T]` is the
  multi-target form, and giving both is a load error.
- `promotion_zone: N` — the piece's own zone, defaulting to the variant's.
- `promotion_optional: true` and `promotion_on_exit: true` — decline while a
  plain move stays legal, and promote when leaving the zone rather than only
  when entering it.
- `promotion_mandatory: N` — promote whenever the piece stands inside the last N
  ranks, which is how a lance or a knight that could never move again must
  promote.

A piece without its own rule keeps the variant's pawn promotion, so Grand
Chess's captured-only choices are unaffected. Shogi's rooks and bishops promote
on leaving the three-rank zone as well, which is why its dragons and horses keep
their rank instead of stranding a piece at the far edge.

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

## Pawn double steps and en passant

`double_step_ranks: [1, 2]` replaces the initial Betza double step for
pawn pieces with a forward two-square move from those ranks, provided both
squares are empty. Ranks are 1-based and relative to the moving side (Black's
rank 1 is the board's last rank); eligibility depends on the current rank, not
whether the piece has moved. Omit the field to retain Betza movement, or use
`[]` to disable these double steps. `ep_exempt_ranks: [1]` prevents a double
step from the listed relative ranks from granting en passant rights; other
long pawn moves retain their usual rights. Both fields inherit unless explicitly
replaced, and apply to every piece marked `pawn`.

Horde uses these settings so a pawn can advance two squares from its first or
second rank, but only a double step from its second rank permits en passant.
Black wins by eliminating every White piece, including promoted pieces; White
wins by checkmating Black. A royal-less side is never in check and stalemate
remains a draw. Black retains normal castling and White promotes normally. See
[Lichess Horde rules](https://lichess.org/variant/horde) and
[Wikipedia: Dunsany's chess](https://en.wikipedia.org/wiki/Dunsany%27s_chess).

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

## Playing with pockets

Both sides' pockets show each available piece and its count beside the board.
Click a piece in your pocket, then a legal empty board square to drop it.
Alternatively, press the **uppercase** piece letter (for example `N` or `P`),
move the cursor, and press enter/space. Uppercase pocket keys leave the usual
lowercase game shortcuts unchanged. Illegal destinations are ignored; escape
cancels pocket selection. Drops appear in the move list as `N@f3` (with `+` or
`#` for check or checkmate).

## Custom drop rules

Drop support is opt-in and independent of variant ids. A variant can inherit
orthodox chess and enable it with `drops: true`. Captures then add the captured
piece to the capturing side's hand, and a move may instead place one hand piece
on an empty square, subject to the same royal-safety check as any other move.
Dropped pieces count as moved; a `pawn: true` piece dropped on a rank
containing that side's setup pawns of the same type may use its initial-only
movement components. Drops are quiet moves, not quiescence captures (check
evasions still consider all legal drops). The computer values pocket material at 80% of its board value.

Piece definitions support:

- `drop_forbid_ranks: [1, 8]`: ranks on which this type may not be dropped,
  counted from the dropping side, starting at 1. An omitted list allows all
  ranks. This is a per-piece restriction, not an engine pawn special case.
- `drop_one_per_file: true`: only one unpromoted piece of this type may stand
  on a file, so the type cannot be dropped there. A promoted piece of the same
  type no longer counts, which is Shogi's nifu rule for pawns.
- `drop_no_mate: true`: a drop of this type may not deliver immediate
  checkmate. Every legal reply is tried, drops and interpositions included, so a
  drop that only looks like mate because the defender can block is allowed.
- `demote: P`: original type to use when reading a promoted piece marked `~`
  in FEN. Actual promotions remember the original piece type, so capturing a
  promoted piece returns that original type to the hand. Promotion does not
  change ordinary movement or royal-safety rules.

As with other inherited pieces, overriding a piece definition replaces its
whole definition, so repeat its movement and flags as well as drop settings.
`drops: false` disables inherited drop rules.

### Pocket FEN

Drop variants accept a bracketed hand after the placement field. Uppercase
letters belong to White, lowercase to Black, and repeated letters give counts:

```text
rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR[Qn] w KQkq - 0 1
```

Mark a promoted board piece with `~`, for example `Q~`, so it returns as the
configured `demote` type when captured. `Position.FEN()` writes this form for
drop variants, including an empty `[]` hand. Existing ordinary FEN placements
remain valid; non-drop variants do not acquire pockets or promoted-piece rules.

The remaining fields are side, castling rights, en passant target, halfmove
clock, and fullmove number. Standard FEN has only one en passant target: for a
custom pawn that crosses several squares, serialization retains only the first
crossed square. This limitation does not affect orthodox chess or Crazyhouse.
