# Vi compatibility ledger — F01/F02/F03/F04/F05/F06/F07/F08/F09/G01/T01/T02/T03/T04/T05/H01/H02/H04/H05/H03

This is an incremental compatibility ledger for issue #67, not a claim of full Vim or Neovim
compatibility. The editor uses the same `Editor.Update` event path standalone and inside a Misa pane.

## Behavioral baseline and provenance

The reference profile is **Vim 9.1 defaults with `nocompatible`**, without user configuration.
For F07 file round trips the specification profile additionally uses `nofixendofline`:
Misa preserves a missing final newline rather than applying Vim's default `fixendofline`.
This describes the reference profile, not an implemented option command.
Neovim differences are notes, not alternate expectations: notably Neovim defaults `Y` to `y$`,
whereas Vim defaults `Y` to `yy`. This task does not change `Y`.

Vim/Neovim documentation and black-box behavior are specifications only. The fixtures and helper
are original; no external editor source, runtime files, tests, or tutor passages were imported or
newly adapted. The implementation uses Vim motion documentation as a specification only. Optional
black-box checks of counted till repeats used Debian Vim 9.1.1230 (`-u NONE -U NONE -i NONE -N -n`,
default `cpoptions`); normal tests do not run it. F05 behavior references Vim 9.1 `nocompatible`
documentation for `undo`, `undo-blocks`, `U`, `:w`, and `modified` only. F06 uses the same
reference profile and documentation for `.`, `repeat.txt`, `r`, `R`, and Replace-mode Backspace
as specifications only; no external code was used. F07 likewise uses Vim 9.1 documentation for
`fileformat`, `endofline`, `fixendofline`, `:w`, and `backupcopy` as specifications only, with
`nofixendofline` as noted above. Existing Toybox/vis attribution in
`internal/vi/NOTICE` remains unchanged. T02 uses the Vim 9.1 `nocompatible` defaults,
including default `cpoptions` A/F, as a behavioral specification only; no external source was used.
The explicit-range saved-state rule below is this task’s contract, not a general Vim parity claim.

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
`Editor.Quit`. Characterwise registers can contain embedded newlines. F07 covers file newline handling;
the `:wq` smoke test additionally asserts exact saved bytes. Standalone wrapper
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
| F07: file safety (done) | `file*_f07_test.go`, `internal/ui/editor_file_f07_test.go`, `internal/ui/editor_file_regression_test.go` | Load errors, newline preservation, safe writes and external-change guards; scope and limitations below. |
| T02: range writes and file identity (done) | Original `*t02*` regression fixtures | Numeric and Visual range writes, append, write identity, save-as and rename semantics; scope and residual deferrals below. |
| T03: shell lifecycle and reads (done) | Original `shell*_test.go` and `internal/ui/editor_shell_regression_test.go` fixtures | Async output/cancellation, shared cwd/environment, addressed file/shell reads and failure-safe filters; limits below. |
| T04: search options and highlighting (done) | `case_t04_replay_test.go`, `options_t04_test.go`, `highlight_view_test.go`, `search_options_entry_t04_test.go`, `internal/ui/editor_search_options_t04_test.go` | `ignorecase`/`noignorecase` (`ic`/`noic`) affect search and substitution; `smartcase` defaults uppercase patterns to case-sensitive; `hlsearch` and `incsearch` highlighting/preview behavior is covered through editor rendering and entry paths. `:set` parsing and option reporting are narrower than full Vim (details below). |
| T05: edit another file | Original `edit*_t05_test.go`, `edit_edges_t05_test.go`, `edit_policy_t05_test.go`, reload replay fixture, and `internal/ui/editor_edit_regression_test.go` | File switching and reload contract below; automated fixtures are being added, not a claim of manual verification. |
| T01: substitute confirmation (done) | `substitute_confirm_test.go`, `substitute_confirm_replay_test.go`, `substitute_confirm_edges_test.go`, `substitute_entry_test.go`, `internal/ui/editor_substitute_regression_test.go` | Public-event fixtures for decisions, ranges, global matches, Unicode, cursor/highlight/prompt, counts, undo/redo, no-op history, dot/register preservation, macro playback, and standalone/mux resize and return. |
| F09: bounded viewport and display cells (done) | `internal/vi/layout_test.go`, `display_f09_test.go`, `internal/ui/editor_display_regression_test.go` | One line-to-cells layout (`layout.go`) behind rendering, cursor, Visual/confirm highlights and `j`/`k`; scope below. |
| G01: Vim pattern subset (done) | `internal/vi/vipat/*_test.go`, `pattern_g01_replay_test.go`, `pattern_entry_g01_test.go`, `substitute_line_test.go`, `internal/ui/editor_pattern_regression_test.go` | One parser/matcher (`vipat`) behind `/`, `?`, `n`/`N`, operator searches, `:s` (including `c`), `:g` and `:v`; scope below. |
| H01: WORD motions | `word_h01_replay_test.go` | `W`/`B`/`E` move by whitespace-delimited WORDs; operator forms use the same motion path, including `dW`/`cW`/`dE`. Counts and line-boundary behavior are covered. |
| H02: word searches | `word_search_h02_replay_test.go` | `*`/`#` search forward/backward for a whole keyword; `g*`/`g#` also match within words. Punctuation is searched literally. Normal, Visual and operator searches use shared search state, including counts, wrapping, `n`/`N`, options and highlighting. |
| H04: word text objects | `word_object_h04_replay_test.go` | Public-event fixtures cover `d`/`c`/`y` with `iw`/`aw`/`iW`/`aW`, operator × object counts, whitespace, tabs, punctuation, trailing whitespace, missing final newline, single- and multiline objects, empty lines, failed counts, named registers, Visual selection and repeated Visual extension, `ciw` followed by dot, undo/redo, a multiline register round trip, and the `!` filter-prompt handoff. These examples are not a claim of complete text-object or Visual parity. |
| H05: paired and quoted text objects | `pair_object_h05_replay_test.go` | `i(`/`a(`/`ib`/`ab`, `i[`/`a[`, `i{`/`a{`/`iB`/`aB`, `i<`/`a<` (either delimiter works as the key) and `i"`/`a"`, `i'`/`a'`, ``i` ``/``a` `` with `d`/`c`/`y`/`>`/`<`/`!` and Visual mode. Fixtures cover nesting, counts, multiline pairs, empty objects, cursor on a delimiter, unmatched delimiters, escapes, Visual growth, registers, dot repeat and undo. |
| H03: sentence, paragraph and structural motions | `structural_motion_h03_replay_test.go` | `{ }`, `( )`, `[[ ]] [] ][`, `[( [{ ]) ]}`, counted `H M L` and counted `|` (goal column kept by `j`/`k`), standalone and under `d`/`c`/`y`, with counts, exclusive-to-linewise rules and the jump mark. Expectations come from Vim 9.2 (`-u NONE -N -es`) used as a black-box oracle; a differential fuzz (900 cases per motion family) matched. |

