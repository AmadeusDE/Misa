# F02/F03 — builtin invocation and environment isolation

This is the current foundation for issue #40, not a claim of shell or utility parity. Builtins remain independent of the shell runtime; composition lives in `internal/builtinexec`.

## Invocation

`builtins.Context` carries `Args`, `CmdName`, `Dir`, streams, `Env`, `Ctx`, terminal dimensions and per-stream TTY flags, and an optional `Exec` callback. Callers supply the terminal metadata; it is not inferred by `Run`. `CancellationContext()` returns `Ctx`, or `context.Background()` when absent.

A nil `Env` inherits the process environment; a non-nil empty slice means an empty environment. Use `Environ`, `LookupEnv` and `Getenv` for invocation-local access. `Environ` returns a snapshot. Registered builtin environment reads use these accessors rather than modifying or reading the process environment directly (apart from the inherited fallback).

`builtins.Run(ctx, name, inv)` accepts a nil invocation and copies the context and its argument and environment slices. A nil `ctx` uses the invocation's cancellation context. It makes `Dir` absolute, substitutes empty input and discard output for nil streams, and sets `CmdName` to `name` only when blank. Unknown builtins return 127. Stream objects themselves are shared, not copied.

`Executor` is `func(context.Context, string, *Context) int`, a neutral nested-execution capability. `builtinexec.Run` composes exact builtin lookup with external execution using invocation-local cwd, environment, streams and PATH; it is not a complete shell launcher. `builtinexec.For` selects the default callback. The multicall entry point retains the OS default SIGINT handling; signal-to-context bridging awaits F05 so that unconverted builtins still interrupt normally.

`ExitStatus` maps cancellation to 130 and deadline exceeded to 124, and supplies success, failure, usage, internal, cannot-execute and not-found constants (0, 1, 2, 125, 126, 127). Process signal statuses use the 128 base where supported. Individual wrappers retain their own status semantics, including timeout's 124 and launch failures 125/126/127.

## F03 — invocation-local environment

`env` computes a non-nil child environment, including an empty slice for `-i`, and passes it through `Context.Env`. It never clears, sets, or restores process environment variables. `builtinexec.For("env")` now supplies the normal executor, so nested builtins and external children receive the same computed environment and directory. The compatibility paths also pass that environment without mutation. `env` retains its legacy preference for an executable regular file in its working directory before nested dispatch; resolution policy and cwd poisoning remain for F06. The shared executor otherwise retains builtin-first lookup.

The audit migrates backup controls (`cp`, `mv`, `ln`), `echo`/`printenv`, PATH lookup, terminal dimensions/colors, locale selection, HOME/login names, temporary directories, and time handling. `install` has no backup-environment implementation to migrate; adding utility flags remains U14. `sshc` carries environment into its SSH options (HOME, USER, SSH_AUTH_SOCK), `vi` passes it to shell child processes, and `devices` resolves storage/configuration paths from invocation HOME/XDG variables. Public web server/admin and editor APIs still inherit process state by default.

`Context.Location()` resolves invocation TZ without changing `time.Local` or caching global state. Unset TZ uses `time.Local`; empty or invalid TZ uses UTC. Zoneinfo names (including names such as EST5EDT when installed), a leading `:`, and absolute zoneinfo files are supported. It does not parse arbitrary POSIX TZ rule strings. `date` retains its existing timestamp-aware POSIX TZ parser for compatibility; extending that behavior consistently to the other utilities is future work. `ls`, `stat`, `touch`, `cal`, and diff headers use the shared location helper.

Concurrent invocation tests exercise conflicting PATH, TZ, backup suffixes and custom variables, including nested `env`, and assert the process environment remains unchanged. An AST guard prevents new direct process-environment access in builtin sources and vi/mweb entrypoints, with narrowly named exemptions for inherited fallback and process-level web APIs.

## Boundaries and follow-up

- **F04:** `nice` supports injection but still changes process/thread priority; move process effects to an isolated child capability.
- **F05:** `sleep` cooperates with cancellation. `yes` and broad streaming/copy loops have not been migrated; cancellation does not imply that all in-process work has stopped.
- **F06:** `timeout` retains legacy dispatch and signal/group handling, including `-s`, `-k`, `-p`, `-f` and ENOEXEC concerns. An integer-returning executor cannot preserve those signal semantics; no injected-timeout guarantee is provided. The `sort` external compressor path was inspected and is also left for F06. Unify launcher resolution, signal supervision and cleanup there.

- **Vi follow-up:** The registered `vi` builtin passes the invocation environment to its shell commands, but the terminal still uses process streams. Shell working directories depend on the edited filename (or process cwd), not consistently on the invocation directory. Cancellation is not propagated. These need a separate design without changing the UI frontend inadvertently.
