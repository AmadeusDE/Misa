# Plan: TUI package manager frontend (pacman + OpenBSD pkg_*)

Status: draft. Nothing here is implemented yet.

## Goal

A full package manager inside Misa: browse, search, inspect, mark, install, remove and upgrade
packages from one screen, on Arch-family systems (pacman) and on OpenBSD (`pkg_add`, `pkg_info`,
`pkg_delete`). It should feel like Misa: keyboard and mouse driven, mux-aware, lipgloss styled,
and fast on a 15k-package repository.

Out of scope for the first version: AUR builds (helpers are an optional later phase), Flatpak (it
keeps its YAML action menu), and repository/mirror configuration editing.

## What exists today

- `internal/pkgmanager` defines data-driven `Manager`s. Each has `Actions` that either `Run` an
  argv, prefill the shell (`ShellPrefill`), or show a `List` built from a command's output. pacman,
  flatpak, git and power are built in, and more can be added as YAML in `~/.config/misa/managers/`.
- `internal/ui/pkgmenu.go` turns those into menu items. Privileged actions run as `sudo pacman …`
  through `Model.launch`, which runs them in the pane's own PTY. Password prompts and progress bars
  already work there.
- Misa already builds and is vetted for OpenBSD.

The YAML menus stay as they are. They suit simple command launchers. The new frontend is a
separate, typed layer for real package managers. A YAML manager can point at it (see Phase 5).

## Architecture

```
internal/pkgdb/                 backend-neutral model + interface (no UI, no Bubble Tea)
  pkgdb.go                      Package, Details, Update, Transaction, Backend interface
  detect.go                     pick the backend for this system
  pacman/                       Arch/pacman backend
    localdb.go                  parse /var/lib/pacman/local/*/desc
    syncdb.go                   read /var/lib/pacman/sync/*.db (gzip tar of desc files)
    updates.go                  checkupdates-style upgrade list (never a partial -Sy)
    exec.go                     transactions: argv builders for -S/-R/-Syu/-Rns
  openbsd/                      OpenBSD pkg_* backend
    installed.go                parse /var/db/pkg/*/+CONTENTS and +DESC
    index.go                    fetch/parse the mirror's index.txt (installurl + release + arch)
    updates.go                  pkg_add -u -n dry-run parsing
    exec.go                     transactions: pkg_add / pkg_delete argv builders
internal/ui/pkg/ (or ui/pkgfront*.go)
  screen.go                     new screen: tabs, list, details, action bar
  list.go                       virtualized list (handles 15k rows; filter is incremental)
  details.go                    details pane (deps, size, files, URL, install reason)
  txn.go                        transaction review modal + run in the pane PTY
```

### Backend interface

```go
type Backend interface {
    Name() string                                     // "pacman", "openbsd"
    Installed(ctx context.Context) ([]Package, error) // fast: reads the local db
    Available(ctx context.Context) ([]Package, error) // sync db / mirror index
    Details(ctx context.Context, name string) (Details, error)
    Updates(ctx context.Context) ([]Update, error)    // must not modify system state
    Orphans(ctx context.Context) ([]Package, error)
    Plan(op Op, names []string) (Transaction, error)  // argv + human summary, no execution
}

type Package struct {
    Name, Version, Repo, Desc string
    Installed, Explicit       bool   // Explicit = user-requested, not a dependency
    Size                      int64
}

type Transaction struct {
    Argv     []string // exactly what will run, shown to the user before running
    Root     bool     // needs doas/sudo
    Summary  string
    DryRun   []string // optional argv that previews the change (pkg_add -n, pacman -p)
}
```

Rules:

- Backends never run privileged commands. They return a `Transaction`, and the UI shows its exact
  argv in a review modal, then runs it with `Model.launch` in a PTY. Password prompts, `pacman`'s
  `[Y/n]` confirmation and progress bars stay native. Job control (Ctrl+Z, `fg`) also works.
- Read-only queries parse on-disk databases directly wherever possible, so listing is instant and
  needs no root. They fall back to running the tool (`pacman -Si`, `pkg_info`) when a file format is
  missing or unknown.
- Privilege: use `doas` when it exists and is configured (the OpenBSD default), otherwise `sudo`.
  Make it configurable as `pkg.privilege: doas|sudo|none` (`none` when Misa already runs as root,
  for example in the busybox-style box mode).

## pacman backend

