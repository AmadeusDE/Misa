# Plan: ugh tool confinement

ugh's permission system decides whether a tool call may run. A decision is only as good as its link to what
the tool then does: the tool must touch the file, run the command and use the arguments that were judged.

## Done

- **One argument parser** (`internal/ugh/toolargs`): the gate and the tools read the same decoded values.
  Non-objects, duplicate keys, keys equal under case folding and trailing data are rejected.
- **Triage unwraps wrappers** (`perm/static/shell.go`): `timeout`, `nice`, `env -S`, `xargs`, `find -exec`,
  `sh -c`, `eval`; `sed` and `awk` are read-only only without write or exec features. Every non-flag argument
  is checked for sensitive paths, and paths are judged where their symlinks lead.
- **Fails closed**: when no model classifier can judge a call, the user is asked; headless runs refuse
  (`headless_prompt` defaults to deny).
- `AGENTS.md`/`CLAUDE.md` are read through `os.OpenRoot`, so a link out of the repository is not followed.
- `read` and `edit` open regular files only and read through a size limit.
- The permission prompt and its log scrub secrets from commands and paths.
- **File tools open what was judged**: `read`, `write`, `edit`, `ls`, `grep` and `find` resolve paths the way
  the rules do (`~` expanded, symlinks in the longest existing prefix resolved), and `read`, `write` and `edit`
  open the file through an `os.Root` of its directory, so a symlink planted after the decision can't lead out.
- **Worktree cleanup** deletes only the branch `Allocate` recorded, and only while git still has it checked
  out in that worktree.
- **Private directories** (`internal/ugh/privdir`): `/tmp/ugh-<uid>` is created 0700 and refused when it is a
  symlink, has another owner or other permissions; the user cache directory is the fallback.
- **Split panes on hidden tags**: a split session that ends while its mux tag isn't shown closes its pane
  there (`onHiddenTag`), as on the visible tag.

## Next

1. **Session files are not scrubbed (U9).** Session JSONL and `.blobs` keep prompts and tool output as the
   model saw them, private to the user (0600 files, 0700 directories); `perm/scrub.go` says so. Scrubbing on
   append would protect copies of the state directory, but `Scrub` also redacts ordinary code
   (`API_KEY = os.Getenv(…)`), so a resumed session would see different text than the original did. Decide
   whether to scrub only registered secret values (exact matches, no patterns) on append.

## Rules

- A regression test for every bypass, named `*_regression_test.go`, written to fail first.
- `internal/ugh/perm/static` must not import `os/exec` or `syscall` (a test enforces it).
- Secrets never appear in errors, logs or display output.
