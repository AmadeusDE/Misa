# Upstream-test accounting (F07)

The Toybox and MIT-uutils adapters describe **adapter coverage**, not whole-command
compatibility. Unit tests, native tests and the separately ported diffutils tests
are not included in these upstream counts. The source registry remains
`internal/inventory/sources.json` (F01); accounting does not introduce another
registry of approved upstreams.

## Generate and read a report

From the repository root:

```sh
env -u OPENROUTER_API_KEY go run ./tests/report -out /tmp/misa-upstream
# /tmp/misa-upstream.json and /tmp/misa-upstream.md
```

The command runs both adapters without the Go test cache, reads their structured
records, and writes JSON and Markdown. It returns a failure if either suite fails
or its records are missing/invalid, saving available partial results first.
For individual suites, set `MISA_TEST_REPORT` to an **absolute directory**:

```sh
MISA_TEST_REPORT=/tmp/misa-accounting go test -count=1 ./tests/toybox ./tests/uutils
```

Each suite writes its own JSON file, so parallel package execution cannot overwrite
another suite. Reporting is opt-in; the command does not source upstream harnesses
or fixture-generation scripts. The adapters still replay their supported, guarded
setup actions. Reports are generated on demand rather than checked
in: executed results and host-dependent skips can vary by platform and filesystem.
The old `docs/TEST_STATUS.md` is historical, not a current coverage measurement.

Every case record is discovered. Toybox records statically detected `testcmd` /
`testing` constructs (including unsupported templates), plus its representable
cases; they are not a verified count of distinct runnable upstream invocations.
Some multiline constructs currently retain their final-line location. `parsed`
means the adapter represented the case; `executed` means it actually invoked the
command under test, not that it merely entered a Go subtest. Each record ends as
passed, failed or skipped. A skip is not a pass. JSON retains source location and
the individual records; Markdown groups counts by suite and utility and lists
skip explanations. A setup failure can be failed without being executed; it carries
a failure diagnostic rather than masquerading as a skip. These denominators are
adapter-specific, not an estimate of all possible upstream test executions.
Uutils records representable assertion
chains plus one gap record for each recognized `test_*` function with no represented
assertion. A partially represented function can still contain unsupported chains
that are not individually discovered. Static-chain-site accounting within those
functions is an explicit **F07 follow-up**, not a claim of full Rust coverage.

The historical before/after counts and **every newly skipped case with its reason**
are listed in [the F07 execution-change summary](upstream-test-execution-delta.md).

Skip categories are:

| Category | Meaning |
| --- | --- |
| `unparseable` | Unsupported syntax/setup/assertion; not silently discarded. |
| `known_failure` | An existing exclusion with a concrete explanation. |
| `deferred` | Explicitly deferred behavior, with a roadmap task ID. |
| `host_dependent` | Requires a particular platform, filesystem, locale, privilege or fixture. |
| `unsafe_setup` | Unsafe imported setup or command refused before execution. |

Native equivalents supplement, but do not turn skipped upstream cases into passes.
`tests/native/upstream_constructs_test.go::TestAdapterNativeConstructs` covers a
multiline chmod-loop equivalent, pipeline-equivalent streaming, and a relative
symlink fixture chain using Go-managed temporary files and explicit invocations.
The applicable skip records link to these native cases.

## Safety and exclusions

Adapters are deliberately limited translators, **not security sandboxes**. The
Toybox shell guard is a **best-effort denylist over the pinned, reviewed fixture
set**: it refuses positively detected host-touching constructs (escaping literal
write paths, privilege/network/device operations and dangerous shell evaluation).
It does not prove arbitrary shell programs safe. Unsupported or dynamic syntax is
not itself evidence of a host write; existing harmless pipelines, redirections and
setup continue to run. Shell startup hooks and fixture-controlled host paths are
not inherited blindly. Do not use this harness to execute unreviewed imports.

The uutils mutator check confines path operands to the private test tree, except
that `cp`, `tee` and `dd` may name the standard stream devices (`/dev/null`,
`/dev/stdin`, `/dev/fd/0`–`2`, …) and paths beneath `/dev/null/`, which always
fail with ENOTDIR. Removing, re-moding or splitting into those paths stays refused.

The uutils adapter replays only recognized Go fixture actions and refuses setup or
mutating operands that it cannot establish as confined to the temporary fixture
root. Unknown Rust fixture setup is recorded rather than silently approximated.
Neither adapter should be expanded merely to improve a pass percentage. The
safety guard is intentionally separate from utility compatibility accounting.

Both `known_failures.txt` files require a reason on every entry. Entries must still
identify a discovered case; representable, safe exclusions are checked for stale
passes. Normal runs retain those exclusions. `MISA_KNOWN_FAILURES=1` is a diagnostic
opt-in to run known failures, not an option to bypass safety validation. F08 owns
builtin correctness fixes; F07 changes only test infrastructure. Existing F08 debt
remains visible in the exclusions (including `ln -sf` self-source handling,
copying to a symlinked destination, move hard-link preservation, and empty-name
`env` assignments). F07 does not reinterpret adapter/locale/filesystem differences
as proven builtin defects. Known host-dependent/deferred entries are not forced
through stale-pass probes; those probes report unchecked skips explicitly.

## Fixture provenance

`tests/provenance/fixtures.json` inventories the local fixture bytes with SHA-256,
source-registry IDs and reported upstream paths. That includes the copied upstream
fixtures under `tests/toybox/files/`, which the toybox adapter exposes to suites as
`$FILES`; every file there must be reviewed, pinned and inventoried. Its tests check completeness,
hashes and safe file types without network access. Approved source URLs, pinned
revisions and licenses resolve through the F01 registry; the per-file verification
status must be read alongside them: `matches-pin` means byte-identical;
`modified-from-pin` means different bytes at the reported path, **not** proof of
derivation from that commit; `unknown-origin` means no comparison was established.
Where a pinned upstream path is itself a symlink, the manifest records its target.
A local hash alone is **not** proof that a file came from the pinned revision.
Existing diffutils attribution is recorded for transparency only: F01 marks that
upstream excluded with an unresolved license
metadata discrepancy. No new diffutils material is imported by F07.