### H04 word-object limits

`iw`, `aw`, `iW`, and `aW` share one object scanner for operator and Visual dispatch.
`d`, `c`, and `y` route to characterwise and linewise operations; `>` and `<` route to the
existing indent operators, and `!` opens the existing range-filter prompt without creating an
edit. The fixtures assert the `d`/`c`/`y` and `!` paths; no dedicated H04 fixture pins `>iw`
or `<aw` results. Paired and quoted objects are H05 (below); prose and tag objects remain H06; broader Visual
parity is H07, and numbered-register or Visual dot-repeat behavior is not claimed beyond the
single `ciw`-then-dot case above.

Known divergence: a counted object that reaches an empty line stops there and treats the empty
line as satisfying the count. Vim instead fails some counted `aw` objects in that situation, so
for example `3daw` over a two-word buffer that contains an empty line covers the buffer in Misa
while Vim leaves it unchanged. Aligning counted `aw` at empty lines with Vim is a follow-up; the
editor does not emulate Vim internals to get there.

H04 expectations were checked against Vim 9.0.1378 (`vim.tiny`, `-Nu NONE`, `nocompatible`
defaults) for the same keystrokes. That is the reference for these objects; it does not revise
the project-wide baseline elsewhere in this ledger. No Vim is required to run the fixtures, and
no external editor code or prose was copied.

