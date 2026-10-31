# Vi compatibility ledger — F01/F02/F03/F04/F05/F06

This is an incremental compatibility ledger for issue #67, not a claim of full Vim or Neovim
compatibility. The editor uses the same `Editor.Update` event path standalone and inside a Misa pane.

## Behavioral baseline and provenance

The reference profile is **Vim 9.1 defaults with `nocompatible`**, without user configuration.
Neovim differences are notes, not alternate expectations: notably Neovim defaults `Y` to `y$`,
whereas Vim defaults `Y` to `yy`. This task does not change `Y`.

Vim/Neovim documentation and black-box behavior are specifications only. The fixtures and helper
are original; no external editor source, runtime files, tests, or tutor passages were imported or
newly adapted. The implementation uses Vim motion documentation as a specification only. Optional
black-box checks of counted till repeats used Debian Vim 9.1.1230 (`-u NONE -U NONE -i NONE -N -n`,
default `cpoptions`); normal tests do not run it. F05 behavior references Vim 9.1 `nocompatible`
documentation for `undo`, `undo-blocks`, `U`, `:w`, and `modified` only. F06 uses the same
reference profile and documentation for `.`, `repeat.txt`, `r`, `R`, and Replace-mode Backspace
as specifications only; no external code was used. Existing Toybox/vis attribution in
`internal/vi/NOTICE` remains unchanged.

## Replay fixtures (F01)

`internal/vi/replay_test.go` defines `replayFixture`, `replayState`, `replayKeys`, and
`runReplayFixture`. Each replay writes an original input to a fresh temporary file, opens it with
`New`, and feeds `tea.KeyPressMsg` events to **`Editor.Update`**, not an internal normal-mode handler.
`New` does not load user configuration. Tests need no installed Vim, network, or global configuration.

Notation:

- Ordinary runes, including spaces and Unicode, are literal key/text events: `2d3w`, `ihello`,
  `:s/a/b/<CR>`.
- `<Esc>` is Escape, `<CR>` is Enter, `<BS>` is Backspace, and `<C-r>` is Ctrl-R.
- `<Up>`, `<Down>`, `<Left>`, and `<Right>` are arrow keys; `<lt>` is a literal `<`.
- Unknown or unterminated tokens fail the test rather than being silently reinterpreted.

A fixture asserts the complete logical buffer (a slice of lines without file terminators), zero-based
cursor `(row, col)` in runes, mode, unnamed register contents and linewise flag, dirty flag, and
`Editor.Quit`. Characterwise registers can contain embedded newlines. File newline/encoding fidelity
is separate F07 work; the `:wq` smoke test additionally asserts exact saved bytes. Standalone wrapper
coverage asserts the emitted `tea.QuitMsg`; mux coverage asserts editor exit returns to the same pane
without quitting Misa and preserves its register.

## Implemented coverage

