# Misa

Misa (Modern Interactive Shell Alternative) is a terminal UI shell written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea). It wraps your commands in menus, a multiplexer and
a set of built-in tools, and it ships as one self-contained binary.

- A prompt with fish-style completion, zsh-format history, aliases, job control (`^Z`, `jobs`, `fg`, `bg`)
  and command correction
- A multiplexer: split panes, tags, a status bar, copy mode, mouse support
- Its own terminal emulator, so programs run inside panes
- Built-in commands (a coreutils subset, `fetch`, `sshc`, a vi, minesweeper, battleship, a [chess
  engine for fairy variants](docs/CHESS.md)), usable on systems with no shell at all (`builtin_mode: inprocess`)
- Chess includes Chess960 (`chess chess960`, randomized back ranks and position-bound castling) and
  Double Big Board (`chess doublebig`, two armies per side on 32×16), Amazon Chess
  (`chess amazon`), No-castling Chess (`chess nocastling`), Capablanca (`chess capablanca`, 10×8 with
  Archbishop and Chancellor), Berolina Chess (`chess berolina`, pawns move diagonally without capturing
  and capture straight ahead), Stalemate-wins Chess (`chess stalematewins`, delivering stalemate wins),
  Self-capture Chess (`chess selfcapture`, orthodox chess plus captures of your own non-royal pieces,
  after the self-capture rule set in [arXiv:2009.04374](https://arxiv.org/abs/2009.04374)), King of the
  Hill (`chess kingofthehill`, a king reaching one of the four centre squares wins at once), Grand
  Chess (`chess grand`), Christian Freeling's 10×10 game with marshals, cardinals and captured-only
  promotion, Three-Check (`chess threecheck`, giving check three times wins), Racing Kings
  (`chess racingkings`), V. R. Parton's 1961 race: no pawns or castling, no checks allowed, and the
  first king onto the eighth rank wins; Black answering there draws, Antichess (`chess antichess`):
  captures are compulsory and a side that loses all its pieces, or cannot move, wins, and Atomic
  (`chess atomic`): captures explode, destroying adjacent non-pawns, and blowing up the enemy king
  wins, and Crazyhouse (`chess crazyhouse`): captured pieces change sides and join the captor's
  pocket, from where they can be dropped back onto the board instead of moving.
- Minesweeper controls: `space` open, `f` flag, `n` new game, `1`–`3` select a level, `c` custom game, `q` quit
- Data-driven package manager menus, a file browser, and tldr/man help
- `misa --gui`: an optional Wayland window, pure Go and software rendered
- `misa --web`: Misa in a browser over a small authenticated web terminal
- `ugh`: a coding agent, embedded as a pane (`ugh`, or `ctrl+w a` for a split)

## Download

Tagged versions are on the [releases page](https://github.com/AmadeusDE/Misa/releases): a `.tar.xz` per
platform with `misa`, `ugh` and `ughbot` for Linux (amd64, arm64, riscv64), FreeBSD, OpenBSD and macOS, plus
Linux AppImages (x86_64, aarch64) that open Misa's Wayland window. Android Termux builds are
`misa-<tag>-android-arm64.tar.xz` and `misa-<tag>-android-amd64.tar.xz` (`misa` and `ugh` only; no `ughbot`).
Every commit on `master` builds the same files as one artifact of the
[Build workflow](https://github.com/AmadeusDE/Misa/actions/workflows/build.yml)
(a zip, which drops the executable bit: `chmod +x` after unpacking).

## Build

Go 1.27.1 or newer. There are no other build tools and no cgo, except the Android CLI,
which links against the NDK (bionic `getaddrinfo` and, on `android/amd64`, the external linker).

```sh
go build -o misa .
go build -o ugh ./cmd/ugh          # the agent's standalone CLI
go build -o ughbot ./cmd/ughbot    # experimental, see below

go build -tags nough -o misa .     # without the embedded agent (~19 MB instead of ~34 MB)
go build -tags nogui -o misa .     # without the Wayland window
go build -tags noweb -o misa .     # without misa --web
just appimage                      # dist/Misa-<version>-<arch>.AppImage

# Termux CLI. ANDROID_NDK_HOME is an NDK unpacked at the version in android/ndk.version.
ANDROID_NDK_HOME=/opt/android-ndk just android-cli          # dist/misa-android-arm64/
ANDROID_NDK_HOME=/opt/android-ndk just android-cli amd64    # dist/misa-android-amd64/
```

Linux is the main platform. OpenBSD, FreeBSD and macOS build and are checked with `go vet` in CI
but get less testing (OpenBSD: `GOTOOLCHAIN=go1.27.1`). The Android CLI targets NDK API 24
(`*-linux-android24-clang`, `CGO_ENABLED=1`). Windows is planned (`docs/plans/windows.md`);
the Android port is `docs/plans/android.md`.

## Run

```sh
./misa                       # first run opens the setup wizard
./misa --gui                 # in a Wayland window
./misa --web                 # in a browser, on 127.0.0.1:8787 (see docs/MISA_WEB.md)
./misa ls -la                # any built-in, as a multicall binary
./misa --list                # the built-ins
```

Configuration lives in `~/.config/misa/` (`config.yaml`, `mux.yaml`, `fetch.yaml`, `gui.yaml`,
`companion.yaml`), or next to the binary. Misa never reads configuration from the current directory.

## The three binaries

| Binary | Status | Docs |
|---|---|---|
| `misa` | Usable day to day | this file, `docs/MISA_WEB.md` |
| `ugh` | Usable; also embedded in `misa` | `docs/UGH.md` |
| `ughbot` | **Experimental.** A multi-bot daemon built on ugh: approvals, routines, coding tasks in throwaway VMs, a TUI and web UI. Developed on Linux. Expect breaking changes. | `docs/ughbot/` |

ugh needs model API keys (OpenRouter, OpenCode, or a Codex subscription). Its permission system asks a
model classifier about risky tool calls; without a working classifier, those calls are put to you, and
headless runs (`ugh -p`) refuse them.

## Contributing

See `CONTRIBUTING.md`. Agents working on the code should read `AGENTS.md`. Security reports:
`SECURITY.md`. Plans for upcoming work are in `docs/plans/`.

## License

Copyright (c) 2026 unix-supremacist.

Licensed under the [Open Software License version 3.0](LICENSE) (OSL-3.0). The OSL treats making the
software available to others over a network (for example running `misa --web` or `ughbot` for other
people) as distribution (its section 5), so the same obligations apply, including offering the source of
modified versions.

Third-party code and data keep their own licenses: see `THIRD_PARTY_NOTICES.md` and `docs/CREDITS.md`.
