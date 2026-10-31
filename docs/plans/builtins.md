# Plan: built-in commands

Misa's built-ins (`internal/builtins`) are a coreutils subset written from documented and tested behaviour,
checked against the toybox, uutils and diffutils test suites in `tests/`. `misa --list` shows them. They run
as a multicall binary (`misa ls`) or in-process (`builtin_mode: inprocess`) on systems without a shell.

Never copy code from GNU coreutils or other GPL projects. Code adapted from permissive projects needs a source
comment, a `docs/CREDITS.md` entry and, for substantial pieces, a package `NOTICE`.

## U32/U33 — `tar` create/list/extract

`tar -c` creates archives, `tar -t` lists them, and U33 adds `tar -x` extraction, using invocation-local
cwd (never a process-wide `chdir`) and no external archive tools. Supported options include `-f FILE`
(default `-`: stdout for creation, stdin for listing/extraction), `-v`, positional/repeated `-C DIR`,
repeated `--exclude PATTERN`, repeated `-T FILE`/`--files-from FILE` (`-` reads names from stdin),
`--null`, `-h`/`--dereference`, `--numeric-owner`, and numeric-only `--owner ID`/`--group ID`.
Traditional `tar cf FILE ...` syntax is also accepted. `-tO` still lists names, as in Toybox; `-cO` is
rejected.

Creation handles regular files, directories, modes, symbolic links and cross-directory hard links (Unix
device/inode identity), traversing each source in sorted `internal/walk` order. Symlinks are stored
without following unless `-h` is given. Writing uses USTAR where possible and PAX for long names/large
values; reading accepts USTAR, PAX and GNU long-name records. Stored create names are normalized to
relative paths (including removal of leading `./`); link targets themselves are preserved. Subsecond
mtime precision is omitted to permit USTAR headers. Exclude globs match path suffixes/components,
including across slashes, and excluded directories are pruned. List operands select a member or its
descendants. Files-from records are literal names, not options; `--null` selects NUL-delimited records.

U33 extraction supports `-x`, `-z` (gzip creation/extraction; gzip magic is auto-detected for listing and
extraction), `-k`/`--keep-old`/`--keep-old-files` (existing non-directory destinations cause errors),
`--overwrite` (the default: atomically replace regular-file destinations), `-O`/`--to-stdout`, `-p`/`--same-permissions`, `-o`/`--no-same-owner`, `-m`/
`--touch` (do not restore mtime), and `--strip-components N`. The final `-C DIR` selects the extraction
destination (creation retains ordered `-C` behavior). Absolute leading slashes are stripped with a warning;
every `..` path component is conservatively rejected. Extraction does not traverse symlink parents and only
creates hard links whose targets remain contained. Gzip input is validated into a spool before extraction
starts; caller-owned streams remain caller-owned. Cancellation is cooperative (F05). Usage errors return 1;
archive and I/O errors return 2. Regular files are staged beside their destination and published only
when complete. Extraction is not archive-wide transactional: entries already completed remain after a
later archive error or cancellation; cancellation returns the F05 interrupt status (130).

By default archived rwx permissions are masked by the current umask; root restores archived ownership,
non-root does not. Directory metadata is applied after contents. Device nodes and FIFOs require privilege;
special-node creation is supported on Linux (including Android, subject to platform restrictions) and
returns an unsupported error elsewhere. No-follow metadata calls can also report kernel limitations.
Unsupported compression selectors `-j`, `-J`, `-a`, sparse files, transforms, and other unlisted features
are not implemented; there is no host-tool fallback.

This is not full tar parity. The Toybox adapter skips unsupported fixtures and byte-exact header differences
rather than claiming the full suite passes. **U34** will add cpio using the shared archive foundation.

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
   --compress-program` execs; not every built-in honours `ctx.Env`; no cancellation (`tail -f`, `sleep`
   need a `context.Context` in `builtins.Context`). Then a `builtins.Run` helper usable as a library.
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
