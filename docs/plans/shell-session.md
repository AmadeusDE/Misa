# S02 — persistent shell session API (with S07 directory and variable semantics)

This implements the library foundation for issue #40, on top of S01's in-tree
mvdan/sh and F02's invocation contract. It does **not** change command dispatch,
add `misa sh`, or claim full POSIX shell compatibility. The S07 section at the
end covers the directory and variable semantics added on top of it.

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
  host `HOME` should `unset HOME` or supply its own value; passing `Env: nil`
  already avoids inheriting the rest of the process environment.
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
