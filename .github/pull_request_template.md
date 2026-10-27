## What and why

<!-- What changed and the reason for it. Link the issue or the docs/plans item, if any. -->

## How it was tested

<!-- The test you added, and that it failed before the change. For UI changes, what you ran by hand. -->

## Checklist

- [ ] `gofmt -l .` prints nothing; `go vet ./...` (also `-tags nough`, `nogui`, `noweb`) and `go test ./...` pass
- [ ] Tests came first; a bug fix has a regression test (`*_regression_test.go` under `internal/ugh`)
- [ ] One change in this pull request
- [ ] Docs updated where behaviour changed (`docs/AGENTS.md` architecture, `docs/UGH.md`, `docs/plans/`)
- [ ] No code copied from GPL projects; adapted permissive code is credited (`docs/CREDITS.md`)
- [ ] If an agent wrote this, it read `AGENTS.md`, and a person reviewed the diff
