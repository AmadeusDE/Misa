# S01 — mvdan/sh foundation import

S01 imports a shared parser/runtime foundation; it does **not** select a new
execution engine. Misa's TUI dispatch, external-shell fallback, PTY handling,
registry and existing builtin behavior are unchanged.

## Baseline decision

Selected **mvdan/sh v3.13.1**, exact tag commit
`2f3f5e36d9b0f8f14c998d50aa20a28832205ae8` (BSD-3-Clause).
The comparison candidate was the inventory's researched revision
`e96ef1dae52c71e86cf87eb594cb4917fa49aaba`.

Before importing, both revisions were tested using the existing Misa consumers
and the same small, native-authored shell probe suite:

| Regression set | v3.13.1 | e96ef1da |
| --- | --- | --- |
| `go test -count=1 ./internal/ugh/perm/... ./internal/ugh/agent/...` | Pass | Pass |
| POSIX AST print/reparse | Pass | Pass |
| Quoting, parameter expansion and empty arguments | Pass | Pass |
| Short-circuit lists and builtin-only pipeline | Pass | Pass |
| Functions, expanding heredoc and subshell variable isolation | Pass | Pass |
| Pre-canceled interpreter and rejecting external-exec handler | Pass | Pass |

The release was read from the Go module cache; the candidate was cloned from
`https://github.com/mvdan/sh` and checked out at the exact researched commit.
Comparison runs used a scratch Misa copy and a separate probe module, with a
**temporary scratch-only** `replace mvdan.cc/sh/v3 => ...` for the candidate.
No replacement directive or comparison dependency update is retained here.
The retained probes are in `internal/sh/interp/foundation_regression_test.go`,
with additional sequential-run persistence and separate-runner isolation tests.
They use an explicit temporary cwd, synthetic environment, in-memory streams
and an exec handler that refuses external execution, not host shell utilities.

Neither revision failed these targeted regressions. The newer candidate has a
substantially larger change surface, including additional job machinery, and its
scratch module resolution selected newer `x/sys` and `x/term` versions. With no
observed S01 benefit, the release minimizes changes to today's permission parser
and needs no dependency version bumps. This is not evidence of full shell,
job-control or signal compatibility. In particular, the researched ENOEXEC /
custom-resolver concern was **not** exercised by these probes. Later S-task
work can import selected newer fixes or job machinery as reviewed, recorded
local patches rather than silently changing the baseline.

## Imported boundary

`go list -deps` was evaluated for roots `syntax`, `expand`, `interp`, `pattern`
and `fileutil`, retaining mvdan packages only. The union of production Go files
was taken across `CGO_ENABLED=0` Linux, Android, Darwin, FreeBSD, OpenBSD and
Windows (Android arm64; other targets amd64).

The closure is six packages under `internal/sh`:

- `syntax` — parser, AST, printer and supporting syntax operations;
- `expand` — shell expansion;
- `interp` — upstream runner, imported but not connected to TUI dispatch;
- `pattern` — shell pattern operations;
- `fileutil` — file helpers required by the runner;
- `internal` — upstream helper functions used by that graph.

This is **34 production Go files / 16,488 lines**, counted with `wc -l`, including
comments and blank lines, excluding native tests, license and provenance files.
The BSD copyright headers are preserved wherever present upstream. Files without
an upstream copyright header remain unchanged apart from the recorded patches;
`NOTICE`, `LICENSE` and the per-file source map cover their attribution.

The import omits upstream CLIs, packages outside this closure, upstream tests,
testdata/downloaded fixtures, platform-excluded production files and generated
third-party material. It also omits `internal/testing.go`: despite its production
filename, it only supplies unused upstream integration-test setup (host `locale`,
process-environment changes and shell fixtures), not a runtime helper. No upstream
`_test.go` files are copied: many interpreter
cases use external shells/utilities, and importing their supporting suite would
expand the test dependency and hermeticity boundary. Existing Misa policy
fixtures remain unchanged; focused native tests cover the import seam instead.

