# Shell session — S02–S04 and S07

This implements the library foundation for issue #40, on top of S01's in-tree
mvdan/sh and F02's invocation contract. S03 adds the explicit `misa sh` CLI, S04 adds registry-aware resolution, and S07
hardens directory and variable semantics. Existing TUI dispatch is unchanged;
this is not a claim of full POSIX shell compatibility.

## Ownership and API

`internal/shell/session.New(session.Options{...})` creates a session owning one
POSIX parser and one interpreter runner. `Dir`, `Env`, and the three streams are
explicit options. An omitted directory uses the current directory at creation;
a nil environment snapshots the process environment, while a non-nil empty
slice starts without inherited entries. The interpreter may create its own shell
variables, such as `PWD`. Nil input is empty and nil outputs discard data. There
are no startup files or implicit project configuration.

`Run(ctx, source)` parses the whole submission before executing anything. Calls
on one session are serialized. Variables (including unexported variables),
exports, functions, interpreter options, cwd, and the previous `$?` survive
between submissions. Independent sessions do not share that state. The runtime
never calls `os.Chdir`, `os.Setenv`, or `os.Clearenv` to implement shell state.
The parser uses the POSIX dialect; imported interpreter extensions are not a
promise of Bash compatibility. Alias expansion remains S14 work.

The result distinguishes status from a lifecycle or runtime failure:

- Ordinary success or command failure sets `Status`; a nonzero command status
  does not itself set `Err`.
- A syntax error returns status 2 and a typed `*SyntaxErr` in both `ParseError`
  and `Err`, including source position, line, column, and message. Parsing cannot
  partially execute a submission or change its prior shell state/status.
- Cancellation returns 130, deadline expiry 124, with the context error in `Err`.
  State already changed by execution is not rolled back. The session can run
  another submission after cancellation.
- `exit N` returns that status and `Exited: true`; later submissions fail with
  `ErrExited`. This does not terminate the Misa process.

`Dir()`, `Env()` (exported environment), and `Vars()` return state snapshots;
variable maps and array values are copied rather than exposed for mutation.
`Close()` is idempotent, cancels an active submission, and prevents later runs.
The caller retains ownership of its streams; the session never closes them.
Cancellation requires cooperative injected execution and I/O: it cannot interrupt
an arbitrary caller-owned blocked reader/writer or forcibly stop Go code.

## Execution boundary

The session imports neither the builtin registry nor Bubble Tea. Its neutral
`ExecHandler` receives an expanded `Invocation` containing the command `Name`,
`Args` (excluding the command name), cwd, exported environment (including command
prefix assignments), and redirected streams. Arguments are never re-split or
interpolated into another shell. Interpreter-owned shell builtins and functions
remain in the runner; other commands reach the injected handler. With no
handler, unresolved commands return 127: there is no automatic PATH execution
or host-shell fallback.

Callers may explicitly select `internal/shell/sessionbuiltins.Run` as the handler.
That separate adapter constructs an F02 `builtins.Context` and calls
`builtinexec.Run`, enabling its builtin/PATH dispatch without introducing that
dependency into the session core. This is an opt-in executor, **not** a sandbox
or an audit that all registered builtins are safe in-process. In particular:

- F03 still owns legacy process-environment reads and `env`'s default global
  environment bridge.
- F04 still owns priority and other process-local effects (`nice`, etc.).
- F05 still owns broad cooperative utility cancellation and streaming loops.
- F06 still owns unified launch, signal/group cleanup, and ENOEXEC semantics.

Those legacy implementations are unchanged. Only audited handlers can promise
process isolation and prompt cancellation for all work they execute. A session
cannot undo process mutations performed by a supplied handler.

## Migration boundaries and verification

S02 adds an isolated API; no existing execution surface is migrated. S03 owns
noninteractive CLI entry points and positional parameters, S04 command resolution
and introspection, S05 the persistent interactive worker, and S06 TUI integration.
S07 (below) owns directory and variable semantics, S08–S14 the remaining shell
semantics, redirection/pipeline lifecycle, source/eval and aliases. S15–S18 own
process-local state, traps and real job control. Existing
editor/menu/agent/platform callers remain for S19–S21 and the S22 rollout gate.

Native tests cover persistent state/status, independent sessions, unchanged
process cwd/environment, typed all-or-nothing parsing, cancellation/recovery,
exit/close, snapshot ownership, handler invocation, and the F02 adapter. A
session dependency test rejects a transitive Bubble Tea dependency. Imported
foundation patches are recorded in `internal/sh/SOURCE.json` and `NOTICE`; no
second shell copy or dependency is introduced.

## S03 — noninteractive CLI

