# Misa card games

Four offline games are available in **Games**, as shell commands, and as standalone
`misa` subcommands. They use the same factories and validators in every mode.

```sh
misa solitaire --draw 3
misa freecell --seed 42
misa spider --suits 4
misa hearts
```

## Options and replay

All four accept `--seed <uint64>`, `--theme classic|default`, `--ascii`, `--help`
and `--version`. The default theme is `classic`. Without a seed, a fresh seed is
obtained from the operating system's cryptographic random source; failure is
reported rather than silently choosing a predictable seed.

| Game | Rules option | Default |
| --- | --- | --- |
| Solitaire | `--draw 1|3` | 1, unlimited stock passes |
| FreeCell | none | eight columns, four cells; automatic foundation off |
| Spider Solitaire | `--suits 1|2|4` | 1 |
| Hearts | `--target <positive integer>` | 100 |

Hearts targets must fit the platform's signed integer with room for another
26-point hand. Invalid options/operands produce an error without starting a game.

The table and Help show `v1:<game>:<seed>:<rule-option>` (FreeCell's option is 0).
The table wraps long IDs instead of truncating them, reserving at least one
playable card row even at 24×14. Hearts shows phase instructions/results and the
last trick winner before score metadata. If status text exceeds the available
space, `? more status in Help` marks the omission; scroll Help for the full text.
Restart replays that deal; New obtains a fresh seed. Hearts restart resets the
scores and pass cycle too: it is explicitly a replay, not an undo. No deal is
silently replaced to make it easier. Random solitaire deals can be unwinnable.
These are **Misa seeds, not Microsoft's numbered deals**. Replay is guaranteed
within the v1 protocol, not across an unspecified future shuffle revision.

### Frozen v1 deal protocol

The standard deck is clubs, diamonds, spades, hearts; within each suit A through K.
Physical IDs are zero-based deck indices. Spider uses eight spade copies per rank
for one suit, four copies of clubs/diamonds for two suits, or two standard decks
for four suits, ordered copy, suit, rank. Every duplicate has its own physical ID.

Shuffle uses SplitMix64 with the seed as its initial state: add
`9e3779b97f4a7c15`, mix with `bf58476d1ce4e5b9` and `94d049bb133111eb`, with
right shifts 30, 27, 31. All operations are modulo 2^64. Descending Fisher–Yates
uses rejection sampling: for bound n discard outputs below `(−n) mod n`, then
take modulo n. Seed 42 starts a shuffled standard deck with physical IDs
`6,28,12,39,40,0,26,25`.

Klondike and Spider allocate consecutive shuffled chunks to columns left to
right; the last card of each chunk is face up. The remaining slice is the stock,
drawn from its end. FreeCell and Hearts deal the shuffled deck round-robin from
its beginning. Hearts hand number r (zero-based) shuffles with
`seed + r * 9e3779b97f4a7c15`, modulo 2^64.

## Controls and accessibility

* Left/right or Tab/Shift-Tab selects a pile; up/down selects a card within it.
* Enter/Space selects a source, then places on the focused destination. Escape
  cancels the source. Columns and long piles scroll to follow focus.
* Click cards or empty piles to select/place. The clickable `< > ^ v` controls
  navigate offscreen columns/cards without a keyboard.
* `n` / **N**: New; `r` / **R**: Restart; `?`: scrollable Help; `q` / **Q**: Quit.
  Abandoning play requires a Yes/No confirmation, also clickable.
* `u` / **U**: Undo; uppercase `U` or Ctrl-R / **Y**: Redo; `h` / **H**: Hint;
  `a` / **A**: explicit Auto. Undo/Redo/Auto do not reveal Hearts information.
* In Help, up/down scroll; Escape closes. Click Up/Down/Close works too.
* Hearts: toggle three cards in **You**, then select **Commit**. Select one legal
  hand card to play. After scoring, select **Next** to deal the next hand.

Cards carry rank and suit symbols independently of color; `--ascii` uses C/D/S/H.
Focus, selected sources and pass selections have separate textual markers as
well as styling. No fonts, sounds, images or runtime downloads are required.
The compact card table uses the existing terminal font and works in monochrome.
Below 24×14, the game shows a resize notice and does not accept board actions.
Resize preserves the deal and cancels a pending source selection safely.

All solitaire histories contain entire player actions, including automatic flips
and Spider completions. A new action clears redo. Undo restores the cards, moves,
score and stock order, but never rewinds elapsed real time. New/Restart reset
the clock. Completion is a quiet status message, with no flashing or sound.

## Rules and scoring

### Solitaire

Klondike deals seven columns of 1–7 cards, top cards exposed, and 24 stock cards.
Tableau sequences descend in alternating colors. Only kings or king-led runs
fill spaces. Foundations ascend A–K by suit; their top cards may return to the
tableau. Only the waste's top card is playable. Drawing three includes a short
final draw; recycling reverses the waste without shuffling. Both draw modes
allow unlimited stock passes.

Misa score is +10 for net foundation placement (−10 on return), +5 for each
new tableau exposure, and no stock/time penalty. This is not exact Windows or
Vegas scoring. Win requires all 52 foundation cards. Auto is offered only with
no stock or hidden tableau cards and commits only after verifying an entire
legal finish. Hints inspect masked visible piles, not hidden card identities;
they describe immediate moves, not solvability.

### FreeCell

All cards are exposed, dealt round-robin to eight columns (7,7,7,7,6,6,6,6).
Build down in alternating colors; any card fills an empty column. Cells each
hold one card. Suit foundations ascend A–K and are one-way. Sequence convenience
moves must be decomposable into legal single-card moves: capacity is
`(empty cells + 1) * 2^usable empty columns`, excluding the destination and not
counting the source as empty. The engine actually executes a decomposition on a
copy before committing, rather than accepting length alone.

There is no automatic foundation movement. Explicit Auto moves one legal,
conservatively safe card: aces/twos, or cards for which opposite-color
foundations have reached rank−1 and the other same-color suit rank−2. It is
undoable. Moves and time are shown; all 52 foundation cards are required to win.
Hints are not a full solver, and not every random deal is asserted solvable.

### Spider Solitaire

104 cards form ten columns (6,6,6,6,5,5,5,5,5,5), top cards exposed, and five
ten-card stock rows. Build down by rank regardless of suit; only same-suit
descending runs move together. Empty columns accept any movable card/run.
Dealing stock requires all ten columns occupied. Exposed suited K–A runs are
removed automatically, processing columns left to right and repeating all
resulting completions/flips. All eight completed runs are required to win.

Score begins at 500, subtracts 1 per player move or stock row, and adds 100 per
completed run. There is no zero clamp. Undo/redo restores the exact score and
includes the whole stock row and all triggered effects. Hidden identities are
masked before hints; blocked immediate play is not called a proven loss.

### Hearts

You are seat 0; seats 1–3 are independent computer opponents. Play is clockwise,
ace high, with no trumps. Each player receives 13 cards. Three-card passes rotate
left, right, across, hold. All four outgoing passes are selected against the
pre-pass hands and committed simultaneously.

The holder leads 2C. Follow suit whenever possible; highest led-suit card wins
and leads next. Hearts may not lead until a heart has been discarded, unless
the leader has only hearts. QS alone does not break hearts. On the first trick,
when void in the led suit, a player must avoid hearts/QS if a non-penalty card is
available; penalty-only hands may discard a penalty.

Hearts cost 1 each, QS costs 13. After all 13 tricks, a moon shooter receives 0
and everyone else 26. There is no diamond-jack bonus or alternate moon choice.
At the configured target, lowest total wins, with tied lowest players sharing
victory. The table shows totals, last trick winner, round breakdown and winners.

Bots use only detached observations of their own hand, own passes, public
plays, scores and suit-void deductions. Their decision API contains no seed,
deck order or opponents' hands. They choose legal passes, manage high-risk
cards, avoid penalties, track voids and try to interrupt public moon threats.
They do not share private memory. Computation is bounded by the fixed hand size
and runs in cancellable commands; owner/generation tags reject stale replies
after restart, pane closure or exit. There is no undo after revealing information.

## Rule references and original work

Consulted for rule facts on 2026-10-29, not for code, assets or copied prose:

* [Bicycle: Solitaire](https://bicyclecards.com/how-to-play/solitaire)
* [FreeCell: Rules and Supermoves](https://en.wikipedia.org/wiki/FreeCell)
* [Spider: Rules and Variants](https://en.wikipedia.org/wiki/Spider_(solitaire))
* [Pagat: Hearts](https://www.pagat.com/reverse/hearts.html)

The explicit variants above take precedence where references describe other
house rules. All code, help, fixtures, geometric backs and card presentation are
original Misa contributions under OSL-3.0. Rule facts and general visual ideas
are distinct from protected expression; this statement is not legal clearance.

`classic` is an original XP-era-inspired design: generic green felt, light cards,
blue title accents and compact gray controls. No Microsoft executables, source,
fonts, sounds, artwork, screenshots, logos or UI resources were used. Misa is
not affiliated with or endorsed by Microsoft. See [CREDITS.md](CREDITS.md).

## Verification and remaining release gates

Automated tests cover rule boundaries, pass directions, first-trick exceptions,
moon scoring/ties, completed bot matches, endgame fixtures, sequence capacity
and decomposition, physical-card conservation, masked hints/AI observations,
shuffle vectors/rejection sampling, atomic rejections, undo/redo, command flags,
menu/shell return paths, both builtin modes, resize bounds, pane closure and stale
computer replies. Layout tests include 80×24, 120×40, 32×18 and 24×14 grids.
Fuzz entry points exercise shuffle and solitaire action histories.
Known-deal SHA-256 vectors freeze complete physical IDs/ranks/suits for seeds 0,
42 and 18446744073709551615 in all games, both Klondike draw modes and all Spider
suit modes. The vectors were calculated independently from the documented v1
protocol; each test specifies its JSON checksum input. Klondike fuzz actions
encode draw/move, source, row and destination independently, with an exhaustive
encoding test covering every movable pile pair and card row.

Local check record (2026-10-29):

| Check | Result |
| --- | --- |
| `go test ./...` | passed |
| New cards/game packages with `-race` | passed |
| Default, `nough`, `nogui`, `noweb` vet and Misa builds | passed |
| OpenBSD, FreeBSD, macOS vet | passed |
| `cmd/ugh` and `cmd/ughbot` builds | passed |
| Shuffle and each solitaire action fuzzer, 5 seconds each | passed |
| All four standalone games: PTY startup and confirmed quit, 80×24 / 32×18 | passed |
| Formatting of tracked and new project Go files; `git diff --check` | passed |

The literal `gofmt -l .` also traverses an existing ignored
`.claude/worktrees/gnu-shell-tests` checkout and reports unrelated files there.
Those files were not modified. Build outputs and PTY transcripts were written
under `/tmp/opencode`, not added to the repository. PTY startup checks do not
constitute manually completed games or frontend screenshots.

Release acceptance is **not yet complete**. Actual manually finished games and
screenshots in a live terminal, Wayland GUI, browser and Android device have not
been recorded. Those environments must be marked **unverified**, not passed.
Android NDK builds/device CI, required hosted CI and review remain release gates.
There is no persistence, network multiplayer, full solver or compatibility claim
for another implementation's assets, numbered deals or every option.
