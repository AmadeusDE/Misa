# Chess difficulty: partial calibration

**Draft evidence, not release calibration.** The implementation provides five
resource/selection profiles, but their relative playing strength is not yet
established. At the request of the author, the initial campaign was shortened
to a few-minute sample before opening a draft PR. No Elo labels are assigned.

## Corpus and baseline

`internal/chess/testdata/difficulty/positions.jsonl` contains 100 authored,
deterministically generated orthodox positions, represented as UCI moves from
the canonical start position. No third-party position collection was used.
Positions are generated from the embedded `Builtins()` variant, never `Load()`
or user YAML. The temporary driver does not accept a variant override.

Generation used Go `math/rand/v2` PCG seeds `(i+1, 6381)` for sample `i=0..99`.
Target plies were `[4,12,24,48,90][i/20] + i%2`. Each ply selected a uniform
legal move; after ply 24, the first capture passing a one-in-three random test
replaced it. Generation stopped when the game ended. The retained move lists,
not regeneration across future move-order changes, define the corpus. This is
a broad legal-position smoke corpus, **not a curated tactical/endgame suite**.
One position is terminal and is retained rather than silently removed.

Even sample IDs are tuning; odd IDs are held out. The split was fixed before
choosing profile constants or collecting tier results. No statistical tuning
was attempted in this partial campaign. Weak policies remain provisional at
4,000/20,000 nodes, depth 1/2, and 35%/15% alternative-selection probabilities
within 200/75 cp respectively. See [CHESS.md](CHESS.md) for all profile settings.

`baseline.jsonl` captures parent revision
`c7ed5251333b05a3e0782c2d530151ff0360c5b5` at depth 4 with a 3-second safety
ceiling, using a fresh engine per position and the complete preceding history.
Every baseline search finished without hitting that time ceiling. The held-out
regression test compares move, score, depth, node count, and terminal status
exactly with the parent. This passes, but is **not** a paired-match or
time-based no-regression claim.

## Initial experiment

The `internal/chess/testdata/difficulty/` evidence directory retains raw JSONL results (gzip where appropriate),
reference scores, supported Stockfish options, checksums, source fingerprints,
and a machine-readable summary. Latency measurements used an AMD Ryzen 5 1600
(6 cores/12 threads), Linux amd64, Go 1.27.1, while other verification workloads
were running. They are illustrative, not controlled idle-system benchmarks.

- Deterministic runs: all 100 positions; seeds 0–9 per beginner/easy position,
  seed 0 for other levels. Fresh engine state for every sample. Normal/hard/
  expert used node limits 30,000/120,000/480,000 and a one-hour safety ceiling;
  weak levels retained their profile depth/node caps. These strong-level node
  limits are experimental controls, **not shipped profile settings**.
- Shipped time-based smoke sample: IDs 000, 001, 020, 021, 040, 041, 060, 061,
  080, 081, covering early, middle, and late generated positions. Same seed
  policy, fresh engine state, actual default time ceilings.
- External reference: the same 10-position subset, comparing unrestricted root
  analysis with `searchmoves <chosen-move>` root analysis, from the same side's
  perspective, at equal budgets. Mate scores are separate from centipawns.
  Negative apparent loss is reanalysed rather than credited as superior play.

The short reference run used the official **Stockfish 19** Linux x86-64
universal executable, downloaded independently into `/tmp/opencode`. Exact
engine identification, SHA-256, supported options and settings are recorded in
`reference-metadata.json`. One thread, 32 MiB hash, full strength, no pondering,
and empty Syzygy path were used. Reference node budgets were 10,000, retried at
40,000 and 160,000 for negative loss, mixed cp/mate outcomes or bounds. Hash was
cleared and `ucinewgame`/readiness synchronization performed between independent
analyses. All root move lists were independently validated with python-chess
1.11.2; castling, en passant and promotion UCI forms were checked separately.

The temporary Python driver used direct process execution via
`SimpleEngine.popen_uci([binary], timeout=15)`, its UCI/readiness handshakes,
bounded searches, output draining, and `quit`/`close` cleanup. The one-off Go and
Python drivers are **outside the repository** and are not maintained test tools.
Exact invocations used (temporary paths are local to this implementation):

```sh
/tmp/opencode/chess-baseline corpus > internal/chess/testdata/difficulty/positions.jsonl
/tmp/opencode/chess-baseline baseline internal/chess/testdata/difficulty/positions.jsonl > internal/chess/testdata/difficulty/baseline.jsonl
/tmp/opencode/chess-tiers-final deterministic internal/chess/testdata/difficulty/positions.jsonl > /tmp/opencode/chess-deterministic-final.jsonl
/tmp/opencode/chess-tiers-final shipped /tmp/opencode/chess-shipped-subset.jsonl > /tmp/opencode/chess-shipped-final.jsonl
/tmp/opencode/chess-python/bin/python /tmp/opencode/chess-reference-partial.py /tmp/opencode/chess-partial-samples.jsonl /tmp/opencode/chess-partial
```

