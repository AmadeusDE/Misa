# F02 — builtin invocation contract

This is the current foundation for issue #40, not a claim of shell or utility parity. Builtins remain independent of the shell runtime; composition lives in `internal/builtinexec`.

## Invocation

`builtins.Context` carries `Args`, `CmdName`, `Dir`, streams, `Env`, `Ctx`, terminal dimensions and per-stream TTY flags, and an optional `Exec` callback. Callers supply the terminal metadata; it is not inferred by `Run`. `CancellationContext()` returns `Ctx`, or `context.Background()` when absent.

A nil `Env` inherits the process environment; a non-nil empty slice means an empty environment. Use `Environ`, `LookupEnv` and `Getenv` for invocation-local access. `Environ` returns a snapshot. Legacy direct process-environment reads are not yet isolated.

`builtins.Run(ctx, name, inv)` accepts a nil invocation and copies the context and its argument and environment slices. A nil `ctx` uses the invocation's cancellation context. It makes `Dir` absolute, substitutes empty input and discard output for nil streams, and sets `CmdName` to `name` only when blank. Unknown builtins return 127. Stream objects themselves are shared, not copied.

`Executor` is `func(context.Context, string, *Context) int`, a neutral nested-execution capability. `builtinexec.Run` composes exact builtin lookup with external execution using invocation-local cwd, environment, streams and PATH; it is not a complete shell launcher. `builtinexec.For` selects the default callback. The multicall entry point retains the OS default SIGINT handling; signal-to-context bridging awaits F05 so that unconverted builtins still interrupt normally.

`ExitStatus` maps cancellation to 130 and deadline exceeded to 124, and supplies success, failure, usage, internal, cannot-execute and not-found constants (0, 1, 2, 125, 126, 127). Process signal statuses use the 128 base where supported. Individual wrappers retain their own status semantics, including timeout's 124 and launch failures 125/126/127.

## Boundaries and follow-up

- **F03:** `env` supports an injected executor and local environment, but its real default via `builtinexec.For` deliberately has `Exec == nil`: the legacy process-environment bridge remains until direct environment readers migrate. That bridge is unsafe for concurrent invocations; do not treat this contract as environment isolation today.
- **F04:** `nice` supports injection but still changes process/thread priority; move process effects to an isolated child capability.
- **F05:** `sleep` cooperates with cancellation. `yes` and broad streaming/copy loops have not been migrated; cancellation does not imply that all in-process work has stopped.
- **F06:** `timeout` retains legacy dispatch and signal/group handling, including `-s`, `-k`, `-p`, `-f` and ENOEXEC concerns. An integer-returning executor cannot preserve those signal semantics; no injected-timeout guarantee is provided. The `sort` external compressor path was inspected and is also left for F06. Unify launcher resolution, signal supervision and cleanup there.