S03 adds a noninteractive shell CLI without changing the normal Misa launch: plain
`misa` remains the TUI. The explicit command is `misa sh`; argv0 aliases `sh` and
`misa-sh` are installed only by `misa --install-sh DIR`. Existing `--install DIR`
behavior remains unchanged. A manual symlink to the binary is an optional
alternative; no alias is enabled implicitly.

Supported forms:

```text
misa sh -c 'cmd' [name [args...]]
misa sh [options] script [args...]
misa sh [options]                 # read the script from stdin
misa sh -s [args...]              # read the script from stdin
```

The default `$0` is `sh` (or the invoked alias). In `-c`, the optional `name`
replaces `$0`, including with an empty string; for a script operand, that operand
is `$0`. `-e`, `-u`, `-x` and clustered switches are forwarded to the
interpreter, and `--` ends option parsing. `-c` and `-s` are mutually exclusive.
No startup files, `ENV` sourcing, login mode or interactive mode are part of S03.

Standard input is read to EOF and then parsed as one submission, so a producer
that keeps the pipe open after a complete command delays execution until it
closes, and commands in that mode cannot consume further standard input (a `read`
sees end of file). A `-` operand means standard input, like `-s`. The read
observes cancellation (130, or 124 on deadline expiry) by abandoning the blocked
read: a caller-owned reader cannot be interrupted or closed. TTY stdin is
refused with a clear diagnostic and status 1; `-c` and script mode may still
inherit a TTY as stdin for the commands they run. Those
commands run in their own process group through F06 `commandexec`, which does not
yet take foreground terminal ownership, so a program that reads the inherited
terminal directly can wait for input that the terminal does not deliver. F06 owns
that fix.

The CLI composes the S02 session with `sessionbuiltins` and F06 `commandexec`; it
never falls back to a host `/bin/sh`, and works with an empty `PATH`. A nil
session environment inherits the process environment; a non-nil empty environment
starts empty. The exit status is the last command's status, or `exit N`. Syntax
errors return 2 and report `file:line:column: message`; a missing script returns
127, other script open/read errors 126, and usage errors 1 (the pre-existing
`--install` usage status 2 is unchanged), following Toybox conventions. A script
operand must be readable, not executable. `--install-sh` replaces its own stale
symlinks, refuses to overwrite a real file (creating neither alias), and reports
each link it writes.

The interpreter gained one local patch: `$0` is now set explicitly and
independently of source names, so `-c cmd ''` can have an empty `$0` while
diagnostics still carry the script name. It is recorded in `internal/sh/NOTICE`
and `internal/sh/SOURCE.json`.

S03 does not solve S04 command resolution or introspection. S05 interactive
execution and S06 TUI integration remain unwired. The remaining S07–S22 roadmap
and boundaries above are unchanged. This is not a claim of full POSIX or Bash
parity.

## S04 — registry-aware resolution and introspection

S04 makes one resolver serve both execution and `type`, `command -v`, `command -V`
and `hash`. It adds no builtin, no registry entry and no dependency; the session
still imports neither the builtin registry nor Bubble Tea. Instead the caller
injects two hooks, and `sessionbuiltins.Configure` fills them with the same
dispatch `Run` uses:

- `Options.MisaBuiltin(name) bool` reports Misa registry utilities.
- `Options.LookPath(name, dir, env) (path, ok)` resolves an executable through the
  invocation's `PATH`. The adapter uses F06 `commandexec.Resolve`, so the search
  rules, `dir` rooting and permission checks match execution exactly.
- `Options.DefaultPath` is the path `command -p` searches; empty means
  `session.DefaultCommandPath` (`/bin:/usr/bin`, the POSIX `_CS_PATH` value).

A nil `MisaBuiltin` knows no registry, and a nil `LookPath` falls back to the
interpreter's own `PATH` search, so an unwired session behaves as before.

### Precedence

One order is used for execution and for reporting:

1. shell keywords (reported by `type`; they are not runnable commands);
2. shell functions;
3. interpreter shell builtins (`echo`, `cd`, `read`, …);
4. Misa registry utilities, matched on the exact bare name only;
5. `PATH` executables, resolved through F06.

Misa utilities therefore shadow `PATH` names by default, exactly like F06's
builtin-first dispatch (a `rev` on `PATH` does not displace the Misa `rev`), and
shell builtins shadow Misa utilities. Two documented escapes preserve user
control:

- An **explicit path** is always external: `/usr/bin/ls` or `./rev` never falls
  back to a builtin with that basename. A missing path returns 127, a
  non-executable or directory returns 126, and no host `/bin/sh` is involved.
- **`command -p name`** forces the external utility found on `DefaultCommandPath`,
  skipping functions, shell builtins, Misa utilities and the session's own
  `PATH`. This is the one-word escape from a shadowing Misa utility; it is a
  deliberate Misa extension of the POSIX `-p` search, which normally still
  prefers builtins. A miss returns 127 with `command: name: not found`.