### H05 paired and quoted object limits

Pairs and quotes share the H04 dispatch (`isTextObjectKey`, `executeObjectOperator`, the Visual
`i`/`a` prefix). Scanners are `pair_object.go` and `quote_object.go`; the dispatch is
`delim_object.go`. Neither needs a language parser.

Behaviour that matches Vim: a cursor outside every pair uses the next pair after it (across lines; a count walks inwards, and a closing delimiter or unmatched opener found first fails with no change and no bell, since Misa has none); a count picks the n-th enclosing pair; a cursor on either delimiter
uses that pair; `i(` on a pair whose delimiters sit on their own lines is linewise, and is
charwise when text shares a line with a delimiter; an empty pair makes `di(`/`yi(` a no-op (the
cursor moves between the delimiters) and `ci(` inserts there; quotes are line-local, a quote
preceded by an odd number of backslashes is ordinary text, a cursor before the first quote uses
the next string, a cursor between two strings selects the gap between the closest quotes on each
side, and a cursor on a quote pairs the quotes of the line from its start; `2i"` includes the
quotes; `a"` takes trailing white space, else leading; repeated Visual `i(`/`a(`/`i"` grow
outwards (the new object must be strictly larger than the selection, and a one-character
selection counts as no selection, so `vi(i(` on `((a))` reselects `a`); Visual `i(` on a
multiline pair includes the final line break.

Known divergences:

- Delimiters are counted by kind only. Brackets inside quotes or comments count, so `f("a)", b)`
  with the cursor on `b` fails where Vim's quote heuristics may skip the `)`.
- A failed command does not discard the keys typed after it: a script or `.` replay continues,
  and the keys that follow run as ordinary commands (Vim aborts the remainder of a typeahead
  sequence). This is how every Misa operator failure already behaves.
- A Visual `i(`/`i"` that finds nothing larger (or an empty pair) leaves the selection unchanged.
- `>i(` on an empty pair indents nothing, and `<`/`>` over a charwise multi-line object shift
  every line the object touches.
- Fixtures were generated from Vim 9.2 run as a black-box oracle (`vim -u NONE -N -es`); the
  failure and Visual cases that depend on keys continuing after a failure are written by hand. No Vim source,
  documentation text or tests were read or copied.

### F08 input and paste contract

Normal-mode paste inserts literal text at the cursor and returns to Normal mode as one undoable
change; it does not replace dot or record macro keystrokes. Insert/Replace paste is literal, normalizes
CR and CRLF to LF, and participates in insert undo and dot repeat. Command/search paste flattens
newlines to spaces and removes other control characters without submitting. Visual, confirmation and
single-character Replace paste are ignored. Multi-rune IME commits are processed in order. Typed
and replayed keys share mode dispatch; terminal Ctrl-I and Tab are indistinguishable. Full macro
recording/playback remains H09, and broader command-line editing remains T07.

Original coverage: `internal/vi/input_f08_test.go` and `internal/ui/editor_input_f08_test.go`
exercise public event routing, literal paste, repeat, undo and embedded mux delivery. Vim v9.1.0000
is the behavioral reference, not a claim of complete paste or input-method compatibility.

### F07 file round trips and safety

Opening a nonexistent path (ENOENT) creates a `[New]` buffer. Other read errors are displayed,
and a normal `:w` is refused after a read failure unless forced with `:w!`. All-CRLF input uses
DOS line endings; mixed line endings use Unix format and retain the CR characters as text. The
final-newline state and empty files are preserved; new buffers default to a final newline. This
is existing UTF-8 text handling, not a promise to round-trip arbitrary binary data. No
`fileformat` or `endofline` option commands are added.