| Scope | Original regression fixtures | Status |
| --- | --- | --- |
| F01: classic tutor lessons 1–2 skills | `hjkl`, `x`, `i`, `A`, `dw`, `d$`, `de`, `2w`, `3e`, `0`, `d2w`, `dd`, `2dd`, `u`, `U`, Ctrl-R | Seeded in `tutor_f01_test.go`; full-state line-delete expectations now pass, alongside narrower content/register checks. This is not full tutor conformance. |
| F01: safety and entry paths | Empty buffers/lines, empty-line forward search, cancellation, `:q` protection, `:q!`, `:wq`, standalone quit and embedded mux return | Tested; forward search handles empty lines without indexing past their end. |
| F02: counts and prefixes | `2d3w`, `5gg`, `2fx`, `3rx`, `3p`, `3P`, counted `D/C/S/s/J/x/X`, `"a2yy` then `"ap` | Tested in `count_replay_test.go` and focused state/safety tests. Counts survive prefixes; operator and motion counts multiply. `3J` joins three lines, not four. |
| F02: cancellation | Escape while `d`, `2"`, or `"a` is pending; invalid operators/prefixes; next-command isolation | Pending count, register, and operator state is discarded on cancellation or invalid input. |
| F02: count safety | Huge `x`, `dd`, multiplied counts, wrapping search and absolute line positions above 10000 | Overflow-safe, bounded by the limits below. The original exhausted-word and line-delete expectations now pass without changing their expected state. |
| F03: operator range normalization and motions | `operator_range_replay_test.go`, `operator_edges_replay_test.go`, `operator_entry_test.go` | Explicit exclusive/inclusive/linewise motion normalization for existing `d`, `c`, `y`, `<`, `>` operators; backward motions and no-ops; `dl`, `dh`, `d0`, `$`, counted `$`, `_`, counted `_`, `dj`/`dk` counts, `gg`/`G` linewise ranges (including operator-pending `gg`). |
| F03: word and line-end ranges | `operator_range_replay_test.go`, `operator_edges_replay_test.go` | `dw`/`de`/`db`/`cw` including whitespace and EOL/EOF, `cw` word-end behavior, `c$`, `cc`/counted `cc`, `y` word/end/`$`; sentence-motion fixtures cover exclusive motions landing at a later line's column zero. `w` as an operator's final step at EOL does not consume the newline, including counted operations and yanks. |
| F03: cursor and dirty-state edge cases | `operator_range_replay_test.go`, `operator_entry_test.go`, `internal/ui/editor_range_regression_test.go` | Preferred column after short lines and `$`; linewise `d`/`c` naturally mark the buffer dirty. The existing dirty-only skips are enabled and pass. Basic multiline change uses the existing single snapshot; broader F05 undo behavior is covered below. |
| F04: find/search motions | `find_f04_replay_test.go`, `search_f04_replay_test.go` | `f`/`t` are inclusive, `F`/`T` exclusive; `;`/`,` counts preserve/reverse direction; literal `/` and `?`, `n`/`N`, wrapping, operator composition and multiplied counts, empty-pattern prompt Enter (`/<CR>`) reuse, cancellation, failures, and empty-line safety. |
| F04: structural motions | `structural_f04_replay_test.go` | `%` matching pairs, fallback scan and inclusive operator ranges; `1..100%` linewise; `gg`/`G` linewise operators; `m[a-z]`, apostrophe linewise and backtick exclusive local marks. |
| F04: entry paths | `motion_entry_test.go`, `internal/ui/editor_motion_regression_test.go` | Standalone and mux motion entry/return coverage through the editor event path. |
| F05: tutor undo, `U`, Ctrl-R | `undo_f05_replay_test.go` | Covers undo/redo and line restore behavior, including no-op and changed-line cases; tutor 2.7 undo/U/Ctrl-R scenarios are covered without claiming full tutor parity. |
| F05: edit transactions and saved identity | `undo_f05_replay_test.go`, `undo_f05_transaction_test.go`, `undo_entry_test.go`, `internal/ui/editor_undo_regression_test.go` | Insert through Escape (including Backspace, newline, counts) and `cw` change form single undo units; clean quit after undo to the saved revision is checked standalone and in a mux pane. No-op/failing commands, yanks, and motions do not create history or clear redo; a committed real new edit clears redo. |
| F06: semantic dot repeat and count overrides | `repeat_operator_f06_test.go`, `repeat_interactions_f06_test.go`, `insert_replace_f06_test.go` | Repeats semantic operator/change, put, insert and Replace-mode edits; explicit dot counts override the stored count. Yank does not replace the repeatable change. A no-op does not replace the prior repeat. |
| F06: insert/Replace behavior and repeat transactions | `insert_replace_f06_test.go`, `repeat_entry_test.go`, `internal/ui/editor_repeat_regression_test.go` | Insert and Replace streams (including Backspace) repeat; `r<CR>` is supported. Each dot execution is one undo transaction; repeated insert/Replace cursor placement and standalone/mux entry paths are covered. |

### Tutor mapping (selected lessons only)

| Tutor lesson | Relevant operation | F06 ledger coverage |
| --- | --- | --- |
| 3.1 | Put (`p`/`P`) | Put behavior/counts are covered by F02; semantic put repeat by F06. |
| 3.2 | Replacement (`r`, `R`) | Character replacement and Replace mode, including `r<CR>` and dot repeat, are covered by F06. |
| 3.3 | Change (`c`) | Existing `cw`/change ranges and semantic dot repeat are covered by F03/F06. |
| 3.4 | `c` with motion | Existing change-motion ranges and semantic dot repeat are covered by F03/F06. |
| 6.1 | Open lines | Existing insert/open-line behavior is covered where exercised by insertion fixtures; no complete lesson-parity claim. |
| 6.2 | Append | `A` and insertion behavior are covered in F01/F06; no complete lesson-parity claim. |
| 6.3 | Replace mode | Replace stream, Backspace and semantic repeat are covered by F06. |
| 6.4 | Yank/put | Yank/register behavior is covered by F02/F03; yanks do not replace the dot repeat, and put is covered by F06. |

