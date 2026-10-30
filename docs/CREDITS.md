## Companion Response Templates

- **cargo-mommy** by Aria Beingessner — MIT / Apache-2.0
  https://github.com/Gankra/cargo-mommy

- **mommy** by Florine Dekker — The Unlicense (public domain)
  https://github.com/FWDekker/mommy

## Built-in Vi Editor

- **toybox vi** by Rob Landley et al. — 0BSD
  https://github.com/landley/toybox

- **vis** by Marc Andre Tanner — ISC
  https://github.com/martanne/vis

## Option Parser

- **gobox** pkg/args — 0BSD
  https://github.com/mveety/gobox

## File Icons

- **g** by Equationzhao — MIT
  https://github.com/Equationzhao/g

## ASCII Art

- **pfetch** by Dylan Araps — MIT
  https://github.com/dylanaraps/pfetch

- **neofetch** by Dylan Araps — MIT
  https://github.com/dylanaraps/neofetch

- **fastfetch** by Linus Dierheimer et al. — MIT
  https://github.com/fastfetch-cli/fastfetch

## Color Palettes

- **hyfetch** by Azalea Gui — MIT
  https://github.com/hykilpikonna/hyfetch

## Command Correction

- **thefuck** by Vladimir Iakovlev — MIT
  https://github.com/nvbn/thefuck

## AI Command Help

- **Gemhelp** (`internal/aihelp`: system prompts, Arch Wiki client, response cache, `--fix` mode) by the Misa author — OSL-3.0
  https://github.com/AmadeusDE/Gemhelp

## Coreutils Reference

Builtins are Go implementations written to pass these projects' test suites. Some follow upstream source closely
(their file headers say "adapted from" or "ported from"), and the test suites under `tests/` are copied or ported
from upstream. License texts are in `THIRD_PARTY_NOTICES.md` and next to the copied tests.

- **toybox** by Rob Landley et al. — 0BSD
  https://github.com/landley/toybox

- **uutils/coreutils** by the uutils contributors — MIT
  https://github.com/uutils/coreutils

- **uutils/diffutils** by the uutils contributors — MIT / Apache-2.0
  https://github.com/uutils/diffutils

- **uutils/util-linux** by the uutils contributors — MIT (`cal`, `mcookie`, `uuidgen`)
  https://github.com/uutils/util-linux

## Chess

Original Go implementation; rules and notation referenced from:

- **Betza's funny notation** by Ralph Betza — piece movement notation used for variant files
  https://www.chessvariants.com/piececlopedia.dir/betzanotation.html
- **Double Chess** (Julian S. Hayward, 1916) and **Chess on a Really Big Board** (Ralph Betza, 1996) — rules as
  described on Wikipedia and The Chess Variant Pages
  https://en.wikipedia.org/wiki/Double_chess, https://en.wikipedia.org/wiki/Chess_on_a_really_big_board
- **Perft results** from the Chess Programming Wiki — node counts used as move-generation tests
  https://www.chessprogramming.org/Perft_Results
- **Search techniques** (PVS, null move, late move reductions, killer/history ordering, Zobrist hashing) as
  described on the Chess Programming Wiki — original implementation, no code copied
  https://www.chessprogramming.org

## Wayland Frontend

- **xkbcommon-keysyms.h** (libxkbcommon, from X11 keysymdef.h) by The Open Group et al. — MIT/X11
  https://github.com/xkbcommon/libxkbcommon
  Keysym names and Unicode mappings in `internal/xkb/keysyms.txt` are extracted from it.

- **Wayland protocol** (wayland.xml) and **wayland-protocols** (xdg-shell, xdg-decoration, cursor-shape) — MIT
  https://gitlab.freedesktop.org/wayland
  Opcodes and message layouts in `internal/wayland` and `internal/gui` follow the protocol XML.

## Go Libraries

- **Bubble Tea** by Charmbracelet — MIT
  https://github.com/charmbracelet/bubbletea

- **Bubbles** by Charmbracelet — MIT
  https://github.com/charmbracelet/bubbles

- **Lip Gloss** by Charmbracelet — MIT
  https://github.com/charmbracelet/lipgloss

- **yaml.v3** by go-yaml — MIT / Apache-2.0
  https://github.com/go-yaml/yaml

- **clipboard** by Ato Araki — BSD-3-Clause
  https://github.com/atotto/clipboard

- **x/crypto** (ssh, ssh/agent, ssh/knownhosts) by The Go Authors — BSD-3-Clause
  https://cs.opensource.google/go/x/crypto

- **x/image** (font/opentype, font/sfnt, Go Mono fonts) by The Go Authors / Bigelow & Holmes — BSD-3-Clause
  https://cs.opensource.google/go/x/image

- **Glamour** by Charmbracelet — MIT
  https://github.com/charmbracelet/glamour

- **charmbracelet/x** (ansi, term) by Charmbracelet — MIT
  https://github.com/charmbracelet/x

- **toml** by the TOML authors — MIT
  https://github.com/BurntSushi/toml

- **pty** by Keith Rarick — MIT
  https://github.com/creack/pty

- **regexp2** by Doug Clark — MIT
  https://github.com/dlclark/regexp2

- **go-runewidth** by Yasuhiro Matsumoto — MIT
  https://github.com/mattn/go-runewidth

- **go-keyring** by Zalando SE — MIT
  https://github.com/zalando/go-keyring

- **x/sys**, **x/term** by The Go Authors — BSD-3-Clause
  https://cs.opensource.google/go/x

- **sqlite** (modernc.org) by The Sqlite Authors — BSD-3-Clause
  https://gitlab.com/cznic/sqlite

- **sh** by Daniel Martí — BSD-3-Clause
  https://github.com/mvdan/sh

- **qr** by The Go Authors — BSD-3-Clause
  https://github.com/rsc/qr
