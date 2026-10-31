# Plan: `locate` and `updatedb` (roadmap O19)

`locate` answers "where is that file?" from a database built by `updatedb`, so a search does not walk the
filesystem. Both are registered in `internal/builtins` (`locate.go`, `locate_db.go`, `updatedb.go`,
`locate_access_unix.go`, `locate_access_other.go`) and run in-process or through the multicall binary like
every other builtin.

```
updatedb [--localpaths='DIR...'] [--prunepaths='PATH...'] [--output=DBFILE]
locate [-d DB] [-i] [-c] [-l N] [-0] [-e] [-b] [-w] [-r] [-a] PATTERN...
```

## References

- `uutils/findutils` at `d71f3242b0bc3b21bb311f04b23ae62e5f3fed67` (MIT): `src/locate/mod.rs`,
  `src/updatedb/mod.rs` and `tests/db_tests.rs` for the option set, matching rules, exit statuses and the
  test cases that inspired ours. Per-file header comments name the file and commit.
- Toybox has no `locate` or `updatedb`, so nothing was ported from it.
- The traversal is the shared `internal/walk` walker (`grep -r` and `find` use it); automaton-free glob
  matching reuses the whole-path glob trick from `find -path`; `-r` uses `internal/posixre`.

Misa's database is **not** compatible with GNU `LOCATE02`/`slocate`. No findutils/mlocate/plocate database
layout was copied: the container is documented below and versioned by us. Reading such a database fails
with `not a Misa locate database (bad magic)` rather than silently producing nothing.

## Database format (version 1)

All integers are little endian.

```
"MISA-LOCATE\0"                     12-byte magic
uint32  version                     = 1
int64   created                     unix seconds
uint32  rootCount
rootCount × (uint32 len, len bytes)  indexed roots, absolute
entry stream, front coded:
  uint32 prefixLen, uint32 suffixLen, suffixLen bytes    (per path, repeated)
  uint32 0xFFFFFFFF                                      end-of-entries marker
uint64  entryCount
uint32  crc32 (IEEE) of the entry stream
"MISA-LOCATE-END\0"                 16-byte end magic
```

* **Names are byte strings.** Every field is length prefixed, so nothing is lost to a NUL terminator and a
  newline inside a name is just another byte. NUL cannot appear in a POSIX name; a path containing one is
  refused by the writer instead of producing a database that cannot be read back.
* **Front coding.** Each entry stores the number of bytes it shares with the previous path plus the
  remainder. `updatedb` feeds paths in walker order (sorted per directory), so consecutive names share long
  prefixes and the database stays much smaller than the sum of the names. The reader refuses a prefix longer
  than the previous path, which no encoder can produce.
* **Integrity.** The trailer carries the entry count and a CRC-32 of the entry stream, followed by a second
  magic. Truncated files, corrupted payloads, a wrong end marker and trailing bytes after the end marker are
  all reported with a specific message (`truncated database`, `corrupted database (checksum mismatch)`,
  `corrupted database (bad end marker)`, `corrupted database (data after end marker)`).
* **Versions.** A future `version` value is refused: `unsupported database format version N (this locate
  reads version 1)`.
* **Bounds.** At most 4096 roots and 1 MiB per name are accepted while reading, so a hostile database cannot
  make the reader allocate without limit.

## `updatedb`

| Option | Status |
|---|---|
| `--localpaths='DIR...'` | supported; whitespace-separated (as findutils), default `/` |
| `--prunepaths='PATH...'` | supported; whitespace-separated, **no** default prunes (findutils defaults to `/tmp /usr/tmp /var/tmp /afs`) |
| `--output=DBFILE` | supported; default `$XDG_CACHE_HOME/misa/locatedb`, else `~/.cache/misa/locatedb` |
| `--findoptions=`, `--prunefs=`, `--dbformat=`, `--localuser=`, `--netpaths=`, `--netuser=` | unsupported, exit 2 with a message naming the option |

* **No root needed.** The default database is per-user in the cache directory, never under `/var`, so
  `updatedb` can run as an ordinary user. When the default location is used its directory is created; an
  explicit `--output` must name a directory that already exists (as with findutils).
* **Pruning** is by path: a path is skipped when it equals a prune path or lies below it (`/a/b` prunes
  `/a/b` and `/a/b/c`, never `/a/bc`). A pruned directory is not descended into. Filesystem-type pruning
  (`--prunefs`) is deliberately not implemented; there is no fstype probe to lie about.
* **Symlinks are never followed.** The walker uses `lstat`, so a symlink is indexed as its own name and its
  target is not traversed.
* **Unreadable directories** are reported on stderr (`updatedb: PATH: Permission denied`) and skipped; the
  run still writes the database, but exits 1 so a script can notice the index is incomplete.
* **Atomic replacement.** The database is written to a temporary file in the destination directory, flushed,
  `fsync`ed, `chmod`ed to 0644 and `rename`d over the target. A walk error, a write error or a cancellation
  removes the temporary file and leaves the previous database intact, so a reader never sees a half-written
  database. The parent directory is not fsynced after the rename.