Normal regular files are saved by a same-directory temporary file and rename, preserving modes.
A file with no write permission bits requires `:w!`; forcing does not bypass OS permission checks.
Symlinks, hardlinked files and files with a different owner or group are written in place to preserve inode metadata;
these writes are **not atomic**. A snapshot of modification time, size and file identity detects
external changes and requires a forced write (`:w!`, `:wq!` or `:x!`) to override the guard.
A failed save retains the dirty state and saved revision. These guards are not a substitute for
coordination with other writers: same-inode edits restoring both size and mtime are undetectable,
and another writer can race the check. Atomic rename is not a crash-recovery or durability guarantee;
ACLs and extended attributes are not preserved by replacement writes.

### G01 Vim pattern subset

Search, `:s` and `:g`/`:v` share `internal/vi/vipat`, a Vim-pattern parser and matcher written for
this editor. Go `regexp` syntax is never exposed. Matching is backtracking, leftmost-first (Vim's
semantics, not POSIX longest), one line at a time.

Supported (default `magic`): literals; `.`; `*`, `\+`, `\=`, `\?`, `\{n,m}` (`\{n}`, `\{n,}`,
`\{,m}`, `\{}`) and non-greedy `\{-n,m}`; `^` (only at the start of the pattern or after `\(`, `\%(`,
`\|`) and `$` (only at the end or before `\|`, `\)`), otherwise literal; a leading `*` is literal;
`\(\)`, `\%(\)`, `\|`; collections `[...]` with ranges, `^` negation, `[:alpha:]`-style classes and
`\e \t \r \b \\ \] \^ \-` (an unterminated `[` is a literal, as in Vim); `\s \S \d \D \w \W \a \A \l \L
\u \U \x \X \o \O \h \H` (ASCII); `\<`, `\>` (word characters are letters, digits and `_`, Unicode
aware); `\t \e \r \b`; the pattern atoms `\c`/`\C` anywhere; and the magic modes `\v`, `\m`, `\M`,
`\V` switching mid-pattern. Backward searches read `\?` as a literal `?`.

Rejected with `Unsupported pattern construct: <construct>` (never treated as literals), each deferred to a
separately scoped follow-up: `\zs`/`\ze`, `\&`, `\@=` `\@!` `\@<=` `\@<!` `\@>` (lookaround), `\1`-`\9`
backreferences, `\n` and `\_x` (multiline), `~` (last substitute string; write `\~` for a tilde), `\%[`,
`\%d`/`\%x`/`\%u` and the other `\%` atoms, `\i \I \k \K \f \F \p \P` (option-dependent classes),
`[[=a=]]`, `[[.a.]]`, `\d123` codes and `\n` inside collections, and unknown alphanumeric escapes. Syntax
errors use Vim's numbers: `E54`, `E55`, `E61`, `E64`, `E554`, `E944`. Matching is bounded: a pattern that
backtracks past the step budget fails with `E363: Pattern too complex` instead of hanging.

Wiring: an empty pattern in `/`, `?`, `:s` and `:g` reuses the last pattern (`E35` when there is none for
`:s`/`:g`); a pattern that compiles becomes the last search pattern, including from `:s` and `:g`, a
rejected one does not, and a rejected search also abandons a pending operator. `:s` and its confirmation
use Vim's zero-width rules: `:s/x*/-/g` turns `abc` into `-a-b-c` and `:s/b*/-/g` into `-a-c`. A match at
the end of a non-empty line puts the cursor on its last character, so `/$` then `n` advances. Match columns
are rune offsets on multi-byte lines.

Behaviour changes: `.`, `*`, `[`, `\(`, `\)` and the other magic characters were literal text (or ad-hoc
`\(` handling) in `/`, `:s` and `:g` and are now pattern syntax; `~` and the constructs above now report
errors; `:s//x/` and `:s//x/c` reuse the last pattern instead of replacing at every position or being
rejected; `:s` and `:g` set the last search pattern. `:g/pat/cmd` previously cut the command at its next
delimiter, so `:g/x/s/a/b/` never ran the substitute; the command is now everything after the pattern.

