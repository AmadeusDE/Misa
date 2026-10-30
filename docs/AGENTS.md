# Misa — Modern Interactive Shell Alternative

Guidance for coding agents. `docs/AGENTS.md` is the only real file: `docs/CLAUDE.md` and `docs/GEMINI.md` are
symlinks to it, and the root `AGENTS.md`, `CLAUDE.md` and `GEMINI.md` link into `docs/`. Edit `docs/AGENTS.md`
directly, never through a link (some write tools replace a symlink with a file). `agentlinks_test.go` checks the links.

TUI shell alternative built with Go and Bubble Tea v2 (`charm.land/bubbletea/v2`, lipgloss v2, bubbles v2).

## Build & Test

```sh
go build -o misa .
go test ./...
go vet ./...
go build -tags nough -o misa .   # without the embedded ugh agent (~19 MB instead of ~34 MB)
go build -tags nogui -o misa .   # without the Wayland window frontend (saves ~1.5 MB)
go build -tags noweb -o misa .   # without `--web` (browser terminal, see docs/MISA_WEB.md)
go build -o ughbot ./cmd/ughbot # the ughbot daemon and CLI (experimental, see docs/ughbot/DEPLOY.md)
gofmt -l .                       # must print nothing
GOOS=openbsd go vet ./... && GOOS=freebsd go vet ./... && GOOS=darwin go vet ./...

# Android CLI (cgo). GOOS=android also matches the linux build tag, so a file
# tagged linux is built for Android too. Where Android must differ, tag the
# Linux file `linux && !android` and add an android sibling. import "C" is
# allowed only in files tagged `android && cgo` (cgo_constraint_test.go).
# Every other target stays CGO_ENABLED=0. NDK pin: android/ndk.version.
# CC is the NDK's *-linux-android24-clang (API 24).
ANDROID_NDK_HOME=... just android-cli          # dist/misa-android-arm64/ (misa, ugh; no ughbot)
ANDROID_NDK_HOME=... just android-cli amd64    # dist/misa-android-amd64/
# GUI library (c-shared, API 26 / minSdk, -tags nough). JNI class app/misa/Native.
ANDROID_NDK_HOME=... just android-lib          # dist/libmisagui-arm64.so
ANDROID_NDK_HOME=... just android-lib amd64    # dist/libmisagui-amd64.so
# APK shell. build-go.sh writes jniLibs/<abi>/{libmisa.so,libmisagui.so}
# (CLI at API 24; c-shared GUI at API 26, -tags nough). version.sh is the
# versionName/versionCode used by Gradle and CI.
ANDROID_NDK_HOME=... ANDROID_HOME=... just apk
ANDROID_NDK_HOME=... ANDROID_HOME=... just apk-release
# Vet matches CI: CGO_ENABLED=1 for both arches (and tags nough, nogui, noweb).
# CGO_ENABLED=0 go vet ./... is arm64 only. android/amd64 cannot load test
# binaries without the external linker, so that arch typechecks libraries
# with go build. The arm64 vet covers the pure-Go android files.
```

Single binary, no external build tools needed except the Android NDK for `GOOS=android`
and, for the APK, a JDK 17 plus the Android SDK.
Go 1.27.1 (OpenBSD: `GOTOOLCHAIN=go1.27.1`). CI (`.github/workflows/ci.yml`)
runs gofmt, the builds, vet for every tag and the other unixes, the tests, and an `android`
job (NDK r30 from `android/ndk.version`, both ABIs, the tag set, cgo and cgo-less vet,
`libmisagui.so` for both ABIs, then `./android/build-go.sh` and `./gradlew assembleDebug lint`,
and an unsigned release APK built twice and diffed).
`android-device` (path-filtered, plus nightly and `workflow_dispatch`) runs the portable
test binaries on an emulator, including `internal/gui/android`, then installs the debug APK,
checks logcat for an `app.misa` crash, and uploads a screenshot.
`build.yml` builds the binaries
for every platform, including `android-arm64` and `android-amd64` CLI trees, per-ABI and
universal debug APKs (release-signed on `v*` tags when `ANDROID_KEYSTORE_B64` is set), and the AppImages
(`packaging/appimage/build.sh`, appimagetool pinned by checksum); a pushed
`v*` tag publishes them as a release. The Android tarball is `misa` and `ugh` only.
ugh lives in this repo: `internal/ugh/*` (agent, tools, llm, perm, session, …; `internal/ugh/ughtui` is the embeddable TUI)
and `cmd/ugh`. Three binaries: `go build -o misa .`, `go build -o ugh ./cmd/ugh`, `go build -o ughbot ./cmd/ughbot`.
ugh's user documentation is `docs/UGH.md`; ughbot's is `docs/ughbot/`.
Tests must pass with no user configuration: don't depend on `~/.config`, the XDG variables or a global git config.
Dependency direction (enforced by `internal/ugh/deps_test.go`): Misa libraries <- `internal/ugh/*` <- `internal/ughbot/*`.

