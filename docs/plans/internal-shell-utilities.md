# Self-contained shell and basic utilities: research and PR roadmap

Research date: 2026-10-30. Misa baseline: `2e11a987d15c186bfc803cb73ea8c9bbdc22d46f`.
Status: proposed implementation backlog; no implementation is included in this research.

Tracking issue: [Misa #40](https://github.com/AmadeusDE/Misa/issues/40).
Complete pinned source and command inventory: [research appendix](https://github.com/AmadeusDE/Misa/issues/40#issuecomment-6527314348).

## Decisions confirmed with the maintainer

- **Self-contained shell**, rather than strictly one-process execution. External applications and Misa child processes remain supported.
- **POSIX first**, with selected Bash features later. This is an engineering target, not a claim of POSIX certification.
- **Broad staged utility backlog**: daily-use scripting first; networking, administration, devices and services later.
- **C-locale baseline first**; explicit Unicode and locale work follows.
- **Bring mvdan/sh code into this repository** and maintain it here, because substantial additions are expected. Do not design around public APIs being the permanent limit.
- External implementation and imported-test references are limited to **Toybox**, **MIT-licensed uutils material**, and **mvdan/sh for shell code**. Existing dependencies and standard-library APIs may be used without researching their implementation.
- This tracking issue is an explicit, one-time exception to the repository's no-plan-only-issues rule. Individual implementation PRs should reference its task IDs; no issue per PR is necessary.

## Executive findings

1. Misa has **115 registered multicall command names**, verified by a default-tag build of this revision and running `misa --list`. That is registration, not a claim of full compatibility; feature-tag builds can expose a different registry.
2. The approved utility repositories expose **407 distinct declared names** in the enumeration described in the inventory. **30 Toybox `USE_SH` declarations** are separated as shell syntax/builtins/entry aliases. Of the remaining names, **278 are absent from Misa's multicall registry**. This includes aliases and UI-only `clear`, and is not 278 independent implementations.
3. **26 uutils/coreutils implementation-directory names are missing**: `chcon chgrp chroot csplit dd df dir dircolors install join mknod more nohup od pinky pr ptx runcon shred stdbuf stty test uptime users vdir who`. The `[` entry is additionally found through Toybox's alias declaration; do not lose it by enumerating only Rust directories.
4. The most useful missing daily-use families are **grep/find/xargs, sed, test/[, install/chgrp, df/dd/od, tar/gzip, ps/free/uptime, stty/more, patch/file**. AWK is valuable but is a language implementation, not a small command wrapper.
5. Shell integration is the first dependency. Misa currently sends quoting, expansions, pipelines, redirections, assignments and some unresolved commands to `sh -c`. Adding utilities alone will not make those paths self-contained.
6. Misa already depends on **mvdan.cc/sh/v3 v3.13.1**, but only uses its parsing APIs in agent policy-related code. Its interpreter is a substantial head start, not a complete interactive shell.
7. The hardest gaps are **persistent pane-local state, process/terminal job control, signal traps, descriptor inheritance, process-local effects and cancellation**. These need dedicated PRs and executable acceptance tests before changing the default engine.
8. Existing `env` clears and rewrites the process environment around nested builtins; `nice` changes priority in its own process/thread context. `Context` has no cancellation or execution capability. These are immediate blockers to generally safe goroutine execution.
9. Existing coverage documents are stale and contradictory: `builtins-coverage.md` lists already-registered tools such as `shuf`, `timeout`, `mkfifo`, `basenc`, `xxd`, hashes and encoding helpers as missing. The new inventory supersedes their missing-command conclusions for this work.
10. **uutils/awk and uutils/diffutils are Apache-2.0**, according to repository metadata checked during this research. They were not cloned or used as external code/test references. Toybox supplies the permitted AWK and diff references. An MIT repository can still contain separately licensed fixtures; repository-level MIT is not blanket permission to import every file.

## Evidence and reproducibility

The companion `internal-shell-utilities-inventory.md` records full source revisions, the complete current registry, all 278 unmatched names, source locations, candidate test files, and a disposition for every name. No other project's implementation was used as an external research source. Existing Misa architecture and documentation were inspected as local project evidence.

Method:

1. Build Misa at the baseline and obtain its runtime registry with `--list`.
2. Enumerate uutils organization repository metadata and inspect the LICENSE files of the MIT utility repositories before using them. Exclude Apache-only repositories and non-utility infrastructure repositories from the utility union.
3. Pin every approved checkout to its actual full Git SHA. Toybox was still at `6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb`; no freshness assumption was made from its version number.
4. Read Toybox `USE_*(NEWTOY/OLDTOY(...))` declarations under `toys/*/*.c`, excluding `toys/example`. Keep aliases, pending implementations and default-off applets visible; declaration is not evidence of maturity.
5. Read uutils `src/uu` implementation directories, excluding `checksum_common`; use the four shipped findutils binaries rather than its `testing-commandline` helper. Single-command grep/sed repositories contribute their command; `red` is another `sed` implementation, not another utility.
6. Separate all Toybox `USE_SH` declarations, including `[[`. `:` is separately declared by Toybox and remains in the unmatched-name count, but is assigned to the shell workstream.
7. Record candidate upstream tests by file existence. **No test-count heuristic is used as delivered coverage or as an effort estimate.** Native fixtures and imported fixtures require individual provenance review before implementation.
8. Inspect the installed mvdan v3.13.1 source separately from current upstream. Run small, native-authored probes against the pinned module with a rejecting external-exec handler; these probes cannot silently use a host utility.

### Pinned-interpreter probes actually run

All were parsed with `syntax.LangPOSIX`, with a synthetic PATH, using v3.13.1. These are observations, not extrapolations from current upstream.

| Input | Observed result |
|---|---|
| quoted `printf`, assignment, expansion | Correct `a b` and `ok` output |
| `trap 'echo trapped' INT` | Invalid signal specification; status 2 |
| `trap 'echo done' 0` | Invalid signal specification; status 2 |
| `umask 077` | Unsupported builtin; status 2 |
| `ulimit -n` | Unsupported builtin; status 2 |
| `jobs; fg; bg` | Each unsupported; final status 2 |
| `type cat; command -v cat`, with an exec handler but no PATH executable | Does not discover the virtual utility; final status 1 |
| `alias hi='echo hi'; hi` | `hi` reaches the exec handler under this configuration; alias behavior needs explicit option/parse-boundary tests |

Current upstream at `e96ef1dae52c71e86cf87eb594cb4917fa49aaba` has additional job machinery, but `fg`/`bg` still report no job control and `trap` still only accepts `ERR`/`EXIT` in the inspected implementation. Its default ENOEXEC fallback creates a new runner with default handlers; that would lose Misa's resolver. This is a reason to choose and test the import revision deliberately, not blindly take either the old dependency or HEAD.

### Local execution surfaces that need migration

| Surface | Evidence at baseline | Required disposition |
|---|---|---|
| Prompt dispatch | `internal/ui/dispatch.go:36-58`; `shell.go:48-68` | Replace `strings.Fields` plus character heuristics with one parser/resolver path. Unknown commands return 127 without launching another shell. |
| Stateful prompt commands | `internal/ui/app.go:1145-1253` | `cd`, aliases and UI state currently have ad hoc handling; make shell state authoritative and reflect it into the UI. |
| PTY execution | `internal/term/terminal.go:180-228` | `StartCommand` launches sh; `StartArgv` supplies global environment; add explicit execution environment and cancellation/lifecycle contracts. |
| In-process interruption | `internal/term/terminal.go:209-242` | Currently abandons the goroutine and closes its PTY. That is not proof that work stopped. |
| Job supervision | `internal/jobleader/jobleader.go`; `internal/ui/jobs.go` | Existing one-command/session logic is reusable, but not a pipeline process-group/job-table implementation. |
| Agent fallback | `internal/ui/agent.go:217` | Route fallback command strings to the internal engine without changing embedded-agent routing. |
| Editor commands | `internal/vi/command.go:499,543,560` | Migrate shell escapes, reads and filters with separate stdout/stderr and exit handling. |
| Agent shell tool | `internal/ugh/tools/bash.go:103-128` | Replace `/bin/sh -c` while preserving timeout, process-group cleanup, bounded output, scratch environment and background tracking. |
| Package-manager action | `internal/pkgmanager/pkgmanager.go:150` | Internalize the wrapper expression; package-manager binaries remain intentional external programs. |
| Legacy launcher | `internal/launcher/launcher.go` | Remove or explicitly migrate the unused host-shell fallback. |
| Utility wrappers | `internal/builtins/env.go`, `nice.go`, `timeout.go`, `sort.go` | Use one injected execution service, including ENOEXEC and builtin lookup; retain intentional external compressor execution only when requested. |
| Configuration, web/GUI/Android entry points | main dispatch, `Bootstrap`, frontend spawn paths | Make internal shell entry points available everywhere, including `nough`, `nogui`, `noweb`, Android CLI and APK. |
| Agent parser/policy | `internal/ugh/perm/static/{shell,triage,commands,git}.go`; `internal/ugh/agent/critical.go` | Move analysis onto the same in-tree syntax types and dialect as execution. Preserve the no-exec/no-syscall dependency rule of static analysis. |

External `git`, package managers, build tools, clipboard helpers and font discovery are distinct optional/application dependencies. Self-contained shell execution does not mean implementing all these applications. `#!/bin/sh` in a third-party program launched outside Misa cannot be redirected merely by changing Misa's PATH; the internal launcher can recognize supported shell scripts, but replacing a system interpreter path is a deployment decision.

## Architecture proposal

### Import boundary and dependency direction

Import the needed mvdan package graph under a clearly attributed in-tree prefix, for example `internal/sh/{syntax,expand,interp,pattern,...}` plus its required internal helpers. Keep the imported foundation recognizable and keep Misa-specific orchestration in a separate `internal/shell` package. The import PR records the exact source revision, original-to-local paths, BSD-3-Clause license, file hashes and deliberate omissions. Retain per-file copyright notices, add `NOTICE` and `docs/CREDITS.md`, and record local patches. Do not mechanically copy unrelated CLIs, downloaded tests or generated third-party material.

Move all current mvdan parser consumers onto that same package graph. Avoid a permanent split where permission analysis parses one dialect/version and execution runs another. Preserve existing static-analysis policy behavior while making supported syntax explicit. Keep formatter functionality only where required by AST printing, diagnostics or existing consumers; do not remove dependencies merely because their names sound optional.

The runtime must **not import the builtin registry directly** if builtins such as `vi`, `env`, `find`, `xargs` or `awk` need to call the runtime. Use a small neutral execution contract and inject registry/external execution at the composition root. A viable direction is:

`shared execution types <- builtin implementations / shell runtime <- CLI and UI composition`

`in-tree syntax <- shell runtime and ugh static analysis`

UI callbacks are adapters; libraries do not import `internal/ui` or `internal/ugh`. The interpreter is not itself a security sandbox.

### Execution and state

- One serialized shell state owner per pane/session; no concurrent calls on one mvdan Runner. It owns variables, exports, functions, positional parameters, options, aliases, logical cwd, previous cwd, umask, job table and status.
- Prefer a **persistent Misa shell worker process per interactive pane**, with a separate control channel for commands/state/events and a PTY for user program I/O. This gives process-local signal/resource state a home without modifying the GUI/TUI process. Validate this in a spike before committing to its protocol. A new `misa sh -c` per submitted line would lose functions and unexported variables and is not a solution for the interactive prompt.
- Keep plain library/captured execution available for isolated script tasks. Goroutine-run utilities are opt-in based on capabilities and cancellation, not assumed safe because they have a `Run` function.
- Process groups belong to **jobs/pipelines**, not one independent group per pipeline stage. A worker must manage foreground terminal ownership, wait status, stop/continue, traps and cleanup coherently with the existing PTY supervisor.
- Distinguish a shell subshell's copied state from a real OS process. When `$!`, process signals, process-local limits or inherited descriptors require process semantics, use a deliberate worker/re-exec mechanism. Go cannot safely emulate arbitrary `fork()` by cloning goroutine state.
- Builtins receive invocation-local cwd, exported environment, stdin/out/err, context, tty information and an executor callback. Nil/inherited environment and deliberately empty environment must have different meanings.
- Start with explicit capability classifications: pure streaming; cancellable filesystem; terminal-interactive; child-process wrapper; process-state mutation. Unknown classifications default to isolated execution until audited.

### Resolution and language policy

Define and test precedence among special builtins, functions, regular builtins, Misa utilities and PATH programs, including `command`, `builtin`, `type`, `hash`, `env`, explicit paths and the current native escape. Preserve a documented way to choose an external utility. An explicit path must not fall back to a builtin with the same basename, and the working directory must not silently precede PATH.

The parser handles syntax; shell expansion produces argv; execution never re-splits argv with `strings.Fields` or interpolates it into a shell string. `misa sh -c`, script-file and stdin modes must define `$0`, positional parameters, exit statuses, diagnostics, startup-file rules and login/interactive behavior. Bare `!` is POSIX negation; the existing `! command` native escape conflicts and needs an explicit migration decision before default rollout.

`test` and `[` are needed both inside the shell and as multicall commands. `[[` is syntax, not an argv alias for `[`. `time` has a shell-keyword/pipeline role as well as a standalone utility role. `clear`, editor shortcuts and games may retain UI actions, but redirected or scripted forms need normal command semantics.

### Compatibility policy

- C locale first is not permission to corrupt UTF-8 or NUL-containing data. Specify byte-vs-character behavior command by command (`wc -c/-m`, `cut`, `tr`, regex character classes, display columns).
- Go's regular expression API is not a drop-in POSIX BRE/ERE implementation. BRE escaping/backreferences and leftmost-longest/submatch behavior need a shared, explicitly tested compatibility layer. `regexp2` already exists, but its different semantics and resource usage need evaluation; translating a few escapes is not a complete solution.
- Toybox and uutils sometimes disagree. For each command, record baseline flags, chosen extensions, unsupported features, exact exit statuses and known intentional differences. Never label a whole command "compatible" because its name exists.
- No fake success for unsupported options or unsupported platform operations. No host-tool fallback in internal-only mode. Features such as generic external-process `stdbuf` are conditional design work, not guaranteed Go wrappers.

## PR roadmap

IDs are stable tracking keys, not promised merge order. Each checkbox is a proposed independently reviewable PR with implementation and meaningful tests. A large task may split further, retaining the parent ID. **106 PR candidates** are listed below: 8 foundations, 24 shell tasks, 44 utility tasks and 30 optional expansion tasks. Early delivery does not wait for optional administration tools.

Every PR inherits the definition of done below. Dependencies list the principal prerequisites; a later compatibility PR also depends on the earlier implementation of that command.

### Foundation: F01–F08

- [ ] **F01 — Capability and provenance baseline.** Add a machine-readable command/alias/platform/capability inventory and audited source manifest; generate human docs from it. Test registry drift and distinguish UI-only names from executable commands. Depends: none.
- [ ] **F02 — Invocation contract and `builtins.Run`.** Introduce cancellation, explicit environment semantics, tty metadata, stable exit results and injectable nested execution without importing shell from builtins. Test empty env, separate streams, relative cwd and command-name propagation. Depends: F01.
- [ ] **F03 — Remove environment cross-talk.** Replace `env`'s `os.Clearenv`/restore and audit direct getenv users (`echo`, `printenv`, `cp`, `ln`, `ls`, `stat`, `touch`, etc.). Concurrent commands with conflicting PATH/TZ/backup settings must remain isolated. Depends: F02.
- [ ] **F04 — Process-effect isolation.** Route `nice`, umask readers/mutations and future credential/resource operations through a dedicated child capability. Test that the Misa parent and another pane keep their priority, environment and creation policy. Depends: F02.
- [ ] **F05 — Cooperative cancellation and stream termination.** Fix `sleep`, `tail -f`, `yes`, copy loops and blocked pipe paths; cancellation must stop work, not merely hide the PTY. Verify early-reader exit, closed stdin, bounded goroutine lifetime and status mapping. Depends: F02.
- [ ] **F06 — Shared command launcher.** Unify invocation-local PATH, explicit-path behavior, 126/127, signals, ENOEXEC, process cleanup and nested builtin dispatch for `env`, `nice`, `timeout` and later wrappers. Test cwd poisoning and empty PATH. Depends: F02–F04.
- [ ] **F07 — Honest upstream-test accounting.** Improve Toybox and MIT-uutils adapters with discovered/executed/skipped/failed reporting, fixture provenance and explicit skip reasons. Do not execute arbitrary imported setup on the developer host. Add native cases for constructs adapters cannot represent. Depends: F01.
- [ ] **F08 — Existing utility correctness debt.** Reproduce and classify current known failures; fix a bounded first batch of link/copy/move/env correctness issues and remove only verified resolved exclusions. Keep format/locale/random-generator differences separate. Depends: F03, F07; subsequent batches can split this task.

### Internal shell: S01–S24

- [ ] **S01 — Attribute and import mvdan foundation.** Choose v3.13.1 versus the researched newer revision by targeted regression results; import its needed package closure, license and source map. Move syntax consumers, preserving static-analysis dependency checks and fixtures. Depends: F01. No default engine switch.
- [ ] **S02 — Shell session API.** Introduce parser/runtime/session ownership and typed execution results, disconnected from Bubble Tea. Test state persisting across submissions and isolation between sessions. Depends: S01, F02.
- [ ] **S03 — `misa sh` noninteractive entry points.** Implement `-c`, script file, stdin, positional arguments, `$0`, syntax errors and status propagation. Add explicitly selected sh installation aliases without hijacking the TUI entry point. Test with no host shell. Depends: S02, F06.
- [ ] **S04 — Registry-aware resolution and introspection.** Integrate Misa utilities into execution and `command -v`, `type`, `hash`, `command` bypass behavior, explicit paths and native-command selection. Test builtins/functions/PATH collisions and executable permissions. Depends: S03.
- [ ] **S05 — Persistent interactive worker and control protocol.** Prototype, then implement, a pane-owned worker with separate control/I/O channels, startup handshake, crash reporting and shutdown. Demonstrate functions, exports and cwd surviving separate commands; measure startup/memory cost. Depends: S02–S04.
- [ ] **S06 — Replace prompt syntax heuristics.** Route normal/fullshell/mux prompt submissions through parser/session APIs; support incomplete multiline input and cancellation. Preserve editor/game/agent UI shortcuts through explicit adapters. Decide and test migration of `! command` versus POSIX negation. Depends: S05.
- [ ] **S07 — Directory and variable semantics.** Implement/test `cd -L/-P`, `CDPATH`, HOME/PWD/OLDPWD, assignments, export/unset/readonly, assignment-only status and temporary command environments; expose cwd-change events for S06's UI adapter. Depends: S02, F03.
- [ ] **S08 — Quoting and expansion contract.** Cover single/double quotes, escapes, IFS splitting, globs, parameter operators, arithmetic, quoted `$@`/`$*`, empty arguments and failed substitutions. Reuse imported behavior where correct; fix differences without reimplementing the parser. Depends: S03, S07.
- [ ] **S09 — Redirections and here-documents.** Implement/verify ordered `2>&1`, append, descriptor close/dup, noclobber, quoted delimiters, `<<-`, large here-docs and cleanup. Test exact bytes and descriptor leaks, including failed opens. Depends: S08.
- [ ] **S10 — Streaming pipelines and substitutions.** Mixed internal/external stages, command substitution newline stripping, pipeline exit status, broken pipes and bounded memory. `yes | head` and multi-megabyte pipelines must terminate cleanly. Depends: S09, F05.
- [ ] **S11 — Lists, control flow and functions.** Test/fix `&&`, `||`, `!`, if/case/for/while/until, functions, nested break/continue/return, and assignment scope. Verify short-circuit side effects and no ambient shell execution. Depends: S08–S10.
- [ ] **S12 — Source/eval/exec and script launch semantics.** Handle `.` lookup/arguments/return, eval parse boundaries, supported shebangs and ENOEXEC with the same resolver. Implement true child-process exec behavior where required; never replace the TUI process. Depends: S11, F06.
- [ ] **S13 — Input and option-processing builtins.** `read -r`, IFS/EOF, `getopts`, shift/set positional parameters and option state. Test stdin consumption alongside pipelines and sourced functions. Depends: S09, S11.
- [ ] **S14 — Alias and command-table semantics.** Implement POSIX alias expansion boundaries, unalias, command hashing invalidation and consistent help/type results; migrate config aliases without naïve string substitution. Test alias recursion and metacharacters. Depends: S04, S11.
- [ ] **S15 — Shell creation masks and limits.** Implement `umask` query/symbolic forms and `ulimit` in the worker, propagating creation policy to redirections/builtins/external children without touching other panes. Test parent isolation and subshell restoration. Depends: S05, F04, S09.
- [ ] **S16 — Signal traps and shell exit rules.** Add INT/TERM/HUP/CHLD handling, numeric names including trap 0, ignore/reset semantics, EXIT lifecycle, interrupted waits, and deliberate `set -e/-u` behavior. Dispatch trap ASTs on the state-owning loop, not arbitrary signal goroutines. Depends: S05, S11–S12.
- [ ] **S17 — Real pipeline job supervision.** Implement one process group per job, foreground tty ownership, launch barriers, per-stage wait accounting and stop/continue. Test a pipeline with external and Misa stages, avoiding competing waiters and orphaned group problems. Depends: S10, S16.
- [ ] **S18 — Interactive/background job commands.** Unify `jobs`, `fg`, `bg`, `wait`, `kill`, `$!` and UI job presentation. Test background read/tty behavior, Ctrl-C/Ctrl-Z, status retention, close-pane cleanup and multiple sessions. Define disown only as a later extension unless intentionally included. Depends: S17.
- [ ] **S19 — Migrate editor/menu/autorun callers.** Replace editor shell escapes and filters, package-manager shell expressions, autorun and agent fallback dispatch. Verify output separation, caller cwd, error propagation and pane message routing. Depends: S06, S12, S18.
- [ ] **S20 — Migrate ugh execution and policy alignment.** Run the agent shell tool through internal execution while preserving its external JSON contract, timeout/group tracking, output limits and scratch env. Test analysis/execution dialect agreement and sensitive-command classification; update tool descriptions. Depends: S12, S16–S18, F06.
- [ ] **S21 — Platform packaging and installed-script integration.** Wire CLI, GUI, web, Android CLI/APK and all build tags to internal entry points. Test executable path resolution on Android, termux cwd/env, denied procfs and child re-exec. Document shell-script launch boundaries. Depends: S19–S20.
- [ ] **S22 — Hermetic shell acceptance and default switch.** Run the supported shell suite in an image/root with no host shell or utilities, with a separate external-program fixture. Validate startup files, status and lifecycle, then switch the default engine; reject unsupported syntax instead of silently delegating. Depends: S21 and the baseline utility set needed by the suite.
- [ ] **S23 — Selected Bash extensions.** In a separate explicit dialect, validate `[[ ]]`, arrays, local/declare, arithmetic commands, process substitution, pipefail and supported option behavior. Add only features whose descriptor/process semantics are implemented. Depends: S22.
- [ ] **S24 — Interpreter maintenance and performance.** Add upstream-update documentation, source-map checks, parser/expansion fuzzing, cold-start/RSS and pipeline benchmarks; remove the external mvdan module only after every consumer migrates. Optimize from measurements and track in-tree fixes. Depends: S22; repeat selectively after substantive changes.

### Daily-use and core utility completion: U01–U44

- [ ] **U01 — `test` and `[`.** Expose one well-tested predicate contract to shell and multicall paths, including argument-count ambiguities, numeric errors, symlinks, `-t`, permissions and the closing bracket. Test true/false/error statuses separately. Refs: Toybox test; MIT coreutils test. Depends: F02, F07, S04.
- [ ] **U02 — Shared traversal substrate.** Extract policy-neutral walking, symlink/depth/error handling, cancellation and file metadata from Misa's existing search paths. Shell find must not inherit agent hidden-file exclusions or result limits. Test loops, unreadable entries and unusual filenames. Depends: F02.
- [ ] **U03 — BRE/ERE compatibility layer.** Specify and implement the C-locale regex subset needed by grep/sed/expr/find, including leftmost-longest matching, backreferences where required, classes and malformed patterns. Bound expensive matching; create an explicit unsupported-extension contract. Depends: F07.
- [ ] **U04 — `grep`, `egrep`, `fgrep` baseline.** Stream files/stdin; implement -e/-f, -E/-F, -i/-v/-x/-n/-c/-l/-q and statuses 0/1/2. Cover long lines, empty patterns, binary bytes and output errors. Refs: Toybox grep, MIT uutils/grep. Depends: U03.
- [ ] **U05 — Recursive and contextual grep.** Add -r/-R, include/exclude, -A/-B/-C, -m/-o, byte offsets and NUL-safe record/name output with explicit semantics. Verify quiet/error interactions and no agent result truncation. Depends: U02, U04.
- [ ] **U06 — `find` expression engine.** Implement argv grammar, implicit AND, OR/NOT/grouping, default -print, -name/-path/-type/-size/time/permission tests and -H/-L/-P traversal. Test precedence and short circuit independently of walking. Refs: Toybox find, MIT findutils. Depends: U02, U03.
- [ ] **U07 — `find` actions.** Add -prune/-depth/-xdev, -print0, -exec with `;` and batched `+`, then -execdir/-delete with correct ordering. Test filename injection, batching limits, symlink changes and partial errors in temporary trees. Depends: U06, F06.
- [ ] **U08 — `xargs` serial execution.** Implement quote/backslash tokenization, -0/-d, -n/-s/-I, empty-input policy, argument-size limits and child-status aggregation. No shell interpolation; builtins must resolve internally. Refs: Toybox xargs, MIT findutils. Depends: F06.
- [ ] **U09 — Parallel xargs and find/xargs integration.** Add bounded -P scheduling, cancellation, stop-on-255, signal failures, oversized environment handling and NUL-safe find pipelines. Test maximum concurrency rather than execution order. Depends: U07–U08, F05.
- [ ] **U10 — `sed` parser and simple execution.** Implement script/-e/-f parsing, addresses/ranges, substitution, print/delete/quit and explicit BRE/ERE selection. Test unterminated final lines, stdin and multiple files. Refs: Toybox sed, MIT uutils/sed; red as secondary only. Depends: U03.
- [ ] **U11 — Sed state/control commands.** Add hold/pattern spaces, n/N/d/D, h/H/g/G/x, labels/branches/test, insert/append/change/transliteration and read/write commands. Test multiline cycles and substitution-success state. Depends: U10.
- [ ] **U12 — Sed in-place editing and extensions.** Implement -i backup/replacement policy, failure cleanup, metadata/symlink behavior and selected extensions. Replace files only after successful output; exercise disk/write failures and links. Depends: U11, F08.
- [ ] **U13 — `chgrp`.** Share ownership traversal policy with chown; implement numeric/named groups, reference, recursion and symlink policies. Test unprivileged failures and platform metadata differences. Refs: Toybox chgrp, MIT coreutils. Depends: F07–F08.
- [ ] **U14 — Real `install` utility.** Add -d/-D/-m/-o/-g/-p/-t/-T and multi-source destination semantics, distinct from `misa --install`. Specify strip behavior explicitly; do not silently invoke an external strip tool. Test creation modes, overwrite and failures. Refs: Toybox install, MIT coreutils. Depends: U13, F04, F08.
- [ ] **U15 — `df` and mount-table abstraction.** Implement filesystem space/inode reporting, units, path operands and mount filtering, with per-platform statfs/mount readers. Test fixtures plus live temporary filesystems; account for Android access restrictions. Refs: Toybox df, MIT coreutils. Depends: F02.
- [ ] **U16 — `dd` streaming baseline.** Parse key=value operands, block sizes, count/skip/seek, short reads/writes and records diagnostics. Test ordinary files and pipes without requiring block devices. Refs: Toybox dd, MIT coreutils. Depends: F05.
- [ ] **U17 — DD conversion and status features.** Add selected conv/iflag/oflag, sparse/sync/noerror/fullblock and status/progress behavior with platform capability checks. Prove partial-block, truncation and cancellation semantics before exposing destructive device use. Depends: U16.
- [ ] **U18 — `od`.** Typed byte/word/integer/float output, offsets, endianness, -A/-j/-N/-t, repeated-line suppression and input concatenation. Golden binary fixtures and short final blocks. Refs: Toybox od, MIT coreutils. Depends: F07.
- [ ] **U19 — `hexdump` and `hd`.** Implement canonical and format-string output using shared byte-reading infrastructure; retain existing `xxd` semantics. Test offsets, repeated blocks and malformed formats. Refs: Toybox hexdump/hd, MIT util-linux. Depends: U18.
- [ ] **U20 — `join`.** Sorted field joins, separators, missing fields, unpaired rows and output-field selection. Test duplicates, empty fields, sortedness policy and C-locale comparison. Ref: MIT coreutils. Depends: F07.
- [ ] **U21 — `csplit`.** Line/regex operands, repetitions, prefixes/digit widths, byte-count output and cleanup/-k semantics. Test partial failure and regex boundary behavior. Refs: Toybox csplit, MIT coreutils. Depends: U03.
- [ ] **U22 — `pr`.** Pagination, headers, dates, columns, merge mode, tab expansion, formfeeds and numbering. Use deterministic locale/time fixtures and distinguish byte width from display width. Ref: MIT coreutils. Depends: F07.
- [ ] **U23 — `dir`, `vdir`, `dircolors`.** Register explicit ls output-mode aliases and implement shell-output/print-database color definitions; test non-tty output and LS_COLORS interaction without regressing Misa's interactive styling. Ref: MIT coreutils. Depends: F03.
- [ ] **U24 — `stty`, `clear`, `reset`.** Add tty-state query/set/save/restore and standalone terminal helpers, separate from UI clear. Begin with portable termios baseline; test via PTYs and handle non-tty stdin. Refs: Toybox stty/clear/reset, MIT coreutils stty. Depends: S09, S17.
- [ ] **U25 — `more` pager.** Streaming/non-tty passthrough, interactive paging/search/resize, quit and terminal restoration. Reuse Misa display primitives where appropriate without importing UI into libraries. Refs: Toybox more, MIT coreutils more. Depends: U24, U03.
- [ ] **U26 — Process snapshot library and `ps`.** Extract reusable procmon/sysinfo data with Linux/BSD/macOS/Android backends; add stable noninteractive columns and selection. Test recorded proc data, disappearing processes and permission failures. Refs: Toybox ps, MIT procps. Depends: F02.
- [ ] **U27 — `pidof`, `pwdx`, `killall`, `pidwait`.** Build on snapshots with careful name/path matching, PID reuse behavior, signal/status conventions and cancellation. Test only owned child fixtures. Refs: Toybox and MIT procps where available. Depends: U26, F05.
- [ ] **U28 — `free`, `uptime`, `vmstat`.** Add snapshot/delta reporting, units, interval/count, unavailable counters and boot-time handling; keep output independent of the interactive top UI. Refs: Toybox, MIT procps/coreutils. Depends: U26.
- [ ] **U29 — `renice`, `nohup`, `setsid`, `setpgid`.** Implement child/process-target wrappers through the execution service, proper tty redirection/HUP behavior and status forwarding. Test isolation and process-group lifetimes. Split by platform if needed. Refs: Toybox, MIT coreutils/bsdutils/util-linux. Depends: F04, F06, S17.
- [ ] **U30 — Standalone `time` and `watch`.** Separate timing one command from timing a shell pipeline; add watch interval/difference/exit behavior, direct-exec versus internal-shell mode, cancellation and PTY restoration. Refs: Toybox time/watch, MIT procps watch. Depends: S18, U24.
- [ ] **U31 — `gzip`, `gunzip`, `zcat`.** Internal streaming codecs, concatenated members, metadata, -c/-d/-f/-k/test behavior and atomic file replacement policy. Test damaged streams, overwrite prompts and no external gzip. Ref: Toybox. Depends: F05, F08.
- [ ] **U32 — `tar` create/list/read baseline.** Implement regular files, directories, modes, symlinks/hardlinks, stdin/stdout, exclude/files-from and ustar/PAX handling using internal archive APIs. Test cross-entry links and long names. Refs: Toybox tar, MIT uutils/tar. Depends: U02.
- [ ] **U33 — Tar extraction and compression integration.** Implement extraction containment, absolute/parent paths, link traversal, metadata policy, gzip and selected overwrite/strip behavior. Test malicious path/link archives, partial writes and cancellation. Depends: U31–U32.
- [ ] **U34 — `cpio`.** Newc list/extract/create baseline, hardlinks, padding and pass-through modes; apply the same explicit extraction path policy. Test binary fixtures and truncated archives. Ref: Toybox cpio. Depends: U32–U33.
- [ ] **U35 — `file` and `readelf`.** Start with documented common signatures and ELF metadata via existing standard APIs; print unknown honestly rather than claiming a full external magic database. Test endianness, truncated inputs and limits. Ref: Toybox. Depends: F07.
- [ ] **U36 — `patch`.** Unified/context diff application, -p/-R/dry-run, offsets/fuzz policy, rejects, additions/deletions and output failure handling. Test against Misa-generated diffs and native malformed/path fixtures. Ref: Toybox patch only for external implementation/tests. Depends: F08.
- [ ] **U37 — `dos2unix`, `unix2dos`, `ts`, `usleep`, `count`, `crc32`.** Small streaming helpers with explicit timestamp format/timezone, newline/BOM and checksum contracts; bounded cancellation. Group only the shared streaming infrastructure, split if review grows. Refs: Toybox. Depends: F03, F05.
- [ ] **U38 — `getopt` and `getconf`.** Keep external getopt distinct from shell getopts; implement quoting/long-option mode and a documented per-platform getconf variable set. Test re-evaluation through the internal shell and unknown variables. Ref: Toybox. Depends: S08, S13.
- [ ] **U39 — `flock`, `fsync`, `fallocate`.** Add fd/path lock modes and command wrapping, flush modes and allocation options with per-platform behavior. Test contention between owned processes, unlock-on-exit and unsupported flags. Ref: Toybox. Depends: F06, S09.
- [ ] **U40 — `who`, `users`, `pinky`, `w`, `last`, `mesg`.** Session/accounting abstraction with platform-specific readers and fixture tests; empty/unavailable accounting is not a fabricated user list. Test terminal permissions in owned PTYs. Refs: Toybox and MIT coreutils/procps/util-linux. Depends: U24, U26.
- [ ] **U41 — AWK language core.** Parse/evaluate patterns, BEGIN/END, fields/records, variables, numeric/string coercion and print/printf. Use only Toybox AWK implementation/test references; add native grammar/coercion tests. Depends: U03, F07.
- [ ] **U42 — AWK language completion.** Arrays, functions, loops, getline, file redirection, pipes and system calls through the internal executor; test file-handle reuse/close, errors and cancellation. Separately record unsupported extensions. Depends: U41, F06, S12.
- [ ] **U43 — `bc`.** Decimal precision/scales, language parser, expressions, variables/functions and documented math-library behavior; do not substitute floating point for decimal semantics. Toybox pending implementation is the reference, not a maturity guarantee. Depends: F07; split math library if necessary.
- [ ] **U44 — Text compatibility consolidation.** Fix shared BRE/ERE drift in existing expr and new tools; audit UTF-8 boundaries, NUL records, huge records, locale assumptions and reproducible benchmark corpora. Publish the actual supported option matrix. Depends: U03–U12, U20–U22, U41–U42.

### Optional/basic-system expansion: O01–O30

These are scheduled after the first self-contained daily-use milestone. They remain concrete PR candidates, but several have explicit feasibility gates rather than a promise to support every platform.

- [ ] **O01 — `mknod`, `chroot`, namespace/IPC tools.** Implement child-only `mknod`, `chroot`, `unshare`, `nsenter`, and `linux32`, plus `ipcs`/`ipcrm`, in separately reviewable slices if necessary. Test namespace/root operations in disposable environments; ordinary tests cover argument and error behavior. Refs: Toybox, MIT coreutils. Depends: F04, F06.
- [ ] **O02 — File metadata extensions.** `getfattr`/`setfattr`, `lsattr`/`chattr`, `getfacl`/`setfacl`/`chacl`; platform-tagged APIs and round-trip tests for supported filesystems. Preserve unsupported-feature errors. Refs: Toybox and MIT acl. Depends: F08, U13.
- [ ] **O03 — Mount/storage inspection.** `mountpoint`, `blkid`, `fstype`, `lslocks`, `lsns`, `lscpu`, `lsmem`, `lsipc`. Use fixture-backed readers and stable parsers before mutators. Refs: Toybox and MIT util-linux. Depends: U15, U26.
- [ ] **O04 — Process/resource extensions.** `pmap`, `lsof`, `iotop`, `hugetop`, `slabtop`, `tload`, `skill`, `snice`, `taskset`, `ionice`/`iorenice`, `chrt`, `prlimit`, `uclampset`, `chcpu`. Split snapshot tools from mutation tools; test parent isolation and Linux/Android permission errors. Refs: Toybox, MIT procps/util-linux. Depends: U26–U29.
- [ ] **O05 — Kernel diagnostics and logging.** `dmesg`, `sysctl`, `logger`; read-only operations first, explicit writes next, internal syslog transport and unavailable-kernel behavior. Refs: Toybox and MIT procps/util-linux. Depends: U26.
- [ ] **O06 — Network clients.** `wget`, `host`, `dnsdomainname`, `nc`/`netcat`; internal HTTP/DNS/socket APIs, TLS verification, redirects, timeouts, stdin streaming and local loopback tests. No dependency on a host curl/wget. Ref: Toybox. Depends: F05.
- [ ] **O07 — Network inspection and routing.** `ip` plus `ipaddr/iplink/iproute/iprule/iptunnel`, `ifconfig`, `route`, `arp`, `netstat`, `rfkill`, `brctl`, `tunctl`, `vconfig`; read-only substrate then namespace-contained mutations. Toybox pending status for several commands raises effort. Depends: O06, O01.
- [ ] **O08 — Network probes and legacy clients.** `ping/ping6`, `traceroute/traceroute6`, `arping`, `sntp`, `ftpget/ftpput`, `tftp`, `telnet`, `microcom`; unprivileged/privileged capabilities explicit and loopback-only CI. Split protocol families as needed. Ref: Toybox. Depends: O06.
- [ ] **O09 — Account inspection and refusal tools.** `nologin`, `chage`, `expiry`, `pwck`, `grpck`; operate on explicit fixture roots first and validate malformed account files without modifying real users. Refs: Toybox, MIT shadow/util-linux. Depends: F07.
- [ ] **O10 — Transactional account/group editors.** `useradd/adduser`, `userdel/deluser`, `usermod`, `groupadd/addgroup`, `groupdel/delgroup`, `groupmod`, `newusers`, `pwconv/pwunconv`, `grpconv/grpunconv`, `vipw/vigr`. Locking, atomicity, rollback and fixture-root tests precede host support. Refs: Toybox and MIT shadow. Depends: O09, U13–U14.
- [ ] **O11 — Authentication and credential launchers.** `passwd`, `chpasswd`, `chgpasswd`, `gpasswd`, `chfn`, `chsh`, `su`, `sg`, `newgrp`, `login`, `sulogin`, `getty`, `newuidmap/newgidmap`. First decide libc/PAM-free authentication scope; no false claim of host-auth parity under CGO=0. Child-only credentials, locked fixture databases, PTY tests. Refs: Toybox, MIT shadow/login. Depends: O09–O10, F04.
- [ ] **O12 — Mount/swap/device operations.** `mount/nfsmount`, `umount`, `losetup`, `blockdev`, `blkdiscard`, `fsfreeze`, `mkswap`, `swapon/swapoff`, `partprobe`, `eject`, `readahead`, `freeramdisk`, `makedevs`. Implement independent slices against disposable disk images/VMs; support helpers only if internal or explicitly documented external. Refs: Toybox, MIT util-linux. Depends: O01, O03.
- [ ] **O13 — Filesystem repair/partitioning feasibility.** `fdisk`, `fsck`: audit what Toybox actually implements versus dispatches to external helpers. Implement only a bounded, tested format; a dispatcher is not an internal filesystem checker. Toybox pending references. Depends: O12.
- [ ] **O14 — Kernel module utilities.** `lsmod`, `modinfo`, `insmod`, `rmmod`, `modprobe`; inspect formats/compression and dependency resolution before adding mutation. Fixture metadata tests; VM-only load/unload. Ref: Toybox. Depends: U35, O12.
- [ ] **O15 — Hardware information/control.** `acpi`, `lspci`, `lsusb`, `hwclock`, `rtcwake`, GPIO and I2C families, `devmem`, `ucsicontrol`, `sendevent`. Split inspection from driver ioctls; fake backends plus opt-in hardware validation. Ref: Toybox. Depends: O03.
- [ ] **O16 — SELinux/Android system tools.** `chcon`, `runcon`, `getenforce`, `setenforce`, `restorecon`, `load_policy`, `log`; determine supported kernel/policy operations and Android app restrictions without assuming libselinux is available. Refs: Toybox, MIT coreutils where present. Depends: O02, F04.
- [ ] **O17 — Extra decompression.** `bunzip2`, `bzcat`, `xzcat`: bzip2 decode via existing standard API; evaluate a bounded in-tree Toybox-derived XZ decoder versus a separately approved dependency. Test malformed data and memory limits; no external xz fallback. Ref: Toybox. Depends: U31.
- [ ] **O18 — `iconv` and character helpers.** Streaming encoding conversion and `ascii`/`unicode`, with explicit supported encodings, invalid-sequence policy and split-codepoint tests. Toybox is the utility reference; existing text APIs may supply mechanisms. Depends: U44.
- [ ] **O19 — `locate`/`updatedb`.** Versioned database format, configurable roots/exclusions, incremental policy and correct access checks at lookup time. Test filenames with NUL-impossible/newline-valid boundaries and atomic database replacement. Ref: MIT findutils. Depends: U02, U06.
- [ ] **O20 — `rename` and `shred`.** Specify util-linux-style rename semantics rather than a Perl-language variant; shred must accurately describe COW/SSD limitations and never imply guaranteed erasure. Test overwrite, links, failures and temporary files. Refs: MIT util-linux/coreutils, Toybox shred. Depends: F08.
- [ ] **O21 — `ptx` and `stdbuf` feasibility.** Implement ptx with documented C-locale rules. For stdbuf, assess whether the required dynamic-loader/libc mechanism can coexist with the project's no-cgo/single-binary goals; changing Go writer buffering does not control arbitrary external libc programs. Publish a decision before implementing or explicitly deferring stdbuf. Ref: MIT coreutils. Depends: F06, U44.
- [ ] **O22 — `pwgen`, `mkpasswd`.** Cryptographic randomness and explicitly supported password-hash formats, deterministic algorithm vectors, tty/secret-output handling and clear compatibility scope. Ref: Toybox. Depends: F07, O11 for shared authentication primitives if adopted.
- [ ] **O23 — Shell assistance and terminal extras.** Standalone `help`, `hexedit`, `chvt`, `deallocvt`, `openvt`, and `mix` only where platform facilities exist. Help derives from the actual registry/options; terminal changes restore state on failure. Ref: Toybox. Depends: U24–U25, S14.
- [ ] **O24 — Event monitoring/debugging.** `inotifyd`, `strace`, `memeater`; executable hooks use the internal executor, tracing stays a separately bounded architecture-specific feature. Use child fixtures and resource limits. Ref: Toybox. Depends: F06, O04.
- [ ] **O25 — Scheduling and log daemons.** `crond`, `crontab`, `syslogd`, `klogd`, `bootchartd`; decide integration with existing ughbot scheduling instead of creating duplicate ownership. Validate time/DST/restart behavior and use the internal shell for scheduled commands. Ref: Toybox. Depends: S22, O05.
- [ ] **O26 — Network services.** `httpd`, `tcpsvd`, `udpsvd`, `telnetd`, `tftpd`, `dhcp`, `dhcp6`, `dhcpd`, `dumpleases`, NBD client/server alias families. Split by protocol; local sockets/network namespaces and bounded resources, no daemon started by default. Ref: Toybox. Depends: O06–O08, O01.
- [ ] **O27 — Init/recovery/power tools.** `init`, `oneit`, `mdev`, `pivot_root`, `switch_root`, `halt`, `poweroff`, `reboot`, `killall5`, `ctrlaltdel`, `watchdog`. VM-only end-to-end tests; distinguish a recovery-toolbox goal from replacing the host init system. Refs: Toybox, MIT util-linux ctrlaltdel. Depends: O01, O12, O25.
- [ ] **O28 — Toybox Git subset feasibility.** `gitclone`, `gitfetch`, `gitcheckout`, `gitinit`, `gitremote`: inspect only Toybox's subset and its format/transport limits. Decide whether that subset serves any Misa use case; do not present it as replacing external Git for ugh worktrees. Depends: O06, U35.
- [ ] **O29 — Locale and advanced text compatibility.** Add selected locale collation/character/numeric/date behavior after a dependency and size review; lock down fallback and environment precedence. Keep C-locale results stable. Depends: U44, O18.
- [ ] **O30 — Cross-platform/size/performance completion.** Consolidate per-platform command support tables, unsupported operations, Android emulator cases, binary-size budgets and streaming benchmarks; split optional feature sets if measured cost warrants it. Depends: implemented optional groups, not all speculative groups.

## Milestones and ordering

**M0 — Trustworthy foundation:** F01–F07 and S01. Fix environment/process side effects before enabling parallel in-process utility execution. Importing shell code and fixing existing utilities can proceed independently after the shared invocation contract is agreed.

**M1 — Internal script engine:** S02–S04, S07–S14, plus U01 and F05. Demonstrate quoting, expansion, lists, pipes, here-docs, functions, sourced scripts, cancellation and explicit command resolution with no host shell. Process-state operations and signal/job semantics still await S15–S18; this is useful before a full interactive worker exists, but is not the final default-switch gate or a claim of complete POSIX behavior.

**M2 — Interactive self-contained shell:** S05–S06 and S17–S22. All owned shell-string execution paths use the internal engine; pane state and job control survive real PTY scenarios. The engine may still run external applications deliberately. This milestone is blocked on signals/process semantics, not on AWK or every utility.

**M3 — Everyday script toolbox:** prioritize U01–U17, U24, U26–U33, U35–U36. Ship missing commands incrementally; no mega-PR containing grep+sed+find. `find -print0 | xargs -0`, safe archive extraction, text filtering and installation workflows should work in a Misa-only utility environment.

**M4 — Broad core/basic parity:** remaining U tasks, then small optional high-value groups such as O03–O06, O17–O20. AWK/BC and regex compatibility are substantial projects and can proceed separately once their foundations exist.

**M5 — Optional system toolbox:** remaining O tasks selected by actual use. Root/network/device families require their own implementation slices and platform test environments. A declared upstream applet is an inventory candidate, not a product requirement to ship immediately.

Critical path: **F02 → F03/F04/F06 → S02/S03 → S05/S06 → S09/S10/S16 → S17/S18 → S19/S20/S21 → S22**. Utility work joins this path only where the hermetic acceptance workload needs it. Shared regex work U03 blocks grep/sed/find regex/awk correctness; shared walker U02 blocks traversal families. Archive format implementation precedes mutating extraction features.

## Acceptance and testing strategy

### Workloads that should determine priority

| Workload | Required capabilities | Milestone evidence |
|---|---|---|
| Daily prompt use | persistent cd/exports/functions, quoting, redirects, external program launch, Ctrl-C/Ctrl-Z, jobs | M2 real-PTY tests across two panes |
| Source-tree search/refactoring | grep/find/xargs, NUL-safe filenames, sed, patch, current diff | M3 native temporary-repository fixtures, including spaces/newlines/leading dashes |
| Download/unpack/install | HTTP client, tar/gzip, install/chgrp, test, mkdir/cp, failure cleanup | M3 plus O06; loopback download server and isolated destination tree |
| Backup and restore | find, tar, gzip, metadata/link handling, df, cancellation | M3/M4 round-trip and failure tests, explicitly supported metadata only |
| Agent code task | same parser dialect as policy, internal shell tool, bounded stdout/stderr, group cancellation, intentional external Git/build tools | S20 and existing agent regression suites |
| Minimal Android/Termux session | no reliance on a particular `/system/bin/sh`, internal utility lookup, permitted procfs behavior, re-exec and PTY lifecycle | S21/S22 emulator tests plus portable utility cases |
| Data/text scripts | join/csplit/pr, regex compatibility, awk/bc, predictable C-locale output | M4 fixtures that exercise coercion, empty records and long inputs |

Common workflow names such as `curl`, `jq`, `zip`/`unzip`, `less`, `ed` and `dc` are **not supplied as utility implementations by this enumerated approved-source set**. They are not silently counted as covered by wget, more, vi, bc or tar. If these become priorities, choose a native implementation using public APIs or explicitly approve an additional reference; this research does not consult other implementations. Similarly, no make/compiler/package-manager replacement is implied by the internal-shell milestone.

### Meaningful shell checks

- Separate commands in one pane retain `x=...`, functions, exported values, aliases and cwd; a second pane retains its own values.
- Quoted empty arguments, filenames with spaces/newlines/leading dashes, `$@`, IFS and parameter errors preserve exact argv.
- Large `cat | grep | sort`, `yes | head`, nested substitutions, here-docs and descriptor duplication make progress and close every owned fd.
- `false && ...`, `true || ...`, conditions under `set -e`, `trap 0`, INT/TERM traps, failed exec and killed children produce the specified status and side effects.
- Stopping/resuming a pipeline, background tty reads, two simultaneous jobs, wait after exit, worker failure and closing a pane exercise real PTY/process-group behavior.
- File script, stdin script, shebang and ENOEXEC execution retain the same resolver; a no-shebang child must not silently regain a default upstream execution handler.
- `env -i`, explicit `/path/to/tool`, `command -v`, `type`, `hash`, nested wrappers and builtin/PATH collisions have consistent documented resolution.
- Agent tool cancellation still removes the owned group, output remains bounded, and background tracking does not lose ownership. Static analysis sees the same syntax that executes.

### Utility and platform checks

- Test output **bytes, exit status, stderr and filesystem/process side effects**, not only a string snapshot. Exercise stdin and `-`, output errors, huge records, short reads, broken pipes, cancellation and multiple operands.
- Imported Toybox/uutils tests are candidates, not an oracle to import indiscriminately. Inspect each file's provenance and avoid separately licensed upstream suites or fixtures even when vendored inside an allowed repo. Use native Misa tests for uncovered logic.
- Existing adapter parses are incomplete. A skipped or statically unparseable Rust scenario is not a passing test. Report counts with denominators and reasons; remove known-failure entries only when the real case runs and passes.
- Run portable cases on Linux, OpenBSD, FreeBSD, macOS and Android as relevant. Use fixture-backed readers for `/proc`, accounting, mount tables and sysfs; live integration tests verify the backend separately.
- Privileged mutation tests use disposable namespaces/VMs/disk images. Normal test runs require no user configuration, global Git config, root access or modifications to real account/network/storage state.
- Empty PATH alone does **not** establish independence from `/bin/sh` or absolute external tools. Use a dedicated minimal test root/image with only Misa and necessary runtime resources; on Android, use controlled launch instrumentation because the system image supplies unavoidable tools.
- Maintain `nough`, `nogui`, `noweb`, Android cgo and non-Android CGO=0 constraints. Cross-vet is type checking, not evidence that the commands behave correctly on that OS; emulator/host runs remain necessary.

### Definition of done for an implementation PR

1. Scope and supported options/platforms are explicit; every unsupported option has an honest result.
2. Native behavior tests precede the logic, with permitted upstream cases where their provenance is established.
3. Cancellation, environment/cwd isolation, nested invocation and stream ownership are tested if applicable.
4. Source attribution and license/NOTICE/CREDITS entries accompany derived code. In-tree mvdan edits remain traceable to the import baseline.
5. Registry/help/completion/docs and the support matrix agree. No hard-coded claim of full GNU/Bash/POSIX parity.
6. Focused tests pass; relevant repository build/tag/vet/platform checks run. No broad test reruns are claimed by this research-only change.
7. The PR references its tracking ID and updates the checklist with the actual PR link. Failed/conditional feasibility work records its decision rather than ticking a nonexistent implementation complete.

## Remaining design decisions, with recommended starting positions

These do not block publishing the backlog. They belong in the named implementation tasks.

- **Import revision:** compare pinned v3.13.1 with the researched newer revision under existing permission-parser tests and the native shell suite; choose one exact baseline in S01.
- **Worker versus in-process session:** persistent worker is the recommended interactive design; validate protocol/state/PTY costs in S05. Do not serialize only exported variables back from one-shot workers and call that persistent shell state.
- **Native escape:** choose an unambiguous explicit form so POSIX `!` can retain its meaning; preserve a migration path in S06.
- **Startup files:** propose explicit Misa-owned configuration and opt-in script sourcing; do not execute a project-directory rc file implicitly. Define `ENV`, login and interactive semantics before advertising general sh replacement.
- **Utility shadowing:** preserve user control over external command choice and make introspection truthful; decide default Misa-utility precedence once in S04 rather than in each wrapper.
- **Archive/file replacement defaults:** document link/path containment, metadata and overwrite policy per format; test rather than infer from another utility's implementation.
- **Optional tools with dependency conflicts:** `stdbuf`, PAM-style login, filesystem repair, XZ, device databases and broad locales require explicit scope decisions. Keep them visible instead of promising a host-tool wrapper as an internal implementation.
- **Breadth versus binary size:** measure first. No Rust or C build tool becomes a runtime/build prerequisite merely because its code is an approved reference; utility implementations remain Go unless a separate platform decision is made.

## Research verification limits

The registry build and isolated v3.13.1 interpreter probes were run. Upstream repositories were enumerated, pinned and inspected; selected architecture, utility option declarations and candidate test locations were read. **The 278 missing commands were not implemented or behaviorally benchmarked, and no whole-project conformance percentage is asserted.** Optional families are deliberately grouped feasibility/implementation candidates; especially languages, network stacks, account tools and device families are likely to split into additional PRs. The roadmap is larger than 40 PRs because the state/process/compatibility work is real, not because every alias needs its own PR.