Left to other tasks: the replacement grammar `&`, `\1`, `~`, case conversion, `:&`, `g&` (G02), Ex addresses and `:g` execution order/register order (G03: `:g/pat/d` still runs bottom-up), search offsets (`/pat/e`) and history (T07). Vim 9.1 `nocompatible` `:help pattern` is used as specification only; no Vim, Neovim or regex-library code is used.

### T04 search options

`:set` recognizes `ignorecase`/`noignorecase` (`ic`/`noic`), `smartcase`/`nosmartcase`,
`hlsearch`/`nohlsearch`, and `incsearch`/`noincsearch`. The case options govern search and
substitution: ignorecase folds by default, while smartcase makes a pattern containing uppercase
characters case-sensitive; `\c` and `\C` explicitly override the option-derived default. Incremental
search previews the current match as pattern input changes, and Escape restores the prior cursor.
Highlight search marks current-pattern matches in the text view. Rendering uses the shared display-cell
layout, so match spans cover tabs and wide/combining clusters.

Option implementation remains partial: only `ignorecase` is retained as editor state and listed by
bare `:set`; `smartcase`, `hlsearch`, and `incsearch` are accepted but their option values are not
reported there. Their full Vim option/query semantics are not claimed. Existing tutor instructions
already describe `\c`/`\C` and `:set ic`/`:set noic`; they remain correct. Tutor text does not cover
`smartcase`, highlighting, or incremental preview, and was left unchanged.


### F09 Display cells and the bounded viewport

`layout.go` turns a line into per-rune spans (display column, width) so a rune index and a display
column convert both ways. Later work (help window T06, line numbers H11, Visual block G04,
`wrap`/`listchars` D07) should build on `newLayout` and `lineLayout.window` rather than counting runes.
Policies:

- Width comes from `rivo/uniseg` grapheme clusters, the same measure lipgloss and `x/ansi` use. A combining
  mark or ZWJ part shares its cluster's cells; the cursor and `h`/`l` still step per rune, and a cursor on a
  mark highlights its cluster. Ambiguous-width characters are one cell.
- Tabs expand to one constant (`defaultTabstop`, 8, read through `Editor.tabstop`) until a `tabstop` option exists.
- Control characters draw as `^X` (`^?` for DEL) and C1 controls as `<85>`; invalid UTF-8 bytes are one rune
  and one U+FFFD cell. Standalone zero-width clusters draw nothing.
- Normal and Visual cursors sit on the last cell of a tab, as in Vim, and the ruler shows `row,col-vcol` when
  the display column differs. Insert and Replace sit on its first cell, before the character. A wide
  character highlights both cells.
- `j`/`k` keep the preferred column (`goalCol`) in display columns; landing inside a tab or wide character
  selects it.
- Lines do not wrap: `nowrap` with Vim's `sidescroll=0` policy. When the cursor leaves the window it is
  recentred, and it scrolls back for shorter lines. `View` never emits a line wider than the pane; a wide
  character cut by either edge becomes a space.
- The status line, `statusMsg` and prompt are cut to the width (the command line keeps its tail). Below
  three rows the prompt/status share the last line (the prompt wins while active); with three or more
  rows the text area is `rows-2` lines.
- `Resize` re-validates the cursor row and column.

### T01 scope and limits