## Architecture

```
main.go                          Entry point — starts Bubble Tea program; `--gui`/`misa-gui` opens the Wayland window,
                                 `--ssh`/`misa-ssh` opens it running sshc; `--web`/`--web-admin` serve it in a browser (main_web.go, `-tags noweb` stub); guiCommand resolves builtins not on PATH
main_gui.go main_nogui.go        runGUI, or its `-tags nogui` stub
internal/
  ugh/                           ugh agent harness merged in (agent, tools, llm, perm, policy, session, ughtui, …); cmd/ugh is its CLI
  ughbot/                        ughbot daemon (docs/ughbot/DEPLOY.md); cmd/ughbot is its CLI:
                                 config (ughbot.toml, capabilities decide a bot's tools), store (SQLite: bot defs, mail with
                                 kinds, ledger, approvals), bot (one resumable agent per bot, bot-to-bot and create_bots tools),
                                 daemon (bots + a lock-free view the bots read, approval queue: hash-bound, 72h expiry = deny,
                                 routines: scheduled mail, fired in one transaction with its mail; repo grants: request_repo ->
                                 add_repo approval, clones in state_dir/repos, code_grants table), sched (cron + at times),
                                 api/client (HTTP over the unix socket), ledger (reserve/settle spend caps),
                                 tui (Bots/Chat/Approvals/Routines/Repos/Cost/Status/Devices; bot-manager preset), webhost (browser terminal)
  ui/
    app.go                       Root model, screen dispatch, menu navigation, shell input
    statusbar.go                 Three-column header: distro ASCII | info items | anime ASCII
    settings.go                  Settings TUI with picker/select/text/number controls
    onboarding.go                First-run setup wizard (companion pieces → theme → info → art → history → shell → mux)
    companion_setup.go           Companion piece pickers (tone, role, address, pronouns, flavor, moods)
    picker.go                    Searchable toggle-list with preview (themes, art, distro)
    pkgmenu.go                   Data-driven package manager menus (enterShellWith, showListAction)
    filebrowser.go               File browser (navigate, open, create, delete)
    helpscreen.go                TLDR/man page viewer with filterable lists
    about.go                     System information screen
    shell.go                     Shell command execution with exit code capture
    keyinput.go                  Key and paste encoding for PTYs (xterm sequences from v2 Code/Mod/Text)
    encode.go                    KeyBytes/MouseBytes/PasteBytes: those encoders exported for the gui frontend
    renderinput.go               renderInput: textinput view with the whole placeholder (bubbles v2 cuts it at Width 0)
    agent.go                     Embedded agent sessions (ugh): `ugh` takes the pane over, agentSession, agentHost mailbox
    agentsplit.go                Split sessions: ctrl+w a, detach on pane close, ctrl+w A reattach, root routing
    agent_ugh.go                 ughtui adapter (`!nough`); without it `ugh` runs the external binary
    agentbar.go                  Mux bar session summary and tags flagged while a session waits for the user
    dispatch.go                  Command planning — builtins (multicall/in-process), direct exec, sh -c only for shell syntax
    route.go                     paneMsg/routeTo/tagCmd — tags pane commands so results return to the owning Misa pane
    mousefwd.go                  Mouse passthrough to programs (xterm 9/1000/1002/1003, SGR/X10), alt-screen wheel scroll
    hostcolors.go                Requests the host terminal fg/bg (tea.Request*Color) for OSC 10/11 replies
    shellcomp.go                 Fish-style tab completion, zsh-format history, external history import
    fullshell.go                 Full-screen shell mode (alt-screen pane)
    mux.go                       Terminal multiplexer — split panes, mux bar, pane management
    mouse.go                     Mouse event helpers — click-to-select, scroll wheel, pane focus (click or hover)
    muxframe.go                  Root frame reuse for hover events that change nothing on screen
    muxconfig.go                 applyMuxConfig and the muxConfigChanged message from panes to the root
    jobs.go                      Job control — stopped programs become jobs; jobs/fg/bg builtins; closeTerminals/Shutdown
    editor.go                    Built-in editor integration (vi wrapper)
    delegate.go                  Compact list delegate using titled interface
    items.go                     menuItem and fileItem types
  builtins/                      Shell built-in commands (coreutils subset + top-commands)
    registry.go                  Command registry — Register/Lookup/Names
    args.go                      POSIX/GNU option parser with compact spec strings
    topcommands.go               History analytics — ranked command frequency across shells
    simple.go                    echo, true, false, yes, printenv
    ls.go cat.go cp.go mv.go     File utilities
    rm.go mkdir.go touch.go      File operations
    chmod.go chown.go link.go    Permissions and links
    tail.go proc.go sysutil.go   System utilities
    man.go install.go util.go    Misc helpers
    fetch.go                     System fetch builtin — ASCII art + info display (mfetch-style)
    sshc.go                      `sshc` — OpenSSH-style flags mapped onto sshclient.Options
    mines.go                     `mines` — level flags (-b/-i/-e, --width/--height/--mines) for the game
    chess.go                     `chess [variant] [-f file.yaml] [--fen pos] [-c white|black|both|none] [--level name] [--think s] [-l]`
                                 — ChessGame builds the game (computer plays Black by default)
    battleship.go                `battleship [-r|--random]` — -r picks a random fleet instead of interactive
                                 placement
  sshclient/                     Built-in SSH client (golang.org/x/crypto/ssh); named sshc so it never shadows OpenSSH
    client.go                    Run: dial, remote PTY + raw local terminal, exit status (remote's, or 255)
    target.go                    Destination ([user@]host, ssh://) + flags + config → address, user, identities
    config.go                    ~/.ssh/config and /etc/ssh/ssh_config subset: Host (wildcards, !), Include, first value wins
    hostkeys.go                  known_hosts check, ask/accept-new/yes trust, changed-key refusal, known key algorithms
    auth.go                      Agent, identity files (passphrase asked only when the server accepts the key),
                                 keyboard-interactive, password
    escape.go resize.go          `~.` disconnect / `~~`; SIGWINCH plus polling (in-process panes get no SIGWINCH)
    prompt.go                    Terminal prompts read byte-wise so typed-ahead input still reaches the remote
  lscore/                        ls formatting engine — colors, icons, git status, sorting
  mines/                         Minesweeper clone: board.go (rules, first reveal always safe, flood fill, chording),
                                 game.go (keys/mouse, clock), view.go (centred layout + hit test), run.go (standalone).
                                 Game satisfies procmon.Viewer, so typing `mines` in Misa opens it as an in-process screen
  battleship/                    Battleship on a standard 10×10 grid (fleet 5-4-3-3-2): board.go (rules, placement,
                                 shots), ai.go (hunt/target), game.go view.go run.go (interactive placement, then
                                 alternating shots; satisfies procmon.Viewer, so typing `battleship` in Misa opens it
                                 in-process — the game also sits in the Games submenu of internal/ui/app.go)
  chess/                         Data-driven chess for any board size and fairy pieces, vs computer or hotseat:
    betza.go                     Betza funny-notation subset → Components (atoms, riders, m/c/f/b/l/r/s/v/i/n/p/g/q)
    variant.go                   Variant YAML (embedded variants/ + chess/*.yaml next to binary/~/.config/misa),
                                 `extends:` inheritance, setup in FEN with multi-digit runs, castling, promotion
    position.go                  Move generation (walk per component; a full open circle of a q rider is a pass),
                                 legality (no royal left attacked), castling, en passant on every crossed square,
                                 Apply (keeps royal squares), ParseFEN; captures() for quiescence
    attack.go                    anyAttacked, pre-filtered by per-type reach tables (offsets a piece could ever hit)
    zobrist.go                   Position.Hash: pieces, moved flags that matter, side, en passant
    match.go                     Match: history/undo, long algebraic notation, checkmate/stalemate/repetition/
                                 move-limit/bare-royals endings, Hashes for the search
    eval.go                      Piece values (YAML `value`, else estimated from empty-board mobility) and a
                                 variant-agnostic evaluation: material, centre, pawn race, royal shelter/edge
    search.go                    Engine: iterative deepening PVS, TT, quiescence, null move, LMR, killers/history,
                                 check extension, repetition vs game history; Limits{Time, Depth, Nodes, Stop}
    difficulty.go                Typed levels (normal default), shallow weak profiles with isolated tables,
                                 seeded full-window root candidate selection; --think changes time only
    game.go view.go run.go       procmon.Viewer screen: cursor/mouse, selection hints, promotion prompt, flip,
                                 letters toggle, move list panel; the computer thinks in a tea.Cmd (thoughtMsg,
                                 gen drops abandoned searches); standalone runner
  vi/                            Built-in vi editor implementation
  ascii/                         ASCII art loading (embedded + external dirs). Android and Termux art
                                 live under distro/pfetch; ids come from sysinfo (termux / android)
  clipboard/                     Host clipboard: atotto on Linux/BSD/macOS, termux-clipboard-* when on
                                 PATH, ErrUnavailable so the UI emits OSC 52. APK JNI replaces it later
  color/                         Color palette loading, hex-to-ANSI, color translation
  config/                        YAML config — config.yaml (menu, ascii, colors, ui, statusbar), mux.yaml, fetch.yaml, gui.yaml
  companion/                     Encouraging messages after command (cargo-mommy derived)
    companion.go                 Respond(), template vars ({role}, {part}, {item}, {prefix}…)
    config.go                    companion.yaml selections + write-ins
    assemble.go                  Build line pools from embedded yaml + selected pieces
    load.go                      //go:embed data/ moods, modifiers, kemonos, catalog
    pieces.go                    Catalog lists for the setup UI
    data/moods/                  normal, needy, ominous, thirsty (NSFW toggle)
    data/modifiers/              mommy, sister, tsundere, … combo lines via `when:`
    data/kemonos/                your/ (puppy) and companion/ (catgirl, …)
    data/catalog.yaml            pronouns, call terms, prefixes, items, parts, emotes
  thefuck/                       Command correction rules — pattern-based fix suggestions
  pkgmanager/                    Data-driven package manager definitions (built-in + external YAML).
                                 Termux adds `pkg` when TERMUX_VERSION or a com.termux PREFIX is set.
                                 MISA_ANDROID_APP=1 hides managers with Distro set (pacman, flatpak, pkg)
  sysinfo/                       System info fetching (basic + extended). Android/Termux detection:
                                 TERMUX_VERSION, PREFIX, MISA_ANDROID_APP, and getprop
                                 (ro.build.version.release, ro.product.model, ro.product.manufacturer)
                                 via __system_property_get (android && cgo) or /system/bin/getprop.
                                 EACCES on proc files used for memory/uptime/kernel shows "—"
  gitinfo/                       Git branch/dirty/ahead/behind detection
  help/                          TLDR downloading, man page parsing, markdown rendering
  launcher/                      Bubble Tea ExecProcess helper (unused: external programs run in-pane PTYs via Model.launch)
  jobleader/                     `misa --job-leader cmd…`: session leader that runs cmd in its own foreground process
                                 group so ^Z stops it; SuspendOnCtrlZ for multicall builtins
  term/                          VT emulator + PTY: reader goroutine, DEC 2026 sync buffering, session-wide kill on Close.
                                 terminal.go (sessions, StartArgv/StartFunc), parser.go, screen.go, sgr.go, charset.go,
                                 reports.go, render.go, mouse.go, input.go (async PTY writes), ptyfd.go (pollable master),
                                 jobs.go (wait4 with WUNTRACED: Stopped/Continue)
                                 snapshot.go (Snapshot: cell copy for frontends; SetOSCHook for titles/OSC 52),
                                 marks.go (combining marks interned so Cell has no pointers: the GC never scans
                                 screens/scrollback — keep Cell pointer-free, a test enforces it). Scrolled-off rows
                                 move into scrollback and evicted/discarded rows are recycled (blankRow), so steady
                                 output doesn't allocate; putASCII writes plain ASCII runs in one pass.
                                 `go test -bench ProcessOutput ./internal/term/` measures parser throughput.
                                 session_linux.go walks /proc for the PTY session on Android too
                                 (GOOS=android matches linux). Unreadable pids are skipped; if /proc
                                 cannot be listed, kill falls back to the leader's process group.
                                 pidfd is preferred; a refused pidfd (Android before 12) signals by pid
  gui/                           Window frontend (`misa --gui [cmd…]`). OS-neutral core plus one backend
                                 per platform. The core owns the terminal, the renderer, damage, the OSC
                                 queue, fonts, and key/mouse/paste translation. Linux other than Android
                                 presents through Wayland (`unix && !android`; Android matches `unix`).
                                 Elsewhere `Run` errors cleanly until that platform's backend exists
    app.go                       Loop, hold, OSC title/52, sizing, zoom, frame pacing
    backend.go                   Backend, Event, Clipboard, PixelFormat (BGRA or RGBA)
    pointer.go                   Pointer position → cell; buttons and wheel → ui.MouseBytes
    shortcuts.go                 ctrl+shift+v and shift+insert paste; ctrl+shift =/-/0 zoom, on tea.Key
    wl_backend.go wl_window.go   Wayland: xdg toplevel, shm buffers, frame callback. Draw goes straight
    wl_input.go wl_keys.go       into the buffer (no copy). Seat, xkb keymap via unix.Mmap, key repeat
    wl_clipboard.go              wl_data_device selection
    fake.go                      Headless backend the core tests drive. No compositor
    backend_other.go             `!unix || android`: Run prints an error and returns 1.
                                 The APK does not call Run
    app.go RunSession            One terminal session on a Backend. Android's library uses it;
                                 Wayland keeps its own setup
    render.go canvas.go          Cells → premultiplied pixels, row damage against what each buffer last
                                 showed. Canvas order follows PixelFormat. Only the default background
                                 takes `opacity` (default 0.55; ARGB8888 buffers unless it is 1)
    font.go                      fc-match when it is on PATH; otherwise `$PREFIX/share/fonts` (Termux)
                                 and `/system/fonts`, then the embedded Go Mono
    boxdraw.go                   Procedural box drawing, arcs, blocks, braille so borders join seamlessly
    palette.go                   gruvbox default, gui.yaml overrides, 256-colour cube
  gui/android/                   ANativeWindow backend. Pure Go geometry, input, damage blit
                                 and the session manager build on every OS (Linux tests and
                                 the emulator job). `window_android.go` is `android && cgo`:
                                 `ANativeWindow_setBuffersGeometry(..., WINDOW_FORMAT_RGBA_8888)`,
                                 lock with the damage rect, copy rows at `buffer.stride`,
                                 unlockAndPost. Density is the core's buffer scale. Insets
                                 shrink the grid. Sessions survive surface loss
  cmd/misagui/                   `libmisagui.so` (`-buildmode=c-shared`, `android && cgo`).
                                 `JNI_OnLoad` in `jni_onload.c` calls `RegisterNatives` on
                                 `app/misa/Native` (`MISA_JNI_CLASS` overrides it). A4 loads
                                 the library from that class's static initializer. API 26
                                 clang, `-tags nough` (the agent runs in the child).
                                 `just android-lib`
  android/                       APK shell (`app.misa`), pure Java, no AndroidX. `MisaActivity`
                                 (SurfaceView, InputConnection, extra-keys row with a latched
                                 Ctrl/Alt that applies to IME text, a MUX row of one-tap
                                 multiplexer buttons, insets, pinch),
                                 `SessionService` (foreground `specialUse` notification, Stop),
                                 `Bootstrap` (D10 env, `filesDir/bin` symlinks to `libmisa.so`,
                                 first-run config). `Native` loads `libmisagui` from its static
                                 initializer. `build-go.sh`, `version.sh`, `just apk` / `apk-release`
  mweb/                          `misa --web` / `--web-admin`: webui.Server whose terminal runs misa (MISA_WEB_CHILD stops
                                 recursion); flags over config.yaml `web:`; XDG state/config dirs. internal/ui never imports webui
  ughbot/sandbox/                Code-task tool protocol: Serve (`ughbot vm-agent` in the VM), Client (tools.Executor +
                                 worktree.GitFunc), Local (in-process, tests)
  ughbot/vm/                     Throwaway qemu VMs: Prepare (cloud image + NoCloud seed over loopback HTTP, pinned
                                 keys, host uid), Start (overlay, 9p shares at the same paths, helper copied in), Cleanup
  webadmin/                      Device/session admin CLI shared by `ughbot devices` and `misa --web-admin` (webui.Admin, QR, enroll), plus the webdevtui adapter
  webdevtui/                     Bubble Tea device management screen (devices, sessions, events, notices) over its own Admin interface;
                                 used by the ughbot TUI Devices screen and the `devices` command (mweb, !noweb)
  wayland/                       Minimal Wayland wire-protocol client: socket, fd passing, object ids, registry, shm
  xkb/                           Text keymap parser (keycodes, types, symbols, modifier_map) + embedded keysym table
  justfile/ gradle/ makefile/    Build tool detection and task/recipe listing
  editor/ fileops/               File editing and operations
```