| need | source |
|---|---|
| installed list | `/var/lib/pacman/local/<name-ver>/desc` (`%NAME%`, `%VERSION%`, `%DESC%`, `%REASON%`, `%SIZE%`, `%DEPENDS%`) |
| available list | `/var/lib/pacman/sync/*.db`: gzip tar archives of the same `desc` format, one per repo (`core`, `extra`, `cachyos-*`, `chaotic-aur`…). Repo order comes from `pacman.conf` |
| details | the parsed `desc` entries. Remote file lists via `pacman -Fl` only if the files db exists |
| updates | `checkupdates` (pacman-contrib) when installed, otherwise the same trick by hand: `fakeroot pacman -Sy --dbpath <tmp copy>` then compare. **Never run `pacman -Sy` against the real db** (partial-upgrade hazard) |
| orphans | `%REASON%` = 1 (dependency) and nothing installed depends on it, or `pacman -Qdtq` |
| install | `sudo pacman -S <names>` |
| remove | `sudo pacman -Rns <names>` (option: `-R` only) |
| upgrade | `sudo pacman -Syu` (whole system only; never upgrade single packages after a sync) |
| clean cache | `sudo paccache -r` if present, else `sudo pacman -Sc` |
| history | `/var/log/pacman.log` (install/upgrade/remove lines, newest first) |

Notes:

- Parse `pacman.conf` `[repo]` sections (with `Include` files) to get repo order and to skip
  disabled repos. A repo's db file with no section is ignored.
- Versions compare with pacman's `vercmp` rules (epoch:pkgver-pkgrel, alnum segments). Port the
  algorithm and test it against `vercmp` outputs recorded as fixtures.
- AUR (optional, Phase 6): if `paru` or `yay` exists, offer AUR search via `<helper> -Ssa` and
  install via the helper in the PTY. Never parse or build PKGBUILDs ourselves.

## OpenBSD backend

| need | source |
|---|---|
| installed list | `/var/db/pkg/<stem-version>/+CONTENTS` (`@name`, `@depend`, `@comment`, `@option manual-installation`) and `+DESC` (first line = comment) |
| explicit vs dependency | `@option manual-installation` in `+CONTENTS`, or `pkg_info -m` |
| available list | `index.txt` from `$(cat /etc/installurl)/<release>/packages/<arch>/`. Release is `uname -r`, or `snapshots` on -current (detect via `sysctl kern.version` containing `-current`/`-beta`). Arch is `uname -m`. Fetch with `ftp -o -` (base system, no extra deps) or Go `net/http`, and cache it under `~/.cache/misa/pkg/` with the mirror's timestamp |
| details | `pkg_info <pkg>` (works for remote packages via `installurl`) |
| updates | `pkg_add -u -n` (dry run) and parse the `old->new` lines |
| orphans | `pkg_delete -a -n` (dry run lists what autoremove would delete) |
| install | `doas pkg_add <stems>`; flavors/branches are chosen in the UI (`vim--no_x11`, `python%3.12`) |
| remove | `doas pkg_delete <stems>`, plus an option for `pkg_delete -a` afterwards |
| upgrade | `doas pkg_add -u` |
| system | separate "System" tab actions, not packages: `doas syspatch`, `doas fw_update`, `doas sysupgrade` (each with a clear warning; sysupgrade reboots) |

Notes:

- OpenBSD names are `stem-version[-flavor]`, and the same stem can exist with several flavors and
  branches. The UI groups by stem and shows flavors as a sub-choice instead of separate rows.
- `pkg_add` prompts interactively for ambiguous choices. Running it in the PTY keeps that working,
  and the dry run shows the choice before the user commits.
- Test on an OpenBSD machine or VM. Record `+CONTENTS`, `index.txt` and `pkg_add -u -n` outputs as
  fixtures so unit tests run on Linux too.

## UI

```
┌ Packages ── pacman · 1808 installed · 12 updates ──────────────────────────────┐
│ [Installed] Available  Updates(12)  Orphans(3)  History        / search…      │
├─────────────────────────────────────────┬──────────────────────────────────────┤
│ * bash         5.3.20-2   core  2.1MB   │ bash 5.3.20-2  (cachyos-v3)          │
│   bat          0.25.0-1   extra 4.4MB   │ The GNU Bourne Again shell           │
│ U curl         8.17.0-1 → 8.18.0-1      │ Installed explicitly · 9.3 MB        │
│ D gcc-libs     15.1.1-2   core          │ Depends: readline glibc ncurses      │
│ …                                       │ Required by: 214 packages            │
├─────────────────────────────────────────┴──────────────────────────────────────┤
│ space mark  i install  r remove  u upgrade all  enter details  ? help  esc back│
└────────────────────────────────────────────────────────────────────────────────┘
```