## Count safety policy

- Count parsing and operator × motion multiplication saturate at `999999999`, safely on 32-bit targets.
- Positional counts such as `G`/`gg` retain that count and clamp to the available buffer extent;
  they are **not** limited to 10000 lines.
- Non-wrapping motions/deletions bound their work by the buffer. Wrapping `n`/`N` searches skip
  complete cycles without changing the resulting counted position.
- Content-producing puts (`p`/`P`) and insert/Replace repetitions (including dot) are limited to
  **10000 repetitions**. This bounds the repetition multiplier, not total buffer bytes.

## Undo model and limitations (F05/F06)

Snapshot storage is retained and capped at **200 undo units**. The cap counts units, not bytes. A
saved revision that falls out of the retained history cannot cause another revision to be treated as
clean: dirty state compares the current revision identity with the saved identity, not undo-stack
length. Revision IDs allocated for edits are monotonic; undo/redo restore the prior revision identity.
A successful write updates the saved identity.

Nested `beginEdit`/`commitEdit`/`abortEdit` groups compound changes. Insert mode, through Escape,
is one unit (including Backspace, newline, and count repetitions); `cw` is one unit. Every dot
execution is one undo unit. Only a committed real edit clears redo. No-ops, failures, yanks, and motions
create no history and do not clear redo. A successful write inside an explicit transaction materializes
a saved revision without splitting its undo unit; further edits or aborting back to the earlier text
are still dirty relative to that write. `U` restores only the last-changed line while it remains
tracked; leaving that row discards the line-restore target, so moving away and back does not recover it.
A successful `U` itself creates a new undoable revision. Undo/redo place the cursor at the earliest
recorded mutation position (clamped in the restored buffer), falling back to the first differing
position for edits without recorded coordinates. Repeated-text insertions/deletions use the actual
mutation start, not the end of a common prefix. Intermediate edits later reversed within a transaction
can still determine this cursor position: `liX<BS><BS>az<Esc>u` on `abc` restores at column zero, even
though the final text difference started at column one. Full Vim deleted-line, indentation, and cursor
heuristics are not claimed. Arrow-driven insert undo breaks are not claimed.

## Deferred and skipped expectations

- **F03:** The range and motion coverage above is partial, not general Vim operator compatibility.
  WORD motions `W`/`E`, `cW`/`dE` are not implemented (H01); `gu`/`gU`/`g~` are not existing
  normal operators here (G06). Sentence-motion/operator coverage remains limited; F04 motions are
  covered above.
- **F04:** Regex and offset syntax, and command/search history (G01/T07), are unsupported. Search
  retains the existing literal-pattern and case-suffix subset; no `//` delimiter grammar is claimed
  (empty prompt Enter reuses the last pattern). `wrapscan`, `matchpairs`, and `cpoptions` are not
  configurable. Mark relocation/navigation enhancements (H10) and Visual parity (H07) remain deferred.
  Uppercase marks retain their existing same-buffer behavior; cross-file marks are not implemented.
- **F05:** Undo behavior is limited to the model and cases above; it is not full Vim undo parity.
  In particular, arrow-driven insert undo breaks and full Vim cursor/deleted-line/indent heuristics
  are not claimed.
- **F06:** Visual-mode and text-object dot repeat are deferred (H07/H04). Numbered-register
  auto-advance on dot (for example, `"1p` followed by `.` selecting `"2`) is TODO(H08). Macro behavior
  is unchanged and not certified. Arrow-driven input expansion and related behavior are unchanged; see
  F08. Dot repeats the last real change semantically, with an explicit count overriding the stored
  count; no-op commands and yanks do not replace that repeat. `r<CR>` is supported. Each dot execution
  is one undo transaction.
- **T05:** `:e`/`:e!` is not implemented. The reload-command fixture remains skipped as `TODO(T05)`;
  F05 tests the internal buffer-history reset boundary only. This reset establishes a fresh clean
  revision and clears undo, redo, and line-restore state; file loading/protection stays in T05.

`misatutor` already describes counted `gg`, operator-find, and `d%`; these existing examples use the
shared motion path. No tutor prose change is needed. This ledger does not claim complete tutor parity.
Broader macro and text-object compatibility remains deferred.