Two build-required **first-party** stringer outputs are retained verbatim and
marked `generated: true`: `syntax/token_string.go` and
`expand/valuekind_string.go`. These are mvdan's own BSD-licensed enum tables,
not downloaded/generated third-party data. Upstream `go:generate` comments are
retained as provenance, not invoked; no generator is needed to build or test.

Misa-specific orchestration belongs in a later `internal/shell` layer, not in
this imported graph. `internal/sh` cannot import other Misa packages. Its
interpreter is not a sandbox and is not linked into permission classification;
policy consumers import only `internal/sh/syntax` and its required dependencies.
The existing static-analysis no-`os/exec`/no-`syscall` graph test now traverses the
in-tree parser rather than stopping at an external module boundary.

## Provenance and local patches

- `internal/sh/LICENSE` contains the upstream BSD-3-Clause text.
- `internal/sh/NOTICE`, `docs/CREDITS.md` and `THIRD_PARTY_NOTICES.md` identify the
  specific in-tree import.
- `internal/sh/SOURCE.json` records the exact revision, package list, omissions,
  original-to-local paths, upstream and post-patch SHA256 hashes, and patches.
- The F01 `internal/inventory/sources.json` retains its researched `revision` and
  records the selected `import_revision` and `source_map` separately.

Local source patches are limited to:

1. Replacing `mvdan.cc/sh/v3/` with
   `github.com/AmadeusDE/Misa/internal/sh/` (also in relevant documentation
   references inside imported source).
2. Canonical gofmt import reordering in `interp/os_unix.go`, required because
   the rewritten import prefix changes lexical order.

`internal/inventory` tests validate the source-map connection, imported file
coverage, hashes and the reversible patch transformations. They also test
rejection of a modified file even when its local hash is updated, and rejection
of an unlisted production Go file. Cross-build-tag import checks reject external
mvdan consumers, cgo and dependencies from `internal/sh` into other Misa code.
Do not merely update a local hash after editing imported code: any future
semantic patch needs an explicit reviewed patch record and corresponding
verification, or a deliberate baseline update. S24 owns broader maintenance,
fuzzing and performance work.

## Migration and follow-ups

All current Go consumers were syntax-only: four files in
`internal/ugh/perm/static` and `internal/ugh/agent/critical.go`. They now use the
same in-tree parser; no vi/UI execution path was migrated. No policy fixture or
classification rule changed. The external `mvdan.cc/sh/v3` requirement is
deliberately kept in `go.mod` for now: several open stacked PRs still import it in
tests, so dropping it here would break their builds on merge. The dependency
boundary test rejects new imports of it, and `go mod tidy` will drop it once
those branches are rebased onto `internal/sh` (S24). The
production closure uses only the standard library and already-present modules;
no new module dependency was added.

S02–S04 still own session APIs, explicit shell entry points and registry-aware
resolution. S05–S22 own persistent workers, UI/caller migration, process/job
semantics, acceptance and the default switch. S23 owns selected Bash extensions.
S24 still owns removing the external module, update tooling,
fuzzing, benchmarks and runtime maintenance remain open. Nothing here claims
complete POSIX/Bash compatibility or changes the default shell engine.

## S01 verification

Passed at implementation time:

- `gofmt -l .` (no output) and `git diff --check`;
- `go vet ./...`;
- OpenBSD, FreeBSD and Darwin `go vet ./internal/builtins/`;
- `go build -tags nough ./...` and `go build -tags 'nogui noweb' ./...`;
- `CGO_ENABLED=0` build and vet of `./internal/sh/...` for Linux, Android arm64,
  Darwin, FreeBSD, OpenBSD and Windows;
- API-key-unset tests for builtins, imported foundation, inventory, permission
  analysis, agent packages, dependency direction and the full repository;
- a normal binary build and multicall smoke checks for `echo`, `printf`, `wc`,
  `true` and `false`, including false's status 1.

All recorded upstream hashes were independently checked against both the cached
v3.13.1 module and the exact Git commit. Full-repository tests needed no flaky-test
reruns. Windows builtin vet was already broken at baseline in unrelated Unix-only
`lscore`, `sshclient`, `procmon`, `sysinfo` and `term` code; it was not treated as an
S01 regression. The imported foundation itself builds and vets on Windows.
