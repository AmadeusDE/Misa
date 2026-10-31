# Vi compatibility ledger — F01/F02

This is an incremental compatibility ledger for issue #67, not a claim of full Vim or Neovim
compatibility. The editor uses the same `Editor.Update` event path standalone and inside a Misa pane.

## Behavioral baseline and provenance

The reference profile is **Vim 9.1 defaults with `nocompatible`**, without user configuration.
Neovim differences are notes, not alternate expectations: notably Neovim defaults `Y` to `y$`,
whereas Vim defaults `Y` to `yy`. This task does not change `Y`.

Vim/Neovim documentation and black-box behavior are specifications only. The fixtures and helper
are original; no external editor source, runtime files, tests, or tutor passages were imported or
newly adapted. Existing Toybox/vis attribution in `internal/vi/NOTICE` remains unchanged.

## Replay fixtures (F01)

`internal/vi/replay_test.go` defines `replayFixture`, `replayState`, `replayKeys`, and
`runReplayFixture`. Each replay writes an original input to a fresh temporary file, opens it with
`New`, and feeds `tea.KeyPressMsg` events to **`Editor.Update`**, not an internal normal-mode handler.
`New` does not load user configuration. Tests need no installed Vim, network, or global configuration.

Notation:

- Ordinary runes, including spaces and Unicode, are literal key/text events: `2d3w`, `ihello`,
  `:s/a/b/<CR>`.
- `<Esc>` is Escape, `<CR>` is Enter, and `<C-r>` is Ctrl-R.
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
| F01: classic tutor lessons 1–2 skills | `hjkl`, `x`, `i`, `A`, `dw`, `d$`, `de`, `2w`, `3e`, `0`, `d2w`, `dd`, `2dd`, `u`, `U`, Ctrl-R | Seeded in `tutor_f01_test.go`; full-state line-delete expectations are skipped for F05, with narrower content/register checks still running. This is not full tutor conformance. |
| F01: safety and entry paths | Empty buffers/lines, empty-line forward search, cancellation, `:q` protection, `:q!`, `:wq`, standalone quit and embedded mux return | Tested; forward search now bounds its start slice on an empty line. |
| F02: counts and prefixes | `2d3w`, `5gg`, `2fx`, `3rx`, `3p`, `3P`, counted `D/C/S/s/J/x/X`, `"a2yy` then `"ap` | Tested in `count_replay_test.go` and focused state/safety tests. Counts survive prefixes; operator and motion counts multiply. `3J` joins three lines, not four. |
| F02: cancellation | Escape while `d`, `2"`, or `"a` is pending; invalid operators/prefixes; next-command isolation | Pending count, register, and operator state is discarded on cancellation or invalid input. |
| F02: count safety | Huge `x`, `dd`, multiplied counts, wrapping search and absolute line positions above 10000 | Overflow-safe, bounded by the limits below. Existing F03/F05 failures remain explicit skips rather than changed expectations. |

## Count safety policy

- Count parsing and operator × motion multiplication saturate at `999999999`, safely on 32-bit targets.
- Positional counts such as `G`/`gg` retain that count and clamp to the available buffer extent;
  they are **not** limited to 10000 lines.
- Non-wrapping motions/deletions bound their work by the buffer. Wrapping `n`/`N` searches skip
  complete cycles without changing the resulting counted position.
- Content-producing puts (`p`/`P`) and insert repetitions are limited to **10000 repetitions**.
  This bounds the repetition multiplier, not total buffer bytes.

## Deferred and skipped expectations

- **F03:** exhausted `dw` at EOF can leave the final character (including huge multiplied counts).
  Inclusive/exclusive operator ranges and general counted `$` remain separate work.
- **F04:** find/search/bracket/prefix motions as operators are not claimed by standalone `2fx` coverage.
  `dfx` has a skipped full-state composition fixture.
- **F05:** linewise deletion currently leaves `modified` false. Full-state `dd`, `2dd`, empty-line `dd`,
  and huge `dd` fixtures retain their correct expectations with `TODO(F05)` skips. Undo transaction
  and saved-state correctness are not claimed by the passing basic `u`/`U`/Ctrl-R sequences.
- **F06:** dot count overrides and semantic repeat of changes/puts are not implemented here.
  Skipped fixtures record dot losing a put count and counted insert leaving the cursor at the first
  repetition instead of the last. Narrower content-growth coverage still runs. Macro composition/
  recording and counted macro replay (`@`) are unchanged and not certified by this fixture subset.

`misatutor` already describes counted `gg`, edits, and insert commands; these changes make those
existing count claims more accurate and require no new tutor prose. Its wider operator/macro claims
remain outside this PR's compatibility claim.