`command name` bypasses functions (POSIX) but not builtins, so `command rev` still
runs the Misa utility, and `command -v`/`-V` report what would run. The working
directory never silently precedes `PATH`; only an empty or `.` `PATH` entry
selects it (F06 rules).

### Introspection output

`type name…` writes one line per operand: `name is a shell keyword`,
`name is a function`, `name is a shell builtin`, `name is a misa builtin`, or
`name is /absolute/path`. A name that is not found writes
`type: name: not found` to standard error and makes the status 1; operands that
are found are still reported. `type -t` prints one token instead: `keyword`,
`function`, `builtin`, `misa` (a Misa registry utility, deliberately distinct
from a shell builtin) or `file`. `type -p`/`-P` force a `PATH`-only search and
print the path, ignoring keywords, functions, builtins and the registry. `type -a`
and `-f` are **unsupported**: they print `type: -a: unsupported option` and exit
2 rather than faking a list, and `--help` is reported as an invalid option. This replaces the interpreter's own `type`, whose
`-a` returned 3 with an unrelated `command:` diagnostic.

`command -v name…` prints the name itself for keywords, functions, shell builtins
and Misa utilities, and the resolved path for a `PATH` executable, matching the
`type` classification; a name that is not found prints nothing and makes the
status 1, while found names are still printed. `command -V` prints the same
lines as `type`. `-v`/`-V` never execute anything. An unknown option prints
`command: invalid option "-q"` and exits 2. `command` with no operands succeeds
silently.

### `hash`

`hash` records `PATH` resolutions in a per-session table. `hash name…` resolves
each name (a `PATH`-only lookup), reports `hash: name: not found` with status 1
for a miss while keeping the names it did record, and prints nothing on success.
`hash -r` clears the table, and a `PATH` assignment clears it implicitly: entries
carry the `PATH` they were resolved with and a differing `PATH` drops them.
Bare `hash` prints the table as sorted `name=path` lines. Names that are shell
builtins, functions or Misa utilities have no path and are neither recorded nor
reported as missing; an operand containing a slash is not a bare command name and
is reported as such instead of being stored. Executing an external command records its resolution
automatically; explicit paths and builtins never are.

A hash entry is a record, not a cache used to launch: execution always
re-resolves through the injected handler, so a stale entry can never run a
different file. This is a deliberate difference from shells that execute the
hashed path.

### Mechanism and limits

The interpreter owns `type`, `hash` and `command` and its builtins cannot see the
registry. The session installs a `CallHandler` that rewrites those three names to
the reserved `__misa_type`, `__misa_hash` and `__misa_command` before
function/builtin dispatch, and its exec handler implements them. A function with
one of the three names is never rewritten, so functions keep precedence; the
private names are reserved and should not be defined. `exec type`, `exec hash`
and `exec command` are recognised by the exec handler as well, because `exec`
reaches it without the call handler. The interpreter's own `builtin` builtin
still calls its internal `type`/`hash`/`command`, so `builtin type x` reports
without registry awareness — an explicit, documented gap that belongs with the
S11 special-builtin work.

Nothing else changes: aliases (S14), `read`/`getopts` semantics (S11),
redirection and pipeline lifecycle (S09/S10), traps, job control and terminal
ownership (S15–S18), the interactive worker (S05) and TUI wiring (S06) remain
unwired, and this is not a claim of POSIX or Bash parity.

### Verification

Native tests cover the resolution order and its collisions (function vs shell
builtin vs Misa utility vs `PATH`), `command` bypass, `command -v`/`-V`/`type`
output and status, `type -t`/`-p`/`-P`, unsupported options, `hash`/`hash -r`/`PATH`
reset/auto-recording, `command -p` forcing the external, explicit paths, 126 for a
non-executable file and 127 for a missing one. They are hermetic: the session
tests inject fake registry and path lookups, and the adapter tests create their
fake executables by linking the test binary into a temporary directory, skipping
where exec permission is unavailable. `shcli` re-runs the same paths through
`misa sh -c`.

## S07 — directory and variable semantics

S07 hardens the directory and variable behaviour of the S02 session rather than
adding an execution surface. It changes the in-tree interpreter where POSIX
requires behaviour that was missing, and it exposes directory changes to a UI
adapter. Everything below is covered by hermetic tests in
`internal/shell/session` (`dirs_test.go`, `vars_test.go`) that use temporary
directories, symlinks, explicitly empty environments and injected handlers.

### `cd`

`cd [-L|-P] [directory]` follows POSIX:

- `-L` (the default) resolves relative operands against the logical `$PWD` and
  removes dot-dot components textually, so `cd link; cd ..` returns to the
  directory that holds `link`, and `cd -L link/..` names that directory.
