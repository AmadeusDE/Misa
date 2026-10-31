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