## Key Patterns

- **Screen-based navigation**: `screen` enum selects which update/view runs. Each screen is a self-contained update+view pair.
- **Menu stack**: `pushMenu()`/`popMenu()` for nested menus. `pushFilterableMenu()` enables bubbles list filtering.
- **Mux architecture**: Outer non-nested Model uses `updateMuxRoot`/`muxView()`. Nested Misa panes use `nestedMode=true`. `forceFrame()` renders lines at `w-1` characters to prevent terminal auto-wrap.
- **Data-driven package managers**: `pkgmanager.Manager` defines detection, actions (run/shell_prefill/list). Built-in pacman and flatpak; Termux `pkg` when `TERMUX_VERSION` or a `com.termux` `PREFIX` is set; external YAML in `~/.config/misa/managers/` or next to binary. `MISA_ANDROID_APP=1` drops managers with `Distro: true`.
- **Android build tags**: `GOOS=android` satisfies `linux`. Android-only files are tagged `android`. `import "C"` only with `android && cgo` (the same constraint covers `.c` files that include `jni.h` or `native_window.h`). `CGO_ENABLED=0` everywhere else. NDK version is `android/ndk.version`; API level for the CLI is 24. `libmisagui.so` is built with the API 26 clang (`just android-lib`).
- **External asset loading**: Distro art from `~/.config/misa/ascii/distro/`, anime only from `~/.config/misa/ascii/anime/` or `ascii/anime/` next to the binary (none is bundled; no art means an empty right column), themes from `presets.json`, all also searched next to binary.
- **Picker screen**: Reusable searchable toggle-list with preview. Used by settings, onboarding. `finishPicker()` checks `m.picker == nil` before overwriting screen to handle chained pickers.
- **Config search order**: next to executable → `~/.config/misa/`. Same for companion, mux, fetch and gui configs. Never the working directory: a cloned repo must not be able to supply `autorun:` or aliases.
- **Config `*bool` pattern**: `BoolDefault()`/`BoolPtr()` helpers for settings that default to true.
- **Configurable colors**: All colors configurable via `colors:` (info items) and `ui:` (prompt, divider, ghost, etc.) in config.yaml. Defaults are gruvbox.
- **Builtins**: Registered via `init()` with `Register(Command{...})`. Each uses the `Spec`/`ParseArgs` option parser. Context provides Dir, Args, Width, Height, Stdout/Stderr/Stdin. When adding or changing a builtin, update `internal/inventory/commands.json`, regenerate the human documentation with `go run ./internal/inventory/gen`, and verify the inventory reflects the registry.
- **History format**: Zsh-style (`: timestamp:0;command`). Import from bash/zsh is read-only — never writes to external history files. Entries without timestamps (bash imports) sort to the front.
- **Companion setup**: Moods, pronouns, call terms, prefixes, items/parts, emotes, kemonos, modifiers. Thirsty unlocks NSFW `{part}` (original mommy: milk, cock, cum, mouth); otherwise `{part}` uses SFW items. Modifiers can add combo lines when other modifiers are also selected. Texts load from embedded yaml under `internal/companion/data/`.
- **Root view**: `Model.View()` returns a `tea.View` (alt screen, `MouseModeAllMotion`) only for the root; panes compose through `render() string`.
- **Mouse support**: Enabled via the root view's `MouseMode` (hover events included). The root converts Bubble Tea's mouse messages once (`fromTea`) into `mouseMsg` (Mouse + `Action`: press/release/motion), which handlers and pane coordinate translation use; `toTea` turns it back for embedded agents. Helpers in `mouse.go`. Click to select/activate list items, scroll wheel on lists/viewports, click mux bar tags to switch, click panes to focus. Hovering focuses the pane under the pointer unless `focus_follows_mouse: false` in mux.yaml. Bare hovers that don't move focus reuse the last root frame (`muxFrame`), so mouse handlers must not change the view on button-less motion. Each screen handler has a `mouseMsg` case. Mux root translates absolute coords to pane-relative coords for nested Misa panes.
- **Shell aliases**: Defined in config.yaml `aliases:` map or at runtime via `alias name=value`. Expanded in `expandAlias()` before command dispatch. `unalias name` removes.
- **Autorun**: Config `autorun:` list of commands executed on startup in `Init()`.
- **Command dispatch**: `planCommand()` runs plain commands without `/bin/sh`. Builtins re-exec the binary as a multicall (`misa <builtin> args`) by default; `builtin_mode: inprocess` in config.yaml runs them on a goroutine attached to a PTY (busybox-style, for systems without a shell). Only input with shell syntax (`needsShell`) goes through `sh -c`.
- **Job control**: Each command runs in its own PTY session. External programs run under `misa --job-leader` (config `job_control`, default on) so ^Z stops them; multicall builtins stop themselves on SIGTSTP. A stopped foreground terminal becomes a `job` and the prompt returns; `jobs`/`fg`/`bg` are handled in `handleBuiltinCmd`. Job terminals keep their single `waitTermUpdate` subscription, so never start a second one on `fg`. Panes and Misa's exit close jobs (`closeTerminals`/`Shutdown`), since a stopped process ignores the hangup.
- **Pane message routing**: Commands from a nested pane are wrapped with `routeTo(owner, cmd)` so their results (`paneMsg`) update the pane that started them, not the focused one. Bubble Tea's own messages (clipboard, colour queries, quit) pass through unwrapped so the program carries them out.
- **Keys in v2**: `msg.String()` names space `"space"`, so `case "…"` matches use that; text inputs that build strings from keys take `msg.Text` (see `typedKey` in vi and procmon). Pastes arrive as `tea.PasteMsg` and go to the focused PTY (`sendPaste`, bracketed when asked) or text input.
- **Embedded agent (ugh)**: typing `ugh [-m … -thinking … -mode … -resume]` takes the pane over with an in-process `ughtui` session (`m.agent`); subcommands and `-p` run the CLI binary. Input goes to the session, its view fills the pane, `/quit` returns to the prompt. A menu `cmd: ugh …` item does the same through `menuCommand` and returns to the menu (`fromMenu`). `Host.Send` fills the session's `agentHost` mailbox, drained by a re-arming command, so mail and command results (`agentMail`/`agentResult`) come back through `routeTo` to the owning pane; messages for an ended session are dropped. Colour replies are broadcast to every session. Session state lives in `agentSession` (agent, mailbox, size, `split`). `ctrl+w a` starts a *split* session: `/quit` closes its pane, while closing the pane detaches the session to the root's `detached` list, where it keeps running and shows in the mux bar; `ctrl+w A` reattaches the latest in a new split. The root routes agent messages by session, not by pane (`agentMsgOf`/`routeAgentMsg`), so they follow a session that moved. Sessions typed into a pane end with it (`closeTerminals`); `Shutdown` also closes detached ones.
- **Command correction (thefuck)**: `internal/thefuck` package with pattern-based rules. After a failed shell command, `thefuck.Match()` suggests a fix. Shown in output; `ctrl+f` accepts and runs the fix.
- **Fetch builtin**: `fetch` command renders ASCII art + system info side by side (mfetch-style). Supports `--info`, `--theme`, `--ascii`, `-e` (emotes/nerd font), `--list-info`, `--list-ascii`, `--list-themes` flags. Configured via `fetch.yaml` (info, emotes, theme, color_mode, ascii, max_height); CLI flags override.
- **SSH client (`sshc`)**: `sshc [-tT] [-F cfg] [-i id] [-l user] [-o Key=Value] [-p port] destination [command]`. Understands HostName, User, Port, IdentityFile, StrictHostKeyChecking and UserKnownHostsFile from ssh_config; everything else (ProxyJump, forwarding, ControlMaster) is ignored. A PTY is requested for a shell, or with `-t`, only when stdin is a terminal. Tests run against an in-process x/crypto server (`client_test.go`).
- **Wayland window (`misa --gui`)**: A separate frontend process, not a Bubble Tea renderer: it starts the misa binary
  (or the given command) in a `term.Terminal` and paints `Snapshot()` cells itself, so Misa runs unchanged. The
  OS-neutral core (`internal/gui`) owns that session; the Wayland backend (`wl_*.go`, `unix && !android`) is the
  only platform window so far. Protocol objects are raw opcodes against `internal/wayland`, from wayland.xml/xdg-shell.xml
  (no generated bindings). The loop draws only on terminal dirty + frame callback; damage is computed per row.
  Keys go keymap → keysym (`internal/xkb`, no dead keys/compose) → `tea.Key` → `ui.KeyBytes`, so the xterm encoding
  lives in one place. The core draws premultiplied pixels straight into the buffer in the backend's order
  (BGRA on Wayland, RGBA on Android). Requires a compositor with
  xdg_wm_base; decorations only where zxdg_decoration_manager_v1 exists (not GNOME). On Android (and any target
  without a backend) `misa --gui` prints an error and exits 1. The APK's window is `libmisagui.so`
  (`cmd/misagui`, JNI class `app/misa/Native`): `internal/gui/android` presents through ANativeWindow and
  keeps one PTY session per tab across surface loss. Settings in `gui.yaml`
  (font, font_size, padding, columns, rows, foreground, background, cursor, palette, opacity, clipboard_write). `misa --gui cmd…` runs
  cmd; a Misa builtin missing from PATH (sshc) runs through the multicall binary. `misa --ssh [sshc opts] dest` is the
  ssh-client launcher. A program exiting non-zero leaves its screen up with a note until a key is pressed.
