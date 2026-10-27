# Contributing

Pull requests are welcome, from people and from coding agents. Agents: read `AGENTS.md` first; it has the
architecture, the project rules and the commands below.

## Before you open a pull request

```sh
gofmt -l .                       # must print nothing
go vet ./...
go vet -tags nough ./... && go vet -tags nogui ./... && go vet -tags noweb ./...
GOOS=openbsd go vet ./... && GOOS=freebsd go vet ./... && GOOS=darwin go vet ./...
go test ./...
```

CI (`.github/workflows/ci.yml`) runs the same checks plus race tests for the terminal, web and agent
packages. Tests must pass on a machine with no Misa or ugh configuration, so don't depend on files in
your home directory or on XDG variables.

## How changes are made here

- **Tests first.** Write the test for testable logic before the code, and check that it fails without your
  change. A bug fix comes with a regression test (in `internal/ugh`, name the file `*_regression_test.go`).
- **Small, decoupled files.** Prefer a new small file or package over growing a large one.
- **Self-documenting code.** Comments explain why, not what; if code needs a paragraph to follow, refactor it.
- **Project rules:** lipgloss for all UI styling (no raw ANSI in UI code), assets via `//go:embed`, no
  `//go:generate`, no cgo. Never write to the user's external shell history files.
- **One change per pull request**, with a description of what changed and why.

## Built-in commands and the conformance suites

`tests/` runs Misa's built-ins against upstream test suites (toybox, uutils, diffutils) plus Misa's own.
Cases Misa doesn't pass yet are listed in each suite's `known_failures.txt` and skipped;
`MISA_KNOWN_FAILURES=1 go test ./tests/...` runs them. When a case starts passing, delete its line.

Built-ins are written from the behaviour the tests and documentation describe. **Do not copy code from GNU
coreutils or other GPL projects**: the GPL is incompatible with this project's license. Code adapted from
permissively licensed projects needs a source and license comment at the top of the file, an entry in
`docs/CREDITS.md`, and, for substantial pieces, a `NOTICE` file in the package (see `THIRD_PARTY_NOTICES.md`).

## Licensing of contributions

Misa is licensed under the Open Software License 3.0. By submitting a contribution you agree that it is
licensed under the same terms, and you confirm that you have the right to contribute it.

## Plans and bigger work

Larger pieces of work are planned in `docs/plans/`. If you want to take one on, open an issue first (the Task template) so the
approach can be agreed before you write a lot of code. Security problems go to `SECURITY.md`, not to issues.