- `-P` resolves symlinks before and while stepping through dot-dot components,
  so the same commands land on the physical parent.
- `--` ends option processing; combined forms such as `-PL` are not options and
  are treated as operands (as in POSIX `cd`, which takes a single option).
- `cd` without an operand uses `$HOME` and fails with status 1 and
  `cd: HOME not set` when `HOME` is unset or empty.
- `cd -` uses `$OLDPWD`, writes the new directory, and fails with status 1 and
  `cd: OLDPWD not set` (writing nothing) when `OLDPWD` is unset.
- With `CDPATH` set and a relative operand whose first component is not `.` or
  `..`, each colon-separated entry is tried (an empty entry means `.`). The
  first directory wins, and `cd` writes the new directory unless the match came
  from `.`. Absolute operands and operands starting with `.` or `..` never
  search `CDPATH`.
- A directory that does not exist, is not a directory, or is not searchable
  fails with status 1, a diagnostic on standard error, and no state change. More
  than one operand fails with status 2.
- `PWD` and `OLDPWD` are updated and exported after every successful change, so
  child commands inherit them; `PWD` is exported from session creation.
- The session never changes the process working directory: `Dir()` is shell
  state only, and no process environment variable is touched.

### Cwd-change events

`Options.OnCwdChange func(oldDir, newDir string)` is optional. Every successful
directory change reports its old and new logical directory in the order it
happened; the events of one submission are queued by the interpreter and
delivered after the submission finishes, outside the run lock, so the callback
may call `Dir()` but must not call `Run`. A failed `cd` reports nothing, `cd .`
reports nothing, and a change inside a subshell or process substitution never
reaches the callback because the interpreter hands its subshells no hook. S06
can therefore mirror the shell directory into the pane without polling.

### Variables and assignments

- Assignment-only commands take their status from the last command
  substitution they performed, or zero when they performed none
  (`x=$(exit 3)` is 3, `false; x=1` is 0).
- An assignment prefix supplies an invocation-local environment: `VAR=x cmd`
  exports `VAR` for that command only, including interpreter builtins
  (`HOME=/elsewhere cd`) and injected handlers, and never persists. A preceding
  assignment of a different value is overridden for that invocation.
- Assignment-only commands do not export their variables; `export` is required.
- `export NAME=value` and `unset NAME` (`-v`, `-f`) work as before. Bare
  `export`, `export -p`, `readonly` and `readonly -p` now write the matching
  variables as `export NAME=value` / `readonly NAME=value` lines with
  re-evaluable quoting, so `eval "$(export -p)"` round-trips.
- A readonly variable rejects re-assignment and `unset` with status 1 and one
  diagnostic, and keeps its value. `readonly NAME` marks a variable that does
  not exist yet, and a later assignment to it fails.
- Unsupported declaration options still fail with status 2
  (`export -Z x`, `declare: invalid option`) rather than being ignored.

### Known differences

- A leading assignment before a declaration keyword is not parsed as a
  declaration: `VAR=x export NAME` fails with status 2 and
  `export: unsupported builtin` instead of persisting the assignment. POSIX
  makes assignments before special builtins stick; that needs a parser and
  runner change, scheduled with the remaining variable work rather than faked
  here.
- `export -n` (a Bash extension, not POSIX) is accepted and ignored by the
  declaration parser instead of un-exporting.
- The session's logical directory, not a possibly modified `$PWD`, is the base
  for relative operands, and `OLDPWD` records the directory the shell actually
  left. Shells differ here; using shell state keeps a forged `PWD` from
  redirecting `cd`.
- `cd` with `-L` verifies the textually cleaned path, so an operand whose middle
  component does not exist (`cd missing/..`) succeeds when the cleaned path is a
  directory. Some shells reject that form; the POSIX algorithm keeps it.
- The interpreter bootstraps `HOME` (from the host user when the session
  environment has none) and `UID`/`EUID`/`GID`. A session that must not see the
  host `HOME` should `unset HOME` or supply its own value; passing `Env: []string{}`
  avoids inheriting the rest of the process environment.
- `readonly` output skips array variables (arrays are a Bash extension; POSIX
  declarations are strings) and prints declared-but-unset names without a value.

### Provenance

The interpreter changes are a recorded Misa patch of the imported foundation:
`internal/sh/SOURCE.json` lists the patched files (`interp/api.go`,
`interp/builtin.go`, `interp/runner.go`) under the `posix-dirs-vars` local patch,
`internal/sh/NOTICE` summarises them, and `internal/inventory/sh_source_test.go`
verifies the recorded bytes and that every patched file is declared. No upstream
test or external implementation was copied; the behaviour follows POSIX and the
documented `cd` semantics only.
