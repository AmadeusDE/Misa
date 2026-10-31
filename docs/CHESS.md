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
(2020).

Custom variant YAML can set `stalemate` to `draw`, `win`, or `loss`. The default
when the field is omitted is `draw` (orthodox chess); `win` means the side
delivering stalemate wins, and `loss` means the side delivering it loses.

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
without `goal` has no goal squares: it is won only by checkmate and otherwise
ends in stalemate, repetition or the move limit. Grand Chess, for one, sets no
goal.

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

## Custom board dimensions

Variants support 1–702 files (labels `a` through `zz`) and 1–99 ranks.

Large boards can be expensive to load and play: reach tables, move generation,
and search consume more memory and time as the board grows.
