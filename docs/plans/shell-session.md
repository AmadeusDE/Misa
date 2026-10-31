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

## S05 — persistent worker and control protocol

### Prototype decision and scope

S05 selects a **persistent child Misa process per pane** as the production
transport. `worker.StartProcess` re-executes the Misa multicall binary as
`misa sh --worker`; it never starts a host shell. A child gives process-local
mutations a separate home and can be killed when cooperative cancellation is
insufficient. This is a library/CLI foundation, not TUI wiring: S06 will create
and own workers from panes. Plain `misa` and all existing builtin dispatch remain
unchanged.

`worker.Start` is the smaller, in-process prototype and portable test transport.
It uses the same framing and session owner over `net.Pipe`, not a second parser
or a state-reconstruction mechanism. It remains useful for audited handlers and
captured scripts, but cannot isolate process-global changes or forcibly stop Go
code. No external implementation was consulted or copied for this orchestration;
the existing attributed in-tree interpreter remains unchanged.

### Channels and lifecycle

The child transport supports Unix, including Android. The parent creates a
socketpair and passes one endpoint as descriptor 3. The exact `sh --worker`
entry point converts that descriptor to an owned control connection; it is not
script input. Command stdin/stdout/stderr remain three separate streams supplied
by the caller (and can later be attached to a PTY). Command output, including
binary data or JSON-looking text, is never parsed as control traffic. Child
launch is explicitly unsupported on other platforms; the in-process transport
is portable. There are no startup files or implicit project configuration.

Control protocol version 1 uses a four-byte big-endian length followed by a JSON
object, with a maximum payload of 1 MiB. The maximum includes JSON escaping and
metadata, not just script bytes. Both endpoints exchange `hello` with `version`
and `capabilities` before accepting commands. Supported messages are:

- `run {id, script}` and `result {id, status, exited, error, syntax}`;
- `cancel {id}` for the active submission;
- `shutdown`, followed by an `exit` report;
- `crash {status}` for a recovered worker failure.

The client serializes submissions, assigns IDs, and keeps its submission slot
until the prior response is consumed, even after the caller cancels. The server
allows only one active submission; that submission alone owns the persistent
session. Functions, unexported variables, exports, options, cwd and status are
retained, not copied back out of one-shot shells. Ordinary command failures stay
shell statuses; syntax, cancellation, exited-session and runtime failures are
separate from control/worker failures. A canceled request does not roll back
already changed shell state.

The child is also supervised with `exec.Cmd.Wait`, independently of control EOF:
a worker exit cannot strand a request merely because another process retained a
control descriptor. Abnormal process exit has a typed `worker.ExitError` and
subsequent requests fail. Recovered panics are reported without including the
panic payload (which might contain command data). Fatal process failures outside
the recoverable owner are detected by the parent wait monitor. Workers are not
restarted automatically: silent restart would lose pane state.

A `result` reports interpreter completion, **not** an output-drained barrier.
The raw I/O and control channels are independent: parent-side output copying may
lag the result. Supplied writers must be safe for asynchronous access; a plain
`bytes.Buffer` must not be read concurrently with a copy pump. Caller-owned I/O
that never returns can leave its copy goroutine blocked, but cannot hold up child
reaping or bounded shutdown. Use file/PTY streams or cooperative I/O in production.
**S06 TODO:** handle output-vs-result ordering explicitly, for example by draining
to quiescence before redrawing a prompt; do not assume receiving a result means
all screen bytes have already arrived.

Shutdown is idempotent. It requests cancellation and graceful termination, then
the child transport bounds waiting and escalates to process kill/reaping. The
caller can set a shorter context deadline. The in-process transport cannot force
an uncooperative handler or caller-owned blocking I/O to return; shutdown can
seal its control connection but must not be described as having stopped that Go
code. Streams remain caller-owned and are never closed by the worker.

### Migration boundaries

No existing interactive execution surface is migrated in S05. S06 owns pane
creation, prompt submission, state reflection and user-visible worker failure
handling. This control channel is not a job-control protocol: PTY foreground
ownership, traps, process groups shared by pipelines, stop/continue and full job
supervision remain S09/S10 and S15–S18. Existing F03/F04/F05 process-state and
cooperative utility limitations still apply; killing a worker is not a claim
that arbitrary descendant jobs are supervised. Editor, agent and platform
execution surfaces remain S19–S21, and the default rollout gate remains S22.