- **Chess variants**: Every rule lives in variant YAML; the engine has no orthodox-chess special cases. Pieces are
  Betza strings (`fmWfcFifmW2` pawn, `qN` rose, `mRcpR` cannon); `royal` pieces may never be left attacked (so with
  two kings, mating either wins), `pawn` pieces promote and take/are taken en passant. A pawn's long first step marks
  every crossed square for en passant. Castling entries are White's four squares, mirrored for Black. A file with
  `extends: <id>` inherits unset fields and merges pieces by letter; `castling: []` drops inherited castling. A later
  file with an existing id replaces it. Built-ins: chess, double (Double Chess 16×12), bigboard (Betza's 16×16).
  Movement is checked by perft (`position_test.go`), so run the chess tests after touching move generation.
  The engine only uses what variants define (values, royal/pawn flags, ranks to promotion), so new pieces need at
  most a `value:`. Search runs off the UI goroutine on immutable Positions; cancel() sets the Stop flag and bumps
  `gen`. Undo takes back plies until a human is to move, and refuses when only computer moves remain.
  `--level beginner|easy|normal|hard|expert` and `d` during play select one level for both computer sides,
  retained across new games, variant changes and hotseat. Weak searches use fresh tables and a per-search
  seed; all recursive nodes, including quiescence, count toward a strict node cap. Profile budgets and
  partial calibration limitations are documented in `docs/CHESS.md` and `docs/CHESS_CALIBRATION.md`.
- **Split config files**: Main `config.yaml` (menu, ascii, colors, ui, statusbar, aliases, autorun, builtin_mode, job_control). Mux settings in `mux.yaml` (auto_pane, bar_auto_hide, bar_position, bar_info, focus_follows_mouse). Saving them from a pane sends `muxConfigChanged` so the root applies them. Fetch settings in `fetch.yaml` (info, emotes, theme, color_mode, ascii, max_height). Window frontend settings in `gui.yaml`. All use same search order (`findConfigFile`).

## Conventions

From `docs/GUIDELINES.md`:

1. Write self-documenting code. Minimal comments — refactor instead of explaining.
2. Test-driven development. Write tests before implementations for testable logic.
3. Keep files small and decoupled. Self-contained, minimally interdependent modules.
4. Balance brevity with performance. Favor doing more with less code unless it hurts performance.

Project rules:

- All styling via lipgloss — no raw ANSI escape codes in UI code.
- All assets embedded via `//go:embed`. No `//go:generate`.
- Tests in `_test.go` files in the same package.
- Companion has its own config file (`companion.yaml`), separate from main config.
- History stored at `~/.local/share/misa/history` in zsh format (`: timestamp:0;command`).
- Config search order: next to executable → `~/.config/misa/` (never the working directory).
- Never write to external shell history files (bash, zsh). Read-only imports.
- Builtins register via `init()` with `Register(Command{...})` and use the `Spec`/`ParseArgs` option parser.
- Plans stay plans: never open a GitHub issue or a pull request for a plan alone. Keep plans in
  `docs/plans/` locally until there is actual code to implement, then open the pull request with that code.
- Don't open a GitHub issue just to track a pull request. If the work warrants a pull request, open it directly.

ugh (`internal/ugh/*`, `cmd/ugh`):

- Every package starts with a `// Package …` comment stating its role; keep it accurate when a package's
  responsibility changes.
- Regression tests are named `*_regression_test.go`. Provider clients are tested against recorded SSE fixtures in
  `testdata/`.
- Secrets never appear in errors, logs or display output (see `auth.MaskKey`, `perm` scrubbing).
- `internal/ugh/perm/static` must not import `os/exec` or `syscall` (enforced by a test).

## Attribution

When adding code derived from external sources:
- Add a comment at the top of the file noting the source and license.
- Add an entry to `docs/CREDITS.md`.
- For substantial derivations, add or update a `NOTICE` file in the package directory.

Commit trailers for agent-made commits:
- Add `Co-authored-by: Cursor <cursoragent@cursor.com>` to commits made through Grok Bot/Cursor.
- Add `Co-Authored-By: DeepSeek V4.1 Flash 1017 <noreply@deepseek.com>` only when DeepSeek actually worked on that
  commit. Never add it to commits written by other models or agents, or by hand.

## Docs

- `docs/GUIDELINES.md` — Coding guidelines (self-documenting code, TDD, small modules, brevity vs performance).
- `docs/CREDITS.md` — Third-party attribution for all derived/referenced code and assets.
- `docs/CODE_OF_CONDUCT.md` — Community code of conduct.
- `internal/companion/NOTICE` — Companion response template attribution (cargo-mommy, fwdekker/mommy).
- `internal/vi/NOTICE` — Vi editor attribution (toybox, vis).
- `THIRD_PARTY_NOTICES.md` — Licenses of third-party code, test suites and data.
- `docs/MISA_WEB.md` — `misa --web`: Misa in a browser (enroll, binding, security).
- `docs/UGH.md` — ugh: providers, running, TUI, tools, permissions, subagents, configuration.
- `docs/ughbot/DEPLOY.md`, `docs/ughbot/WEB.md` — ughbot (experimental): setup, bots, approvals, web UI.
- `docs/plans/` — Plans for upcoming work: `terminal.md`, `web.md`, `ugh-confinement.md`, `ughbot.md`,
  `builtins.md` (with `utility-comparison.md` and `builtins-coverage.md`), `windows.md`, `android.md`,
  `pkg-frontend.md`.
- `SECURITY.md`, `CONTRIBUTING.md` — Reporting vulnerabilities; how changes are made here.