Confirmation supports the G01 pattern subset (originally literal-only), `i`/`g`, `%`, numeric and Visual
ranges (including saved `'<,'>` bounds), and an optional trailing delimiter. The `c` prompt accepts `y`, `n`, `a`, `q`,
`l`, Escape, and Ctrl-E/Ctrl-Y viewport scrolling. Accepted replacements form one undo
unit and preserve registers. Confirmed substitution is not dot-repeatable and retains the
previous dot change. Zero accepted replacements (or identical replacement text) leave history,
redo and dirty state unchanged. The cursor finishes on the last substituted line. Newline-producing replacements are explicitly rejected in confirmation mode until G02;
the replacement grammar remains G02. Confirmed substitution inside `:global`/`:v` is
explicitly rejected rather than starting a prompt inside their synchronous execution (G03).
Playback of an existing macro key stream routes confirmation answers like typed input; full
command/confirmation recording remains deferred with H09. The `n` flag and numeric Ex counts are
not added. T01 uses Vim 9.1 `nocompatible` documentation (`:s_c`, `:s_flags`) as specification only.

### T02 range writes and file identity

`:w[!] path` accepts numeric and Visual line ranges, including saved `'<,'>` bounds.
Invalid numeric write addresses and inverted ranges are rejected with E16 rather than clamped.
`:w[!] >> [path]` appends; appending to a missing path requires `!` under the selected
Vim default profile. A partial write to the current file is refused with E140 unless forced.
An explicit range never marks the buffer clean, even when it spans the entire buffer.
As in the selected Vim profile, a clean buffer also stays clean after a forced partial
write or append to its current file, even though the file can then differ from the buffer.
The external-file snapshot is refreshed after success, so a later full `:w` does not falsely
report an outside modification. Use a full `:w` to replace that partial/appended file.

For a named buffer, `:w other` writes a copy, records the alternate filename, and preserves
the current filename and dirty/saved revision. An unnamed buffer adopts the name and becomes
clean on a successful full, non-range write. `:saveas`/`:sav!` assigns the destination and
cleans on success; `:file name` changes the name without writing, preserves dirty state, and
reports `[Not edited]`. `:wq` and `:x` with a range or name use the same write identity rules;
`:x` writes only when modified. Relative write paths resolve against the editor working directory.
Failures retain the saved revision and dirty state. F07's safety guards and line-format handling
remain in force.

`:[range]w !cmd` now shares T03's asynchronous shell runner. General Ex address grammar
remains G03, and general alternate-buffer navigation remains H12. Complete end-to-end tutor
verification remains T09. No live-terminal manual check is claimed.

### T03 shell command/output lifecycle

`:!cmd`, `:r[ead] !cmd`, `:[range]!cmd`, and `:[range]w !cmd` run asynchronously through
one `sh -c` runner. All use the editor's invocation/working directory, **not** the open file's
parent directory, and the environment captured when the editor is created. Unix uses `/bin/sh`;
other platforms look up `sh`. No interactive terminal/PTY or configurable `shell` /
`shellcmdflag` options are provided. Commands such as `:!ls` and `:!rm FILE` invoke commands
available in that environment; they are not editor builtins.

While a command runs, the editor renders and resizes normally, but editing keys are blocked.
Ctrl-C cancels the command; on Unix the whole shell process group is killed. Other platforms
can only guarantee terminating the direct shell process. Output is collected until completion,
not streamed. `:!` and `:w !` display scrollable, wrapped multiline output; Enter dismisses it.
Stdout and stderr are captured separately and displayed in that order (not interleaved), with
`shell returned N` for a nonzero exit, or an error/cancellation diagnostic. Terminal controls in
displayed output are stripped rather than executed. The runner is intended for finite-output,
noninteractive commands; output is currently collected in memory without a size cap.

`:r[ead] file` and `:r[ead] !cmd` insert below the current or addressed line; `:0r` inserts
before the first line. Relative read paths use the same editor directory. Blank lines are
preserved, and reads use the buffer's line-ending format. Missing final newlines are preserved
when the insertion reaches EOF; an internal boundary necessarily adds a line separator.
Successful range filters replace the selected lines as one undo transaction, including EOF
metadata. Undo/redo restores exact final-newline state. Unlike Vim, **any** nonzero exit,
launch error, or cancellation leaves a filtered buffer unchanged, even if stdout was produced.
Failures show stderr/status; successful read/filter diagnostics do not become buffer text.

