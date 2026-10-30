# Chess

Play orthodox chess or a bundled/custom variant against the computer or another
person. By default the computer plays Black at **normal** level.

```sh
chess --level beginner
chess --level hard --think 5
chess double --level easy
chess fourplayer
chess --computer both --level expert
chess --computer none
chess --list
```

`--computer` accepts `white`, `black`, `both`, or `none`. One level applies to
every computer-controlled side in the two-player variants. Those levels support
custom fairy pieces; no external engine is used. Four-player Teams uses a
separate fixed ruleset and computer player described below.

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

## Four-player Teams

`chess fourplayer` starts a draft Teams game on a 14×14 cross, with each
3×3 corner removed. Red (bottom) and Yellow (top) are partners against Blue
(left) and Green (right); turns proceed Red, Blue, Yellow, Green. You play
Red and the computer plays the other three armies, including your partner.
The `--computer`, `--level` and `--think` settings do not change this mode;
`--fen` is not supported. The computer searches up to four plies with alpha-beta pruning. It evaluates material from the root team's perspective (Red + Yellow versus Blue + Green), with each team maximizing or minimizing on its turns. Checkmate scores are mate-like, favoring a win and preferring faster wins / slower losses; stalemate scores zero. The search has a 1,500-node budget across iterative deepening. If the budget interrupts an iteration, the computer keeps the best move from the last fully completed iteration; if none completed, it falls back to an ordered legal move. This is a shallow draft search, not a strong general-purpose opponent.

Each army starts with the usual eight pieces and eight pawns facing inward. Pieces move as in chess, cannot capture a partner, and only the moving player’s king must be safe. Either opposing army can give check. Pawns can advance two squares on their first move and automatically promote to a queen on the eighth rank from their own back rank. Castling moves the king two squares toward an unmoved rook; the path must be clear and the king may not castle out of, through, or into check. En passant is not implemented. Checkmate of the player whose turn it is ends the game in a win for the opposing team; stalemate ends it as a draw. Repetition and move-limit draws are not implemented.

This is a simplified draft, not a full implementation of every four-player chess ruleset. Its YAML entry selects the fixed Teams engine with `players: 4`; the board, armies and rules are hard-coded in Go, not configurable through YAML.

## During play

- Arrows or `h/j/k/l`: move the cursor; space/enter or click: select/move.
- `d`: cycle beginner → easy → normal → hard → expert → beginner.
- `c`: cycle computer sides; `u`/backspace: undo to a human turn.
- `n`: new game; `v`/`V`: next/previous variant.
- `f`: flip; `t`: toggle piece letters; `q`: quit.

In Teams, `c` and `d` do not change the fixed player or computer settings;
undo takes back moves to Red’s turn. The other listed navigation controls apply.

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