* **Exit status:** 0 on success, 1 when the walk reported an unreadable path or the database could not be
  written (or the run was cancelled), 2 for a usage error.
* **Incremental policy: none.** Every run rebuilds the whole database from the configured roots. There is no
  `--require-visibility`, no "update only changed directories" mode, and no record of what changed since the
  last run. A database is a snapshot; this is deliberate, not a stub that pretends to be incremental.
* **Cancellation.** A cancelled invocation (`Context.Cancellation`) aborts the walk, removes the temporary
  file and exits 1.

## `locate`

| Option | Status |
|---|---|
| `-d DB`, `--database=DB` | supported |
| `-i`/`--ignore-case` | supported |
| `-c`/`--count` | supported |
| `-l N`/`--limit N` | supported; `0` means no limit (findutils behaviour differs across versions, so it is documented here) |
| `-0`/`--null` | supported |
| `-e`/`--existing` | accepted; the existence check is always on, so it is the default |
| `-b`/`--basename`, `-w`/`--wholename` | supported; the last of the two wins |
| `-r`/`--regex` | supported via `internal/posixre` (POSIX extended) |
| `-a`/`--all` | supported; all patterns must match |
| `-S`/`--statistics`, `-E`/`--non-existing`, `-L`/`--follow`, `-P`/`-H`/`--nofollow`, `-m`/`--mmap`, `-s`/`--stdio`, `-p`/`--print`, `--max-database-age`, `--regextype` | unsupported, exit 2 with a message naming the option |

* **Pattern rules** follow findutils: a pattern containing `*`, `?` or `[` is matched against the whole path
  as a glob, anything else is a substring match. As in findutils' `fnmatch(3)` call the glob is not anchored
  and `*`/`?` may span `/`. `-b` reduces the subject (not the pattern) to the last path element first.
  Several patterns match any by default (`-a` requires all).
* **Lookup-time access checks.** A path is printed only when (a) `lstat` still finds the entry — a file
  deleted after indexing is silently dropped — and (b) every ancestor directory is searchable by the calling
  user, checked with `access(2)` (`syscall.Access`, `X_OK`). A database built by another user therefore cannot
  disclose a name the caller could not resolve itself. A dangling symlink counts as existing, because the
  entry exists even when its target does not. On platforms without `access(2)` the search check is skipped and
  only existence is verified; that limitation is in `locate_access_other.go`.
* **Database selection:** `-d`, else `LOCATE_PATH` (a single database; a path list is refused with a message
  instead of being half-honoured), else the updatedb default above.
* **Exit status:** 0 when at least one path matched (or, with `-c`, when the count is non-zero), 1 when
  nothing matched or the database could not be opened/read, 2 for a usage error.
* Databases are read sequentially with a 64 KiB buffer; the whole database is never held in memory.
* Cancellation (`Context.Cancellation`) stops the scan and exits 1 with `locate: interrupted: ...`.

## Known differences from findutils and uutils

* The database format is Misa's own; `LOCATE02` and `slocate` databases are rejected.
* Existence and searchability are always enforced at lookup time, so `locate` prints fewer paths than
  findutils, which trusts the database unless `-e` is given. `-E`/`--non-existing` cannot be supported
  meaningfully and fails.
* `--statistics`, `--mmap`, `--stdio`, `--print`, `--max-database-age`, `--regextype`, `--follow`/`--nofollow`,
  `--prunefs`, `--findoptions`, `--netpaths`, `--localuser`, `--netuser` and `--dbformat` are unsupported and
  fail with exit 2 instead of being accepted as no-ops.
* `-l 0` prints everything; `-r` uses POSIX extended syntax (findutils defaults to Emacs syntax);
  `-a` with `-r` requires every regex to match.
* `updatedb` prunes by path only (no `-fstype` probe) and has no default prune list; its default database is
  the user cache, not `/usr/local/var/locatedb`.
* `--localpaths`/`--prunepaths` split on whitespace, so a root or prune path containing a space cannot be
  expressed (same limitation as findutils' `updatedb` argument format).

## Tests

`internal/builtins/locate_db_test.go` (format round trip, front coding, header layout, every corruption
path), `locate_test.go` (substring/glob/regex/case/basename/count/limit/null matching, newline in a name,
deleted-after-indexing, the `access(2)` check, unreadable and truncated databases, usage errors, default path
selection via the invocation environment, cancellation) and `updatedb_test.go` (build, prune, symlinked
directory not followed, unreadable directory reported and skipped, atomic replacement, un-creatable output,
cancellation leaves no database, unsupported options, default database location).

`uutils/findutils`' `tests/db_tests.rs` uses `assert_cmd`/`cargo_bin` and fixture databases under
`test_data/`, so it cannot be run by Misa's `tests/uutils` adapter (which reads `tests/by-util/test_<cmd>.rs`
from uutils/coreutils). Their cases are reimplemented natively above; each test file says so in its header.