`!!`, counted `!!`, and `!{motion}` open a range-filter command prompt using the existing
motion resolver. Escape cancels the prompt. Read/filter edits preserve registers and the prior
dot change; shell commands are not dot-repeatable. Shell commands during macro playback are
explicitly rejected; complete recording/playback remains H09. Shell commands inside `:global`
are also rejected; general Ex composition/compound asynchronous commands remain G03. Original regression tests cover shell output, status, cwd/environment,
exact read/filter bytes, failures, cancellation, undo/redo, and standalone/mux result delivery.
The reference is Vim 9.1 `nocompatible` documentation (`:!`, `:r!`, `:range!`, `shell`,
`shellcmdflag`) as specification only, with the stricter failure policy described above. No
external implementation, fixtures, or tutor prose were copied; existing attribution is unchanged.

### T05 edit and reload boundary

`:e`, `:ed`, `:edi`, and `:edit` load a file, with optional `!`. Without an
argument they reload the current filename; an unnamed buffer reports an error.
A modified buffer is protected by E37 unless forced, including a reload or an
explicit edit of the same path. A missing file opens empty with `[New]`; other
read failures preserve the current buffer, name, history and alternate name.
On success, undo/redo and line-restore history are reset, local marks and jumps
are cleared, and the cursor starts at line 1, column 1, including `:e #`.
Registers and the previous dot change survive. The previous filename becomes
the alternate only when the filename changes. This is a small identity/history
boundary for future help and H12, not a multi-buffer cache.

The whole argument is one filename, including literal spaces. `%` and `#`
expand to current and alternate names anywhere in it; `~/` and `$VAR`/`${VAR}`
use the environment captured when the editor was created. Variable names match
`[A-Za-z_][A-Za-z0-9_]*`; malformed or unsupported dollar forms and unset or
empty variables are rejected without changing the buffer. Relative paths use
the editor working directory. There is no shell or glob expansion, `~user`,
filename modifiers, or quote grammar. Ranges, `+cmd`/`++opt`, and edit nested
in `:global` are unsupported. Full buffer navigation and independent buffer
histories remain H12; help remains T06; complete tutor verification remains T09.

T05 references Vim 9.1 `nocompatible` documentation only. No external code was
consulted and no new attribution is required. No live-terminal manual check is
claimed.

### Tutor mapping (selected lessons only)

| Tutor lesson | Relevant operation | Ledger coverage |
| --- | --- | --- |
| 3.1 | Put (`p`/`P`) | Put behavior/counts are covered by F02; semantic put repeat by F06. |
| 3.2 | Replacement (`r`, `R`) | Character replacement and Replace mode, including `r<CR>` and dot repeat, are covered by F06. |
| 3.3 | Change (`c`) | Existing `cw`/change ranges and semantic dot repeat are covered by F03/F06. |
| 3.4 | `c` with motion | Existing change-motion ranges and semantic dot repeat are covered by F03/F06. |
| 4.4 | Substitution | T01 covers literal substitution confirmation and cancellation, ranges, per-line/global matches, and optional trailing delimiter; the pattern subset is G01 and the replacement grammar remains G02. |
| 5.1–5.2 | Shell listing/removal and write | T03 covers asynchronous shell command routing/output; T02/F07 cover writes. |
| 5.3 | Write a selection | T02 covers numeric and Visual range writes and file identity; complete lesson verification remains T09. |
| 5.4 | Read file/command output | T03 covers addressed reads, exact blank lines and EOF handling. |
| 6.1 | Open lines | Existing insert/open-line behavior is covered where exercised by insertion fixtures; no complete lesson-parity claim. |
| 6.2 | Append | `A` and insertion behavior are covered in F01/F06; no complete lesson-parity claim. |
| 6.3 | Replace mode | Replace stream, Backspace and semantic repeat are covered by F06. |
| 6.4 | Yank/put | Yank/register behavior is covered by F02/F03; yanks do not replace the dot repeat, and put is covered by F06. |
| 7.2 | Edit/reload a file | T05 covers the edit boundary; Misa-native startup configuration remains T08 and full lesson verification T09. |

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
A successful full current-file save, unnamed-buffer adoption, or save-as updates the saved identity;
range writes, appends, and named-buffer copies do not, as detailed under T02.