- Tabs: Installed, Available, Updates, Orphans, History (pacman only) and System (OpenBSD only).
  Mouse: click tabs and rows, wheel scrolls, double-click opens details (the Misa list
  conventions in `mouse.go`).
- Badges: `*` explicit, `D` dependency, `U` upgradable, `O` orphan, `M` marked. Colors come from
  `config.yaml ui:`, never hard-coded.
- Search is incremental over name and description, with fuzzy ranking reused from the existing
  filter code. The list is virtualized (only visible rows render), because bubbles' `list` is too
  slow with 15k items.
- Marking: space toggles. `i`/`r` act on all marked rows (or the cursor row). The review modal
  shows the `Transaction.Argv`, the dry-run output when the backend has one, and
  "Run / Cancel / Copy to shell". Copy-to-shell prefills the Misa prompt, as `ShellPrefill` does
  today.
- Running: `Model.launch(txn.Argv...)` runs it in the pane PTY. When it exits the screen comes
  back, caches are invalidated and the tab refreshes. A non-zero exit leaves the output in the
  shell buffer and shows the status.
- Data loading is always a `tea.Cmd`; the UI never blocks. Show a spinner on a tab while it loads,
  and serve cached results immediately.
- Entry points: the existing "Pacman" menu item opens the frontend, and so does a new "Packages"
  root item where a backend is detected. Also a `pkg` Misa builtin (`pkg search foo`,
  `pkg install foo`, `pkg ui`) for shell users. In mux, it opens in the current pane like any
  other screen.

## Config

`config.yaml`:

```yaml
pkg:
  backend: auto          # auto | pacman | openbsd
  privilege: auto        # auto (doas if configured, else sudo) | doas | sudo | none
  aur_helper: auto       # auto | paru | yay | none  (pacman only, Phase 6)
  confirm: true          # show the review modal before running
```

## Phases

1. **pkgdb core + pacman read-only.** Covers the types, local and sync db parsing, `pacman.conf`
   repos, vercmp and orphans, all with fixture tests. A benchmark parses the real sync dbs
   (target: under 150 ms cold for about 15k packages).
2. **UI read-only.** Adds tabs, the virtualized list, search, details, mouse, and loading states.
   UI tests use a fake `Backend`.
3. **Transactions (pacman).** Adds marking, the review modal, privilege detection and launch in
   the PTY, refresh after completion, and the history tab from `pacman.log`.
4. **OpenBSD backend.** Adds `+CONTENTS` and `index.txt` parsing, flavors, the dry-run parsers
   and the System tab. Unit tests use fixtures, and a manual checklist runs on the QEMU VM.
5. **Integration.** The YAML `Manager` gains `frontend: pacman|openbsd` so menu items open the new
   screen. Adds the `pkg` builtin and docs (CLAUDE.md Key Patterns, the README section).
6. **Optional:** AUR via paru/yay, then other backends behind the same interface (apk, xbps,
   FreeBSD `pkg`) if wanted.

## Testing

- Parsers: fixtures in `testdata/` (pacman `desc` files, a small sync `.db` built in the test with
  `archive/tar`, a `pacman.conf` with includes, OpenBSD `+CONTENTS`/`index.txt`/dry-run output).
- vercmp: a table of pairs recorded from real `vercmp` (epochs, pkgrel, alpha/numeric runs).
- Backend contract tests run against a fake filesystem root (`Backend` takes a root path), so
  nothing reads the host's real db in CI.
- UI: a fake backend plus the existing pump helpers. These check tab switching, marking, the review
  modal argv, and refresh after `termExited`.
- No test ever runs a privileged command. Transactions are asserted as argv only.

## Risks and decisions to confirm

- **Direct db parsing vs running the tools.** Direct parsing is much faster and needs no root, but
  it ties Misa to the on-disk formats. These have been stable for years on both systems, and the
  plan falls back to running the tools.
- **Update checks without root.** `checkupdates` needs pacman-contrib; the fallback copies the db
  to a temp dir. On OpenBSD `pkg_add -u -n` needs root for some checks, so it may have to run via
  doas, with a prompt.
- **Large lists.** A custom virtualized list is new code to maintain; the alternative is paging
  bubbles' list.
- **Where it lives:** a top-level screen (proposed), or a mux pane kind so it can sit beside a
  shell. A screen already works inside any mux pane, so a new pane kind isn't needed.