An initial larger reference attempt (50,000/200,000/800,000 nodes) was stopped
to respect the shortened campaign. Its incomplete output is not included in
the summary. Full UCI transcripts, including that interrupted attempt, remain
under `/tmp/opencode/chess-*-uci.log`; the completed short attempt had no
handshake/search failures. The reference results describe `(position, move)`
pairs, independently of the Misa sample that first requested them. Moves not
covered by the retained reference sample are explicitly counted as unmeasured.

Stockfish was used **only as an external, one-time verifier**. No Stockfish code,
network, weights, executable, UCI client, test dependency or CI integration is
part of this change. Ordinary tests exercise Misa directly.

## Interpretation

The shipped-budget sample produced the following results (9 nonterminal
positions; 10 seeds each for weak levels). Latencies are medians in seconds;
cp columns include only measured, exact, non-mate, nonnegative comparisons.

| Level | Median depth | Median nodes | Median seconds | cp mean / median / p90 | cp samples (distinct positions) |
| --- | ---: | ---: | ---: | --- | ---: |
| beginner | 1 | 137 | 0.003 | 57.2 / 34 / 123 | 51 (7) |
| easy | 2 | 1,769 | 0.006 | 32.4 / 34 / 67 | 41 (5) |
| normal | 10 | 1,174,087 | 1.976 | 45.5 / 7.5 / 165 | 4 (4) |
| hard | 11 | 1,832,620 | 3.994 | 45.5 / 7.5 / 165 | 4 (4) |
| expert | 11 | 4,107,477 | 9.175 | 45.5 / 7.5 / 165 | 4 (4) |

The small sample cannot establish adjacent-tier ordering. Each of normal/hard
and hard/expert selected identical moves in 8 of 9 nonterminal positions, and
their measured cp losses were identical. Several constrained/unrestricted reference
comparisons still disagree after retries; those are excluded as uncertain, not
clamped to zero or converted from mate to centipawns. Because exclusions can
differ by level, aggregate cp losses need not use identical position cohorts.
Weak-level seeds are repeated samples of the same positions, not independent
evidence of hundreds of distinct tactical tests.

Two fresh-engine deterministic runs reproduced all 2,300 results exactly
(move, score, depth, nodes, found), excluding latency. On the full deterministic
corpus, median depths were 1/2/5/7/9 and median nodes were
218/2,191/30,000/120,000/480,000, in tier order. These demonstrate resource
separation, not statistically demonstrated playing-strength separation.

Mean/median/p90 loss and latency/depth/node distributions are in `summary.json`.
Mate outcomes, uncertain references, terminal samples and missing references
are counted separately. There is no labelled tactical solve-rate claim from
this uncurated corpus. Unit tests do cover mates, evasions, forced moves,
promotion, castling, en passant and repetition, plus bounded self-play on
`double`, `bigboard`, and a custom cannon/nightrider fixture.

## Required before making the PR ready

- [ ] Add a curated, provenance-recorded orthodox suite with tactical labels,
  quiet play, representative endgames, forced mates and repetition histories.
- [ ] Resolve uncertain references at higher budgets and assess all held-out
  positions, with at least 10 seeds for weak profiles and controlled/repeated
  shipped-budget timing runs. Report tactical solves and missed/allowed mates.
- [ ] Run at least 50 identical opening positions with colors reversed for
  each adjacent pair (100 games per pair), recording seeds, hardware and time
  controls; report opening-pair-clustered confidence intervals. Extend samples
  until stronger-tier score lower bounds exceed 50%, or retune/merge tiers.
- [ ] Run parent-vs-normal paired matches and shipped-time baseline comparisons.
- [ ] Run color-swapped games against a fixed Stockfish configuration as an
  external smoke check. No such games were run in this partial campaign.
- [ ] Investigate match/quality disagreements; freeze profile constants only
  after calibration, and document limitations without Elo claims.
- [ ] Complete Android NDK/cgo, APK and emulator CI checks (SDK/NDK unavailable
  locally). Pure-Go Android arm64 vet passes.

## Local verification

Passed: focused chess/builtin tests; all repository tests; chess race tests;
the CI terminal/web/UI/ugh race selection; `go vet ./...`; module verification;
all native binary builds; nough/nogui/noweb and combined-tag builds; tagged vet;
OpenBSD/FreeBSD/Darwin vet; pure-Go Android arm64 vet. The held-out parent
baseline is checked by `TestNormalHeldOutBaseline` with no external engine.

`gofmt -l .` also descends into a pre-existing, ignored
`.claude/worktrees/gnu-shell-tests` tree and reports unrelated formatting there.
Those files were left untouched. Tracked repository files and new Go files for
this change are formatted, matching the CI file selection. `git diff --check`
passes.
