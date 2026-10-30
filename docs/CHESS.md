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

## During play

- Arrows or `h/j/k/l`: move the cursor; space/enter or click: select/move.
- `d`: cycle beginner → easy → normal → hard → expert → beginner.
- `c`: cycle computer sides; `u`/backspace: undo to a human turn.
- `n`: new game; `v`/`V`: next/previous variant.
- `f`: flip; `t`: toggle piece letters; `q`: quit.

The header shows, for example, `Chess · hard · Black to move · thinking…`.
Changing difficulty cancels the old search and starts the new policy without
applying obsolete results. The chosen level survives new games, variant
changes, and hotseat, but is not persisted after quitting. Analysis after a
computer move reports that move's score, including deliberate alternatives;
an unanalysed legal fallback is labelled instead of showing a fabricated score.

## Custom board dimensions

Variants support 1–702 files (labels `a` through `zz`) and 1–99 ranks.
Large boards can be expensive to load and play: reach tables, move generation,
and search consume more memory and time as the board grows.

## Bundled variants

- **Chess** (`chess`): orthodox chess on an 8×8 board.
- **Double Chess** (`double`): two armies per side on a 16×12 board.
- **Big Board** (`bigboard`): Betza's 16×16 fairy-piece variant.
- **Double Big Board** (`doublebig`): two armies per side on a 32×16 board.
- **Crazyhouse** (`crazyhouse`): orthodox chess with pockets and drops. Captures
  change a piece's colour and put it in the capturer's pocket; captured promoted
  pieces return as pawns. Instead of moving, drop a pocket piece onto an empty
  square. Pawns cannot be dropped on rank 1 or 8. Drops must leave your king safe,
  but may give check or checkmate. Castling, en passant, promotion, and stalemate
  draws otherwise work as in chess. A pawn dropped on its starting rank can make
  the usual initial double step; a dropped rook cannot restore castling rights.
  Rules: [Lichess](https://lichess.org/variant/crazyhouse) and
  [Wikipedia](https://en.wikipedia.org/wiki/Crazyhouse).

### Playing with pockets

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
