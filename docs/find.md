# `find` compatibility (U06)

Misa's `find` is an in-process filesystem walker, not a claim of complete POSIX,
GNU findutils or Toybox compatibility. Expressions support parentheses,
negation, implicit and explicit AND, OR and comma; absent an output action,
matching paths are printed. The walker supports `-P`, `-H`, `-L`, `-depth`,
`-mindepth`, `-maxdepth`, `-xdev`/`-mount`, `-prune`, `-quit`, `-print`
and NUL-terminated `-print0`. Metadata and pattern predicates are subject to
platform filesystem support.

Predicates include `-true`, `-false`, `-name`, `-iname`, `-path`, `-ipath`
(and `-wholename`/`-iwholename`), `-type`/`-xtype` (including comma-separated
types), `-empty`, `-size`, `-mtime`/`-atime`/`-ctime`,
`-mmin`/`-amin`/`-cmin`, `-newer`/`-anewer`/`-cnewer`, `-samefile`,
`-perm`, `-uid`, `-gid`, `-user`, `-group`, `-nouser`, `-nogroup`,
`-links` and `-inum`. Numeric predicates use exact `n`, greater-than `+n`
and less-than `-n`. `-size` rounds up in units of 512 bytes (default or `b`),
bytes (`c`), two bytes (`w`), KiB (`k`), MiB (`M`) or GiB (`G`). Age
predicates use whole 24-hour days or whole minutes respectively. `-perm`
accepts octal or symbolic modes: exact, `-` for all bits, `/` for any bit.
`-lname`, `-ilname` and `-newerXY` (including `-newerat`) are unsupported.

`-regex` and `-iregex` use the in-tree `posixre` ERE engine and match the
**entire displayed path**. This is Misa's baseline, not Toybox compatibility:
Toybox's pinned test file does not test regex. `-regextype` is not supported.

`-printf` accepts exactly `%%`, `%p`, `%f`, `%h`, `%P`, `%s`, `%m`, `%M`,
`%u`, `%g`, `%t`, `%T@`, `%y`, and `%d`; flags, widths, precision and other
directives are not supported. Escapes are `\a`, `\b`, `\f`, `\n`, `\r`,
`\t`, `\v`, `\\`, up to three octal digits, and `\c` (stop this format).
Unsupported directives/escapes are errors.

`-delete`, `-exec`, `-execdir`, `-ok`, and `-okdir` are explicitly deferred
to U07 and rejected, not passed to a host utility. Tests use the pinned 0BSD
Toybox `find.test`; see `tests/toybox/known_failures.txt` for cases deferred
or excluded and their reasons. The existing uutils adapter targets coreutils,
not uutils/findutils, and does not run findutils tests.