### Verification and cost

Native tests and benchmarks accompany the implementation. Results and exact
commands are recorded below after verification; cross-vet is type checking,
not a claim of runtime behavior on those systems.

Constructors (`Start`, `StartProcess`, `Connect`) use their context for startup
and handshake only. Per-command contexts cancel individual submissions; explicit
`Shutdown` owns the worker lifetime. `Serve`'s context, in contrast, owns that
server's lifetime. The child shutdown timeout defaults to one second and may be
configured; the in-process caller must provide a deadline if its handler might
not cooperate. Canceling during a blocked control write seals the client, since
a partially written frame cannot safely be reused. On control EOF the child
transport allows up to 100 ms for independent process reaping to supply an exact
`ExitError`; if a still-live worker closes its socket, the original transport
error is returned instead of waiting indefinitely. A signal-terminated process
has `ExitCode() == -1`, with the underlying `*exec.ExitError` available through
`errors.As`; a recovered owner panic uses status 125.

Measured on Linux/amd64, Intel Xeon VM, Go 1.27.3, `CGO_ENABLED=0`, with no race
instrumentation (short samples, not performance guarantees):

```sh
env -u OPENROUTER_API_KEY CGO_ENABLED=0 go test ./internal/shell/worker \
  -run '^$' -bench . -benchmem -benchtime=100x -count=3
CGO_ENABLED=0 go build -o /tmp/misa-worker-cost .
```

| Measurement | Observed cost |
|---|---:|
| In-process startup + graceful shutdown | 55–68 µs; about 32 KiB allocated |
| In-process `true` round trip | 14–16 µs; about 5.6 KiB allocated |
| Test-binary child startup + graceful shutdown | 2.67–2.74 ms |
| Test-binary child `true` round trip | 41–49 µs |
| Idle test-binary child RSS | about 12,288 KiB per child |
| Full Misa binary spawn through handshake (30-run median) | 15.29 ms |
| Full Misa binary `true` round trip (30 × 100 commands, median) | 37.17 µs |
| Full Misa idle RSS delta versus no worker | +26,964 KiB per child |
| RSS growth after state setup and 100 `true` requests (median) | +1,066 KiB |

The test-binary startup benchmark includes shutdown and process reaping; it
**underestimates full-binary initialization**. The full-binary probe launched
`/tmp/misa-worker-cost sh --worker` directly with an empty `PATH`, descriptor 3
connected to a Unix socketpair, and stdio attached to `/dev/null`. It timed the
version-1 hello round trip, sampled `/proc/<pid>/status` `VmRSS`, submitted three
state/setup/check requests and 100 `true` requests, sampled RSS again, then sent
shutdown and waited for exit. The state checks exercised functions, cwd and
`printenv` across requests. No host shell was used. RSS is resident memory, not
private/PSS memory: shared executable/runtime pages may be counted in more than
one process, so these deltas must not be summed as an exact physical-memory
budget. There is no per-pane RSS for the goroutine prototype because it shares
the process; its allocation numbers above are not a resident-memory estimate.
The child cost is material enough that S06 should avoid speculative idle worker
creation and measure actual pane workloads before optimizing this protocol.

Final checks passed: `gofmt -l .` (empty), `go vet ./...`, `nough` and combined
`nogui noweb` builds, focused builtins/shell tests, full tests with
`OPENROUTER_API_KEY` unset, and shell race tests. The vet/build/full-test checks
also passed with `CGO_ENABLED=0`. OpenBSD, FreeBSD and Darwin builtins plus shell
cross-vet passed; Windows builtins were already failing baseline type checking
in unrelated platform code, but the standalone worker package cross-vet passed.
Android/arm64 `CGO_ENABLED=0` shell builds passed. No Android emulator, Android
NDK/cgo build, or real-PTY/job-control runtime test is claimed by S05. Full tests
passed without needing the known flaky-test reruns. Binary smoke checks covered
`echo`, `printf`, `rev`, `sh -c`, invalid worker descriptor handling and the
persistent full-binary protocol probe.
