# getconf

`getconf` prints POSIX system configuration values. The interface and the name
set follow Toybox's `toys/posix/getconf.c` (0BSD, commit
`6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb`); the values are Misa's own, taken
from runtime probes, per-platform constants and POSIX.1-2008, never from a
guess. Implementation: `internal/builtins/getconf.go` plus
`internal/builtins/getconf_platform_*.go`.

## Interface

```
usage: getconf -a [PATH] | -l | NAME [PATH]
```

- `getconf NAME` prints the value and a newline.
- `getconf NAME PATH` prints the value of a `pathconf(3)`-sourced name. The
  path is required for those names and rejected for every other name.
- `getconf -a [PATH]` prints every name and value, group by group, as
  `fmt.Printf("%-34s %s\n", name, value)`; the path defaults to `/`.
- `getconf -l` prints the group titles `sysconf(3)`, `pathconf(3)`,
  `confstr(3)`, `<limits.h>`, `Misc` in that order, each followed by its names
  indented by two spaces.
- `-a` and `-l` together: `-a` wins (Toybox's `if (FLAG(a))` comes first).
- `--help` (or no operand at all, as in Toybox) prints the usage text on stdout
  and exits 0.
- Toybox's spec is `>2al`: a third operand is `getconf: extra operand 'x'` and
  exit 1. `-v` and `--version` do not exist and fail as unknown options.

The same in-memory table (`confGroups`) drives `-a`, `-l` and single lookups, so
the three modes cannot disagree.

## Where values come from

Each name is answered from exactly one of:

- **measured** — a runtime probe, named in the tables below;
- **compiled** — a per-platform constant from `getconf_platform_*.go`, or a
  POSIX.1-2008 / C constant that is the same everywhere Misa runs;
- **undefined** — printed as the word `undefined`, exit 0 (Toybox's
  `sysconf(-1)`/`pathconf(-1)` path). Never a fabricated value.

## Linux and Android

Android uses the Linux table (Android is Linux) with one difference: confstr
`PATH` is `/system/bin`, because Android has no `/usr/bin`.

| Name | Value | Source |
| --- | --- | --- |
| `PAGESIZE`, `PAGE_SIZE` | 4096 (per page size) | measured: `os.Getpagesize()` |
| `NPROCESSORS_CONF`, `NPROCESSORS_ONLN`, `_NPROCESSORS_CONF`, `_NPROCESSORS_ONLN` | 8 (per machine) | measured: `runtime.NumCPU()` |
| `OPEN_MAX` | per machine | measured: soft `RLIMIT_NOFILE` (`getrlimit`); the compiled 1024 is used only if the call fails |
| `CHILD_MAX` | per machine | measured: soft `RLIMIT_NPROC`, the kernel's cap on processes per real UID; `undefined` when unlimited or on failure |
| `LEVEL1_ICACHE_*`, `LEVEL1_DCACHE_*`, `LEVEL2/3/4_CACHE_*` (`_SIZE`, `_ASSOC`, `_LINESIZE`) | per CPU | measured: `/sys/devices/system/cpu/cpu0/cache/index*/{level,type,size,ways_of_associativity,coherency_line_size}`; `undefined` for caches the machine lacks or when sysfs is absent |
| `ARG_MAX` | 131072 | compiled: `<linux/limits.h>` |
| `CLK_TCK` | 100 | compiled: the kernel's `USER_HZ` |
| `HOST_NAME_MAX` | 64 | compiled: `utsname.nodename` is 65 bytes including the NUL |
| `LINE_MAX` | 2048 | compiled: POSIX's minimum, which Linux does not lower |
| `IOV_MAX`, `UIO_MAXIOV` | 1024 | compiled: `<linux/uio.h>`'s `UIO_MAXIOV` |
| `LOGIN_NAME_MAX` | 256 | compiled: the platform's documented login name limit |
| `NGROUPS_MAX` | 65536 | compiled: the kernel's fixed supplementary group limit |
| `SYMLOOP_MAX` | 40 | compiled: the kernel's `MAXSYMLINKS` |
| `TTY_NAME_MAX` | 32 | compiled: the platform's documented tty name limit |
| `TZNAME_MAX` | 6 | compiled: POSIX's minimum, which Linux does not lower |
| `NAME_MAX` (pathconf) | 255 | compiled: `<linux/limits.h>` |
| `PATH_MAX` (pathconf) | 4096 | compiled: `<linux/limits.h>` |
| `LINK_MAX` (pathconf) | 127 | compiled: `<linux/limits.h>` |
| `PIPE_BUF` (pathconf) | 4096 | compiled: the page size `<linux/limits.h>` documents |
| `MAX_CANON`, `MAX_INPUT` (pathconf) | 255 | compiled: `<linux/limits.h>` |
| `FILESIZEBITS` (pathconf) | 64 | compiled: Linux file offsets are 64-bit |
| `_POSIX_VDISABLE` (pathconf) | 0 | compiled: Linux's `_POSIX_VDISABLE` |
| `CHOWN_RESTRICTED`, `NO_TRUNC` (pathconf) | 1 | compiled: POSIX's values for these properties |
| `PATH` (confstr), `CS_PATH` | `/usr/bin:/bin` | compiled: the platform's standard command path |
| `SYMLINK_MAX`, `ASYNC_IO`, `PRIO_IO`, `SYNC_IO` (pathconf) | `undefined` | no source without a C library |

## Darwin

| Name | Value | Source |
| --- | --- | --- |
| `PAGESIZE`, `PAGE_SIZE` | per page size | measured: `os.Getpagesize()` |
| `NPROCESSORS_CONF`, `NPROCESSORS_ONLN`, `_NPROCESSORS_CONF`, `_NPROCESSORS_ONLN` | per machine | measured: `runtime.NumCPU()` |
| `OPEN_MAX` | per machine | measured: soft `RLIMIT_NOFILE`; compiled 256 as a fallback |
| `CHILD_MAX` | per machine | measured: soft `RLIMIT_NPROC` |
| `ARG_MAX` | 1048576 | compiled: `<sys/syslimits.h>` |
| `CLK_TCK` | 100 | compiled: the platform's clock tick |
| `HOST_NAME_MAX` | 255 | compiled: `<limits.h>` |
| `LINE_MAX` | 2048 | compiled: POSIX's minimum |
| `IOV_MAX`, `UIO_MAXIOV` | 1024 | compiled: the platform's `UIO_MAXIOV` |
| `NGROUPS_MAX` | 16 | compiled: `<sys/syslimits.h>` |
| `SYMLOOP_MAX` | 32 | compiled: `<sys/syslimits.h>` |
| `NAME_MAX` (pathconf) | 255 | compiled: `<sys/syslimits.h>` |
| `PATH_MAX` (pathconf) | 1024 | compiled: `<sys/syslimits.h>` |
| `LINK_MAX` (pathconf) | 32767 | compiled: `<sys/syslimits.h>` |
| `PIPE_BUF` (pathconf) | 512 | compiled: `<sys/syslimits.h>` |
| `FILESIZEBITS` (pathconf) | 64 | compiled: the platform's file offsets |
| `_POSIX_VDISABLE` (pathconf) | 0 | compiled: the BSD `_POSIX_VDISABLE` |
| `CHOWN_RESTRICTED`, `NO_TRUNC` (pathconf) | 1 | compiled: POSIX's values |
| `PATH` (confstr), `CS_PATH` | `/usr/bin:/bin:/usr/sbin:/sbin` | compiled: the platform's standard command path |
| `LOGIN_NAME_MAX`, `TTY_NAME_MAX`, `TZNAME_MAX`, `MAX_CANON`, `MAX_INPUT`, `SYMLINK_MAX`, cache names | `undefined` | no documented value this build can state without a C library, and no sysfs |

## FreeBSD and OpenBSD

Both use the same table, with `CLK_TCK` 128 and `NGROUPS_MAX` 1023 on FreeBSD
and 100 and 16 on OpenBSD.

| Name | Value | Source |
| --- | --- | --- |
| `PAGESIZE`, `PAGE_SIZE` | per page size | measured: `os.Getpagesize()` |
| `NPROCESSORS_*`, `_NPROCESSORS_*` | per machine | measured: `runtime.NumCPU()` |
| `OPEN_MAX` | per machine | measured: soft `RLIMIT_NOFILE`; compiled 64 as a fallback |
| `CHILD_MAX` | per machine | measured: soft `RLIMIT_NPROC` |
| `ARG_MAX` | 262144 | compiled: `<sys/syslimits.h>` |
| `CLK_TCK` | 128 / 100 | compiled: the platform's clock tick |
| `HOST_NAME_MAX` | 255 | compiled: `<limits.h>` |
| `LINE_MAX` | 2048 | compiled: POSIX's minimum |
| `IOV_MAX`, `UIO_MAXIOV` | 1024 | compiled: the platform's `UIO_MAXIOV` |
| `NGROUPS_MAX` | 1023 / 16 | compiled: `<sys/syslimits.h>` |
| `SYMLOOP_MAX` | 32 | compiled: `<sys/syslimits.h>` |
| `NAME_MAX` (pathconf) | 255 | compiled: `<sys/syslimits.h>` |
| `PATH_MAX` (pathconf) | 1024 | compiled: `<sys/syslimits.h>` |
| `LINK_MAX` (pathconf) | 32767 | compiled: `<sys/syslimits.h>` |
| `PIPE_BUF` (pathconf) | 512 | compiled: `<sys/syslimits.h>` |
| `FILESIZEBITS` (pathconf) | 64 | compiled: the platform's file offsets |
| `_POSIX_VDISABLE` (pathconf) | 0 | compiled: the BSD `_POSIX_VDISABLE` |
| `CHOWN_RESTRICTED`, `NO_TRUNC` (pathconf) | 1 | compiled: POSIX's values |
| `PATH` (confstr), `CS_PATH` | `/usr/bin:/bin` | compiled: the platform's standard command path |
| `LOGIN_NAME_MAX`, `TTY_NAME_MAX`, `TZNAME_MAX`, `MAX_CANON`, `MAX_INPUT`, `SYMLINK_MAX`, cache names | `undefined` | no documented value this build can state without a C library |

## Any other platform

`getconf_platform_other.go` reports `undefined` for every platform-dependent
name; only the platform-independent names below still have values.

## The same everywhere Misa runs

- `_POSIX_VERSION` = 200809, `POSIX2_VERSION` = 200809, `_XOPEN_VERSION` = 700:
  POSIX.1-2008 (SUSv4) constants.
- `<limits.h>`: `CHAR_BIT` 8, `WORD_BIT` 32, `INT_MAX`/`INT_MIN`,
  `SCHAR_MAX`/`SCHAR_MIN`, `SHRT_MAX`/`SHRT_MIN`, `UCHAR_MAX`, `UINT_MAX`,
  `USHRT_MAX` come from Go's fixed-width types (`math.MaxInt32` and friends).
  `SSIZE_MAX` is `math.MaxInt64` where `strconv.IntSize == 64` and
  `math.MaxInt32` where it is 32. `ULONG_MAX` follows the same width rule,
  because `long` is pointer sized (LP64) on every platform Misa builds for.
  `LONG_BIT` is `strconv.IntSize` (bits, 64 or 32).
  `CHAR_MAX`/`CHAR_MIN` follow the target ABI's plain `char`: -128/127 on x86
  and RISC-V, 0/255 on ARM, AArch64 and PowerPC (whose ABIs make plain `char`
  unsigned), except on Apple platforms, which keep it signed.
- The `_POSIX_*` and `POSIX2_*` names under `<limits.h>` are POSIX.1-2008's
  minimums and the standard's own utility limits: `_POSIX_ARG_MAX` 4096,
  `_POSIX_CHILD_MAX` 25, `_POSIX_LINK_MAX` 8, `_POSIX_MAX_CANON` 255,
  `_POSIX_MAX_INPUT` 255, `_POSIX_NAME_MAX` 14, `_POSIX_NGROUPS_MAX` 8,
  `_POSIX_OPEN_MAX` 20, `_POSIX_PATH_MAX` 256, `_POSIX_PIPE_BUF` 512,
  `_POSIX_RE_DUP_MAX` 255, `_POSIX_SSIZE_MAX` 32767, `_POSIX_STREAM_MAX` 8,
  `_POSIX_SYMLINK_MAX` 255, `_POSIX_SYMLOOP_MAX` 8, `_POSIX_TZNAME_MAX` 6,
  `_POSIX_TTY_NAME_MAX` 9, `_POSIX_LOGIN_NAME_MAX` 9,
  `_POSIX_HOST_NAME_MAX` 255, `_POSIX_AIO_MAX` 1, `_POSIX_AIO_LISTIO_MAX` 2,
  `_POSIX_DELAYTIMER_MAX` 32, `_POSIX_RTSIG_MAX` 8, `_POSIX_SEM_NSEMS_MAX`
  256, `_POSIX_SEM_VALUE_MAX` 32767, `_POSIX_SIGQUEUE_MAX` 32,
  `_POSIX_TIMER_MAX` 32, `_POSIX_THREAD_KEYS_MAX` 128,
  `_POSIX_THREAD_DESTRUCTOR_ITERATIONS` 4, `_POSIX_THREAD_THREADS_MAX` 64,
  `POSIX2_BC_BASE_MAX` 99, `POSIX2_BC_DIM_MAX` 2048, `POSIX2_BC_SCALE_MAX`
  99, `POSIX2_BC_STRING_MAX` 1000, `POSIX2_CHARCLASS_NAME_MAX` 14,
  `POSIX2_COLL_WEIGHTS_MAX` 2, `POSIX2_EXPR_NEST_MAX` 32, `POSIX2_LINE_MAX`
  2048, `POSIX2_RE_DUP_MAX` 255.
- `Misc`: `LFS_CFLAGS` is `-D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64` when
  `strconv.IntSize == 32` and an empty line otherwise (Toybox's rule);
  `LFS_LDFLAGS` and `LFS_LIBS` are empty lines.

## What is always `undefined`

Answering these would mean asking a C library, a kernel interface or a
subsystem Misa does not implement, so they print `undefined` on every platform:

- every POSIX.2, X/Open and POSIX option test (`_POSIX_THREADS`,
  `_POSIX_BARRIERS`, `_POSIX_ADVISORY_INFO`, `_POSIX_ASYNCHRONOUS_IO`,
  `_POSIX_TRACE*`, `_POSIX_V7_*`, `POSIX2_C_BIND`, `POSIX2_PBS*`,
  `_XOPEN_UNIX`, `_XOPEN_STREAMS`, …): they report whether the C library
  supports an option;
- `AIO_LISTIO_MAX`, `AIO_MAX`, `AIO_PRIO_DELTA_MAX`, `ATEXIT_MAX`, `BC_*`,
  `COLL_WEIGHTS_MAX`, `DELAYTIMER_MAX`, `EXPR_NEST_MAX`, `MQ_OPEN_MAX`,
  `MQ_PRIO_MAX`, `RAW_SOCKETS`, `RE_DUP_MAX`, `RTSIG_MAX`, `SEM_NSEMS_MAX`,
  `SEM_VALUE_MAX`, `SIGQUEUE_MAX`, `STREAM_MAX`, `TIMER_MAX`,
  `PTHREAD_DESTRUCTOR_ITERATIONS`, `PTHREAD_KEYS_MAX`, `PTHREAD_STACK_MIN`,
  `PTHREAD_THREADS_MAX`, `_AVPHYS_PAGES`, `_PHYS_PAGES`: C-library or VM
  values with no portable probe here (`_PHYS_PAGES` would need `/proc`);
- `V7_ENV` (confstr): no C library to ask. Toybox on glibc prints an empty
  line here, because `confstr` returns length 0;
- `ASYNC_IO`, `PRIO_IO`, `SYNC_IO` (pathconf): no asynchronous I/O probing.

## Path argument rules

- A `pathconf(3)` name (`ASYNC_IO`, `CHOWN_RESTRICTED`, `FILESIZEBITS`,
  `LINK_MAX`, `MAX_CANON`, `MAX_INPUT`, `NAME_MAX`, `NO_TRUNC`, `PATH_MAX`,
  `PIPE_BUF`, `PRIO_IO`, `SYMLINK_MAX`, `SYNC_IO`, `_POSIX_VDISABLE`) requires
  a path: `getconf NAME` without one is a usage error.
- Every other name rejects a path: `getconf NAME PATH` is a usage error.
- The path is never opened. The values are the platform maxima from the table
  above, not a probe of the filesystem the path lives on, so `getconf NAME
  /nonexistent-path` succeeds and prints the same value as `getconf NAME /`.
  A real per-filesystem answer would need `pathconf(3)`, which needs a C
  library (and `statfs` field names differ per OS).
- `CS_PATH` is accepted as a synonym for the confstr `PATH` (Toybox strips the
  `CS`, because autoconf asks for `CS_PATH`).

## Errors and exit status

- Unknown name: `getconf: bad 'NAME'` on stderr, exit 1, nothing on stdout.
- Usage errors (`NAME needs a path`, `NAME does not take a path`,
  `extra operand 'x'`): the message and then the usage text on stderr, exit 1,
  the way Toybox's `help_exit` reports them.
- Unknown option: `getconf: unknown option '-v'` on stderr, exit 1.
- `undefined` is not an error: it goes to stdout and exits 0.

## Known differences from Toybox

- Toybox's name set has no C library behind it, so Toybox prints whatever
  `sysconf`/`pathconf`/`confstr` return; Misa prints `undefined` for anything it
  cannot measure or read from a documented constant. Expect more `undefined`
  lines than Toybox on the same machine, and never a made-up `-1`.
- Toybox takes the path for `-a` only when two operands are given
  (`path = (toys.optc==2) ? toys.optargs[1] : "/"`), so `getconf -a /tmp`
  silently uses `/`. Misa treats `-a PATH` as the path and rejects a second
  operand. The values are identical, since the path is not probed.
- `-l` ignores operands in both implementations.
- `V7_ENV` is `undefined` (Toybox on glibc prints an empty line).
- `CHAR_MIN`/`CHAR_MAX` follow the target ABI instead of being assumed signed,
  so ARM/AArch64/PowerPC builds report 0/255 as `<limits.h>` does there.
- No `-v` (neither implementation has it) and no `--version` (Toybox has none
  either); glibc's `getconf -v` is not reproduced.
- Unknown-option wording is Misa's own (`getconf: unknown option '-v'`).

## References

- Toybox `toys/posix/getconf.c`, 0BSD, commit
  `6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb` — interface, name lists, order,
  output format and the `CS_PATH` workaround.
- POSIX.1-2008 (SUSv4), `getconf`, `sysconf(3)`, `pathconf(3)`, `confstr(3)`,
  `<limits.h>`, `<unistd.h>` — modes, name meanings, minimum values and the
  constants in the `<limits.h>` group.
