# S02 — persistent shell session API

This implements the library foundation for issue #40, on top of S01's in-tree
mvdan/sh and F02's invocation contract. It does **not** change command dispatch,
add `misa sh`, or claim full POSIX shell compatibility.

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
S07–S14 own detailed shell semantics, redirection/pipeline lifecycle, source/eval
and aliases. S15–S18 own process-local state, traps and real job control. Existing
editor/menu/agent/platform callers remain for S19–S21 and the S22 rollout gate.

Native tests cover persistent state/status, independent sessions, unchanged
process cwd/environment, typed all-or-nothing parsing, cancellation/recovery,
exit/close, snapshot ownership, handler invocation, and the F02 adapter. A
session dependency test rejects a transitive Bubble Tea dependency. Imported
foundation patches are recorded in `internal/sh/SOURCE.json` and `NOTICE`; no
second shell copy or dependency is introduced.
