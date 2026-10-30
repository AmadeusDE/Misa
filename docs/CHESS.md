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
