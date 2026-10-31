# Plan: built-in commands

Misa's built-ins (`internal/builtins`) are a coreutils subset written from documented and tested behaviour,
checked against the toybox, uutils and diffutils test suites in `tests/`. `misa --list` shows them. They run
as a multicall binary (`misa ls`) or in-process (`builtin_mode: inprocess`) on systems without a shell.

Never copy code from GNU coreutils or other GPL projects. Code adapted from permissive projects needs a source
comment, a `docs/CREDITS.md` entry and, for substantial pieces, a package `NOTICE`.

## Known failures

Cases Misa doesn't pass are listed with a reason in `tests/toybox/known_failures.txt` and
`tests/uutils/known_failures.txt`, and skipped; `MISA_KNOWN_FAILURES=1 go test ./tests/...` runs them.
Grouped by cause:

- **Output format**: `wc` column widths for single files and totals (10 cases); `rmdir` and `mv` message
  wording; `seq -w` padding with hex operands.
- **Missing options**: `ls --time=WORD`, `--time-style` validation; negative `nl -i`.
- **Sorting**: `sort -V` tie order, `sort -n` accepting a leading `+`, `--random-source` read without `-R`.
- **Locale**: `sort` collation and `numfmt` grouping need locale support; decide whether Misa supports
  locales at all before working on these.
- **Randomness**: `shuf --random-source` and seeded output don't match uutils' generator (9 cases). Matching
  is only worth it if a script relies on it.
- **Filesystem-dependent**: `du` block counts differ on btrfs and zfs (toybox, 10 cases). Run these on ext4
  or tmpfs, or compare apparent sizes.
- **Test adapter gaps**: multi-line setup loops in toybox `chmod` cases; `mv -i` cases that reuse files
  from earlier cases.
- **Others**: `cp` mode of a symlinked destination, `ln -sf a a` should fail, `env =BLAH`, `mv` keeping hard
  links across a directory move, `touch -t` inside a DST gap.

When a case starts passing, delete its line.

## Next

1. **Safe in-process use.** `builtin_mode: inprocess` runs built-ins on goroutines in one process, so
   process-wide state is shared. Remaining: `currentUmask` flips the umask off Linux; `sort
   --compress-program` execs; not every built-in honours `ctx.Env`; `builtins.Context` now carries a
   `Cancel` channel, which `bc` observes, but `tail -f` and `sleep` are not wired to it yet. Then a
   `builtins.Run` helper usable as a library.
2. **Missing utilities.** `utility-comparison.md` lists every utility in toybox, BusyBox and uutils
   against Misa's (2026-10-15); `builtins-coverage.md` ranks the missing ones by how many upstream
   test cases would run unchanged and how much work they are. The POSIX ones toybox and uutils both
   have: `test`/`[`, `od`, `dd`, `df`, `install`, `chgrp`, `nohup`, `mknod`, `who`, `renice`, `ps`,
   `pidof`, `xargs`, `find`, `grep`, `sed`, `tar`. `find` and `grep` should share a walker and matcher with ugh's search tool (`internal/search`, see
   `docs/plans/ughbot.md`). Each new command starts from its upstream tests.
3. **`fetch` styling.** `fetch` writes raw ANSI to stdout. It is command output, not Misa UI, so this may be
   fine; if it stays, say so in the file, otherwise move it to lipgloss.

## How to add or fix a command

1. Find the upstream cases in `tests/toybox`, `tests/uutils` or `tests/diffutils`, or write native cases in
   `tests/native`.
2. Write the failing test, then the code. Use `Spec`/`ParseArgs` for options and register with
   `Register(Command{…})` in `init()`.
3. Run `go test ./internal/builtins/ ./tests/...` and remove any known-failure lines that now pass.