Nested `beginEdit`/`commitEdit`/`abortEdit` groups compound changes. Insert mode, through Escape,
is one unit (including Backspace, newline, and count repetitions); `cw` is one unit. Every dot
execution is one undo unit. All accepted replacements in a confirmed substitution form one unit.
Only a committed real edit clears redo. No-ops, failures, yanks, and motions
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

- **F03:** The range and motion coverage above is partial, not general Vim operator compatibility. `gu`/`gU`/`g~` are not existing
  normal operators here (G06). Sentence-motion/operator coverage remains limited; F04 motions are
  covered above.
- **F09:** `wrap`, `list`/`listchars`, `number`, `tabstop`, `sidescroll`/`sidescrolloff` and `ambiwidth`
  are not implemented (D07/H11/T06). Visual `j`/`k` does not keep a preferred column yet. Cursor steps
  per rune, not per grapheme. Mux chrome under about five columns may itself overflow; the editor's output does not.
- **F04:** Search offsets and command/search history (T07) are unsupported; patterns are the G01 subset.
  No `//` delimiter grammar is claimed
  (empty prompt Enter reuses the last pattern). `wrapscan`, `matchpairs`, and `cpoptions` are not
  configurable. Mark relocation/navigation enhancements (H10) and Visual parity (H07) remain deferred.
  Uppercase marks retain their existing same-buffer behavior; cross-file marks are not implemented.
- **F05:** Undo behavior is limited to the model and cases above; it is not full Vim undo parity.
  In particular, arrow-driven insert undo breaks and full Vim cursor/deleted-line/indent heuristics
  are not claimed.
- **F06:** Visual-mode dot repeat and broader text-object repeat parity remain deferred (H07/H04);
  `ciw` followed by dot is covered by the H04 fixture above. Numbered-register
  auto-advance on dot (for example, `"1p` followed by `.` selecting `"2`) is TODO(H08). Macro behavior
  is unchanged and not certified. Arrow-driven input expansion and related behavior are unchanged; see
  F08. Dot repeats the last real change semantically, with an explicit count overriding the stored
  count; no-op commands and yanks do not replace that repeat. `r<CR>` is supported. Each dot execution
  is one undo transaction.
- **T02/T03 residual:** Full tutor 5.x verification remains T09; broader Ex addressing and
  compound asynchronous command execution remain G03, and buffer navigation H12.
- **T05:** Ranges, `+cmd`/`++opt`, edit inside global, shell/glob and other
  filename grammar remain unsupported; H12 buffer navigation and T09 manual tutor
  verification remain deferred. See the T05 boundary above.

`misatutor` describes counted `gg`, operator-find, `d%`, and substitution confirmation;
the motion examples use the shared motion path. This ledger does not claim complete tutor parity.
Broader macro, Visual, and text-object compatibility remains deferred; H04 examples are listed above.

### H03 limits

- nroff macros (the 'paragraphs'/'sections' options, `.IP`, `.SH`) are unsupported; sections are `{`/`}`/form feed in column 0.
- Form-feed lines bound paragraphs and sections but not sentences.
- `[( [{ ]) ]}` count brackets inside double quotes (Vim skips them).
- Visual `v}`/`v{` do not include the EOL of an empty target line.
- The exclusive-column-0 rule and the `d` linewise exception apply only to the structural motions; older F03 fixtures (`2dw`, `jdb`) differ from Vim 9.2 and are unchanged.
- `>` indents empty lines (pre-existing). `^`/`G` on an all-blank line land on its last character, as in Vim.
- Oracle quirks: a failed motion aborts the remaining keys of a `normal!`; `zt`/`zz`/`zb` are unreliable under `-es`, so H/M/L fixtures use `NGzt` immediately before the motion.
