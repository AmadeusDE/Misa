# Plan: Windows port, GUI first

Status: draft, now tracked by #108, whose task IDs (F01–R02) supersede the W/UW numbering below where they differ.
The gui core/backend split (W2, shared with Android A2) has landed; the Win32 backend is not started. #108 F01
(package selection and the jobleader boundary) and F02 (metadata boundary) are delivered, see "F01 delivered" and "F02 delivered". Part 1 is Misa (phases W0–W6); part 2 is ugh (phases UW0–UW4), which
Misa needs before its Windows build can include the agent. The measurements below are from 2026-10-13; code
added since then (`internal/webui`'s file lock, `internal/ughbot`, `internal/ugh/confine`) needs the same
treatment. ughbot is out of scope for the Windows port (it is not redefined as Linux-only: it also builds for the BSDs and macOS).

# Part 1: Misa

## Goal

`misa --gui` opens a native Windows window that runs Misa (or any program) the way it does on Wayland.
After that, `misa.exe` runs as a console TUI and Misa's features (mux panes, builtins, sshc, fetch,
procmon, ugh) work on Windows 10 1809+ / Windows 11, amd64 and arm64.

Hard constraints:
- **No cgo.** Win32 goes through `golang.org/x/sys/windows`, plus `windows.NewLazySystemDLL(...).NewProc(...)`
  for user32/gdi32/dwmapi/shell32 calls that x/sys lacks. Callbacks (the window procedure) use
  `windows.NewCallback`. Cross-compiling with `GOOS=windows` from Linux has to keep working.
- No `//go:generate`. Assets go in via `//go:embed`. That rules out `.syso` resources from rsrc/go-winres:
  build the window icon at runtime (`CreateIconIndirect` from an embedded PNG) and set DPI awareness in code
  (`SetProcessDpiAwarenessContext`), not through a manifest.
- The Linux/BSD behaviour stays unchanged. Every Windows file is build-tagged.

## Where things stand (measured 2026-10-13)

`GOOS=windows go build` per package (x/sys v0.48.0 already has `CreatePseudoConsole`, `ResizePseudoConsole`,
`PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`, `NewProcThreadAttributeList`, job objects and `LockFileEx`):

| package | failure |
|---|---|
| `main`, `ui`, `gui` | (fixed by F01) `internal/jobleader` was `//go:build unix`, so the package was empty |
| `term` | `pollable`, `setWinsize` (ptyfd.go) and `waitProcess` (jobs.go) are unix-only. creack/pty compiles but can't start anything on Windows |
| `wayland` | `unix.UnixRights`, `ParseUnixRights`, `Mmap`: SCM_RIGHTS and shm are Unix by nature |
| `gui` (beyond ui) | `input.go` imports `x/sys/unix` (keymap mmap). Everything else is Wayland-bound or OS-neutral, see W2 |
| `lscore` | `syscall.Stat_t` (fileinfo.go:70, 221) |
| `sysinfo`, `procmon`, `builtins` | `statfs_unix.go` is tagged `!openbsd && !freebsd …`, so it includes Windows. `syscall.Statfs`, `/proc` |
| `procmon` | Unix signal table (htop.go:41–62), `syscall.Kill`, `Setpriority` |
| `sshclient` | `syscall.SIGWINCH` (resize.go:21) |
| `builtins` | masked by its dependencies. By grep: `Stat_t` ×24, `Umask`, `Mkfifo`, `Getgroups`, `Getpriority`, `unix.Uname`, `UtimesNanoAt`, `Access`, `Renameat`/`RENAME_EXCHANGE` |
| ugh (via `agent_ugh.go`) | see part 2; `-tags nough` avoids it |

Already portable: the VT engine (`term` parser/screen/snapshot), the gui renderer (`render.go`, `canvas.go`,
`boxdraw.go`, `palette.go`, sfnt font loading), `ui.KeyBytes`/`MouseBytes`/`PasteBytes`, config search
(`os.UserConfigDir` → `%AppData%\misa`), Bubble Tea v2 itself (console input via `charmbracelet/x/windows`), and
embedded Windows ASCII art (`internal/ascii/distro/*/Windows_11.txt` etc.).

## Decisions

- **D1. GUI first, with `-tags nough`.** The ugh port (part 2) is off the critical path. Windows
  builds use `nough` until UW2 lands there, then the default build includes ugh.
- **D2. ConPTY in `internal/term`.** Windows has no PTYs. `CreatePseudoConsole` plus `CreateProcessW` with
  `STARTUPINFOEX` is the only pure-Go way to give a child a console. Every session goes in a **job object**
  (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`). That replaces the session-wide SIGKILL in `session_linux.go`,
  since Windows has no sessions or process groups to signal.
- **D3. Win32 + GDI, not Direct2D/Direct3D.** The renderer already produces premultiplied ARGB8888, which is
  byte-for-byte the BGRA layout of a top-down 32bpp `CreateDIBSection`. GDI needs only flat C calls; D2D/DXGI
  are COM vtables, painful without cgo and with nothing to gain for a software canvas.
- **D4. Split `internal/gui` into an OS-neutral core and per-platform backends** (W2), on Linux first, with no
  change in behaviour. That way the Win32 backend is a sibling of the Wayland one, not a fork of the package.
- **D5. One binary, `misa.exe`, console subsystem.** It has to be a console image, because ConPTY children
  (Misa itself under the window, multicall builtins) only attach to a pseudoconsole if they are one. ConPTY
  sessions run `os.Executable()`. `misa --gui` is the window, as on Linux:
  - **Run from a terminal:** it keeps that console and blocks it until the window closes, the same as
    `misa --gui` in a Linux terminal.
  - **Run from Explorer, the Start menu or a shortcut:** Windows gives the process a fresh console.
    `GetConsoleProcessList` reporting only Misa itself means nobody else uses that console. In that case,
    hide it (`ShowWindow(GetConsoleWindow(), SW_HIDE)`) and `FreeConsole()` before creating the window.
    - Cost: a brief console flash (or a Windows Terminal window, where that's the default terminal app).
    - Mitigation now: a Start-menu shortcut to `misa.exe --gui` with "Run: Minimized", so the flash becomes a
      taskbar blip.
    - Mitigation later: newer Windows 11 builds have a manifest `consoleAllocationPolicy` of `detached` plus
      `AllocConsoleWithOptions`, which remove the console entirely. That needs a manifest resource (a
      checked-in `.syso`, not generated), so check which build it shipped in and decide then.
  - After `FreeConsole` there's no stderr, so GUI startup errors go to `MessageBoxW`.
  - The argv0 dispatch (`misa-gui`, `misa-ssh`) strips `.exe` and compares case-insensitively, so a copy or
    hardlink named `misa-gui.exe` still works. It's optional; nothing depends on it.
- **D6. No `/bin/sh` on Windows: Misa's own shell runs shell syntax, `misa --sh -c CMD`** (new
  `internal/msh`). This is the start of Misa's POSIX-shell and later bash replacement, not a stopgap wrapper.
  For now its engine is `mvdan.cc/sh/v3` (pure Go, already in the module graph through ugh; it parses and
  interprets bash-flavoured syntax too). An exec handler runs Misa builtins in-process and everything else
  through PATH/PATHEXT. The `--sh -c` entry point stays fixed while the engine grows into Misa's own.
  - `shellPlan` uses it when `sh` isn't on PATH, so minimal Unix systems benefit too.
  - It's a flag, like `--job-leader`, not a registered builtin, so it never shadows a real `sh`.
  - Until it's good enough to be the default, the interactive full-shell mode defaults to `pwsh.exe` →
    `powershell.exe` → `%ComSpec%`. A `shell:` key in config.yaml overrides both.
- **D7. No job control on Windows.** There's no SIGTSTP/SIGCONT: ^Z goes to the program as 0x1a, `jobs`/`fg`/`bg`
  print that they aren't supported, `job_control` is forced off, and `jobleader` gets a `!unix` stub (`Run`
  execs and waits; `SuspendOnCtrlZ` does nothing). F01 added `jobleader.Supported` (false there), and the
  UI forces `jobControl` off with it; the I06 UI work (`jobs`/`fg`/`bg` messages) builds on that constant.
- **D8. Fonts without fontconfig:** read the font registry (HKLM and HKCU
  `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`, via `x/sys/windows/registry`), then index family and
  style from each file's sfnt `name` table, lazily and cached. The default chain is the configured font →
  Cascadia Mono → Consolas → embedded Go Mono.

### Open question: is ConPTY good enough for Misa's own UI? (spike S0)

ConPTY is a console host, not a pipe. It re-renders the child's screen and historically drops or rewrites
sequences it doesn't understand. Misa uses several of those: SGR mouse modes, the kitty keyboard protocol, OSC
10/11 colour queries (`hostcolors.go`), OSC 52, DEC 2026 synchronized output, and focus events. Test this
before building on it (S0, right after W1): run `misa.exe` in a W1 ConPTY and log both directions.

If it degrades, the options in order are:
1. Load a newer `conpty.dll` + `OpenConsole.exe` (Microsoft Terminal releases, MIT) from next to the exe via
   `LoadLibrary`, falling back to kernel32's. This is pure Go and is what WezTerm does. Credit it in
   `docs/CREDITS.md`.
2. **Run Misa in-process inside the `misa --gui` process** over pipes: `tea.WithInput`/`WithOutput`, and
   `WindowSizeMsg` sent on resize through a new `ui.RunOn(r, w, rows, cols)`. Misa's own UI then bypasses
   ConPTY completely. Only programs Misa launches in panes go through ConPTY, as they do on Linux through PTYs.

## Phases

The milestones in order:
- **M-a:** `misa --gui cmd.exe`/`pwsh.exe` works, so the window is a usable terminal (W0–W3).
- **M-b:** `misa --gui` runs Misa usably (S0 + W4).
- **M-c:** Misa's subsystems work (W5).
- **M-d:** ugh is built in (W6).

### W0. Compile for Windows (stubs, build tags, CI gate)

Goal: `GOOS=windows go build -tags nough .` and `go vet` pass, and the Linux/OpenBSD builds are unchanged.
- `jobleader/jobleader_other.go` (`!unix`): `Flag`, `Run` (exec + wait + exit code), `SuspendOnCtrlZ` no-op. Done in F01.
- `term`: rename `ptyfd.go`/`jobs.go` to `pty_unix.go`/`jobs_unix.go`, tag `session_other.go`
  `unix && !linux` (done in F01), and add temporary `pty_windows.go` stubs that return an error (replaced in W1).
- `wayland` (every file, tests included, now `unix`; done in F01) and the Wayland files of `gui` (already `unix && !android`). `main_gui.go` keeps `!nogui`, and the gui package
  compiles on Windows with a stub `Run` that reports "not yet" until W3.
- `statfs_unix.go` ×3: retagged `unix && !(openbsd || freebsd || netbsd || darwin || dragonfly)` in F01. Add
  `statfs_windows.go` (`GetDiskFreeSpaceExW`).
- `lscore`: move the `Stat_t` reads behind `fileinfo_unix.go`/`fileinfo_windows.go` helpers
  (owner/group/nlink/inode/blocks). Windows returns attributes from `Win32FileAttributeData` and leaves owner
  blank for now.
- `procmon`: signal table and kill/renice into `signals_unix.go`. `signals_windows.go` offers Terminate
  only (`TerminateProcess`) and maps renice to `SetPriorityClass`. The process list stays unimplemented until W5.
- `sshclient/resize.go`: SIGWINCH part to `resize_unix.go`. Windows relies on the polling that already exists
  for in-process panes.
- `builtins`: after the above, fix what's left per file. Builtins that mean nothing on Windows (`chown`,
  `chgrp`, `mkfifo`, `id`, `groups`, `nice`, `logname`) move their `Register` into `_unix.go` files, so
  they don't exist there rather than failing. `uname` → `RtlGetVersion`; `touch` → `os.Chtimes`;
  same-file checks in cp/mv/ln → `GetFileInformationByHandle` file index instead of inode.
- Tests that spawn `sh`: `//go:build unix` or `t.Skip` on Windows.
- Add to the Build & Test section of `docs/AGENTS.md`:
  `GOOS=windows go vet -tags nough ./...` and `GOOS=windows GOARCH=arm64 go build -tags nough .`, run before
  every commit from now on, so Unix-only code can't creep back in.

### W1. ConPTY sessions in `internal/term`

- Refactor `Terminal.ptm *os.File` / `cmd *exec.Cmd` into a small platform interface. Unix behaviour stays
  exactly as it is:
  ```go
  type ptyConn interface { io.Reader; io.Writer; Close() error; Resize(rows, cols int) error }
  ```
  plus a `proc` with `Wait() int`, `Kill()`, `Pid()`. `StartReader`, `drainInput`, `Resize` and `Close` go
  through these.
- `pty_windows.go`, `StartArgv`:
  1. two `CreatePipe` pairs and `CreatePseudoConsole(size, inRead, outWrite, 0)`
  2. `NewProcThreadAttributeList(1)` + `Update(PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, hpc)`
  3. `CreateProcessW` with `EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT | CREATE_SUSPENDED`, the
     command line built with `windows.ComposeCommandLine`, argv[0] resolved by `exec.LookPath` (PATHEXT), and
     `TERM=xterm-256color`/`COLORTERM=truecolor` in the environment block
  4. `AssignProcessToJobObject`, then `ResumeThread`. Assigning before resuming means no grandchild can
     escape the job.
  5. close the ConPTY-side pipe ends in the parent.
- **Exit detection differs from Unix:** the output pipe does not break when the child exits, only when
  `ClosePseudoConsole` runs. So a goroutine waits on the process handle, records the exit code, then calls
  `ClosePseudoConsole`. That flushes the last output, the reader sees EOF, and `Finish`/`reap`/`exited` run in
  the existing order. `ClosePseudoConsole` can block until output is drained on older builds, so the reader
  must be running before it's called (it always is after `StartReader`).
- `Close` → `TerminateJobObject` + `ClosePseudoConsole`. `Resize` → `ResizePseudoConsole`.
- `jobs_windows.go`: `Stopped()` is always false; `Continue()` returns an "unsupported" error.
- `StartFunc` (in-process builtins): there's no tty side to hand out. Use a plain pipe pair, so builtins see a
  non-tty. That's acceptable because multicall (`misa.exe ls …` under ConPTY) is the default `builtin_mode`;
  document that `inprocess` mode is degraded on Windows.
- Tests (`//go:build windows`): `cmd /c echo hi` output, exit codes, resize, and that `Close` kills a
  grandchild (`cmd /c start /b ping -t …`).
- **S0 spike** (see above): log what ConPTY passes through for Misa's sequences and pick a route before W4.

### W2. Split `internal/gui` into core + backends (on Linux, no behaviour change)

Today the shared logic is interleaved with Wayland objects in `gui.go`, `input.go`, `keys.go` and
`clipboard.go`. Target layout:

| core (OS-neutral) | Wayland backend (`unix`) | Win32 backend (`windows`) |
|---|---|---|
| `app.go`: loop, hold, OSC queue, sizing, zoom (from gui.go) | `wl_window.go` (window.go) | `win_window.go` |
| `pointer.go`: position → cell, buttons, drag, wheel/value120 → `ui.MouseBytes` (from input.go) | `wl_input.go`: seat, xkb keymap, repeat timer | `win_input.go`: WM_KEY*/WM_CHAR/mouse |
| `shortcuts.go`: ctrl+shift+v / zoom on `tea.Key` instead of keysyms (from keys.go) | `wl_keys.go`: keysym → `tea.Key` | `win_keys.go`: VK → `tea.Key` |
| `render.go` `canvas.go` `boxdraw.go` `palette.go` | `wl_clipboard.go` | `win_clipboard.go` |
| `font.go` (sfnt, sizing, fallback cache) | `fontfind_fc.go` (fc-match) | `fontfind_windows.go` (registry) |

Backend contract:
```go
type backend interface {
	ready() <-chan struct{}      // something to dispatch
	dispatch(h host)             // runs on the app goroutine; calls h.key/h.mouse/h.resize/h.focus/h.close/h.paste
	canvas(pw, ph int) *surface  // a buffer to draw into (nil: none free yet)
	present(s *surface, damage []image.Rectangle)
	canPresent() bool            // Wayland: no frame callback pending; Win32: frame pacing
	setTitle(string); copy(string); paste(); scale() float64
}
```
The app keeps its single goroutine. Wayland maps `Ready` to a queue fed from `conn.Events()` and
`Dispatch` to `conn.Dispatch`, which calls the host before returning, so a configure still draws
before the next protocol message. Win32 feeds `Events` from its window thread (W3).

Android A2 landed this split. The contract the core uses is `internal/gui/backend.go` (exported, so
an `internal/gui/android` subpackage can implement it): `Size() (w, h int, scale float64)`,
`Present(pixels, stride, PixelFormat, damage)`, `SetTitle`, `Events`, and `Clipboard`, plus
`Ready`/`Dispatch`/`CanPresent`/`Buffer` so Wayland keeps its single goroutine and its shm buffers.
`PixelFormat` is `FormatBGRA8888` (Wayland ARGB8888, Win32 DIB) or `FormatRGBA8888` (Android
`WINDOW_FORMAT_RGBA_8888`). The core draws in the backend's order, so the hot path does not swizzle;
`Present` swizzles only a foreign buffer. Wayland files are `wl_*.go` with build tag `unix && !android`
because `GOOS=android` matches both `linux` and `unix`. `font.go` still calls `fc-match` when it is on
PATH; the non-fontconfig path searches `$PREFIX/share/fonts` and `/system/fonts`, then Go Mono.
`fontfind_windows.go` stays with W3. Scale is `float64`. Wayland's buffer scale is still an integer.

- **Scale becomes `float64`.** Windows DPI is fractional (125%, 150%). Fonts rasterize at `pt × scale`, padding
  scales the same way, and the core works in pixels with logical size = pixels / scale. Wayland still sets an
  integer `wl_surface.set_buffer_scale`; `wp_fractional_scale_v1` can use the float later.
- Verify: the existing gui tests plus a manual `misa --gui` session on Wayland behave exactly as before.

### W3. Win32 backend (`misa --gui cmd.exe` works: milestone M-a)

- **Threads:** the window thread calls `runtime.LockOSThread`, registers the class, creates the window and runs
  `GetMessageW`/`TranslateMessage`/`DispatchMessageW`. The WndProc (one `windows.NewCallback`, created once)
  turns messages into events on a channel and wakes `ready()`. The app goroutine renders. Because the modal
  resize loop only blocks the window thread, output keeps drawing during a drag-resize.
- **Present:** one top-down 32bpp DIB section, shared under a mutex. The app draws damaged rows and calls
  `InvalidateRect` per damage rect. `WM_PAINT` does `BitBlt` from a memory DC under the same mutex.
  `WM_ERASEBKGND` returns 1 (no flicker). Paced with `term.FrameDelay` at the monitor's refresh rate
  (`DwmGetCompositionTimingInfo`, default 60 Hz). Recreate the DIB on resize.
- **DPI:** `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)` at start, initial scale from
  `GetDpiForWindow`/96, `WM_DPICHANGED` → `scale` + `SetWindowPos` to the suggested rect. Use
  `AdjustWindowRectExForDpi` to size the window for `columns`×`rows`.
- **Keyboard:** `WM_KEYDOWN`/`WM_SYSKEYDOWN` → named keys (VK table: arrows, Home/End, PgUp/PgDn, Ins/Del,
  F1–F24, Enter/Tab/Backspace/Esc, keypad) and ctrl/alt combinations. The base rune for those comes from
  `ToUnicodeEx` with an empty key state, which gives the "unshifted Latin key" rule `teaKey` already applies on
  Wayland. `WM_CHAR`/`WM_SYSCHAR` → text: `TranslateMessage` handles dead keys, AltGr and IME commits (Wayland
  has no compose today), and UTF-16 surrogate pairs are joined. Rules:
  - a key that will produce `WM_CHAR` is not also sent from `WM_KEYDOWN`
  - AltGr arrives as Ctrl+Alt: a right Alt with a synthesized left Ctrl is text, not a ctrl combo
  - Alt+key must not reach `DefWindowProc` (it would beep or open the system menu), except Alt+F4
  - F10 arrives as `WM_SYSKEYDOWN`
  - key repeat is the OS's (the repeat bit), so there's no repeat timer
- **Mouse:** `WM_MOUSEMOVE`, L/M/R/X button down/up (`SetCapture` while held), `TrackMouseEvent` for leave.
  `WM_MOUSEWHEEL`/`WM_MOUSEHWHEEL` deltas are in 120ths, so they feed the core's existing value120 path.
  `WM_SETCURSOR` → I-beam.
- **Clipboard:** `CF_UNICODETEXT` read/write (`OpenClipboard`, `GlobalAlloc`/`GlobalLock`, UTF-16), CRLF →
  CR on paste the way other terminals do, and OSC 52 copy through `copy()`.
- **Window:** `WS_OVERLAPPEDWINDOW`, title from OSC 0/2 (`SetWindowTextW`), dark title bar
  (`DwmSetWindowAttribute(DWMWA_USE_IMMERSIVE_DARK_MODE)`) when the background is dark, icon built at runtime from
  embedded art. Hold-on-failure works unchanged. Startup errors go to `MessageBoxW` once the console is freed (D5).
- **Opacity (second pass):** opaque first. For `opacity < 1`, `DwmExtendFrameIntoClientArea` with margins −1
  makes DWM use the client alpha of the premultiplied blit; optionally `DWMWA_SYSTEMBACKDROP_TYPE` (Mica/Acrylic,
  Windows 11 22H2+). Check on real hardware; skip on Windows 10 if it misbehaves. Not `UpdateLayeredWindow`:
  it loses the system frame.
- **Fonts:** `fontfind_windows.go` (D8). Per-rune fallback walks Segoe UI Symbol, Segoe UI Emoji (monochrome
  outlines; sfnt can't draw COLR), Cascadia Mono/Code NF if installed, MS Gothic / Yu Gothic, Malgun Gothic,
  Microsoft YaHei, Nirmala UI and Segoe UI Historic. `.ttc` indexes already work (`parseFontFile`).
- `gui.yaml` keys stay the same and are found in `%AppData%\misa\gui.yaml`.
- Unit tests (any OS, pure functions): VK → `tea.Key` table, AltGr/dead-key decision, UTF-16 clipboard
  round-trip, DIB pixel layout = canvas layout, font registry value parsing (`"Cascadia Mono (TrueType)"` →
  family).

### W4. Misa itself under the GUI (milestone M-b)

- `main.go`: argv0 dispatch normalizes `misa-gui.exe` → `misa-gui`. `startGUI` frees an Explorer-allocated
  console (D5). `guiCommand` and `BuiltinExecutable` use `os.Executable()` as they do on Linux.
- Build: `go build -tags nough -o misa.exe .`, documented in `docs/AGENTS.md`, plus a note on the "Run: Minimized"
  Start-menu shortcut for `misa.exe --gui`.
- **Dispatch (`dispatch.go`, `shell.go`):** `needsShell` treats `\` as shell syntax, so every `C:\…` path
  would go to the shell. On Windows `\` is a path separator: drop it from the set and check `fields[0]` for
  `\` and drive letters as well as `/`. `shellPlan` → `[misa.exe --sh -c cmd]` (D6). `agent.go:217`
  `m.launch("sh","-c",…)` goes through `shellPlan` too.
- `internal/msh` (D6): the interp runner, with Misa builtins in its exec handler. Tests run on every OS.
- Job control off (D7). `handleBuiltinCmd` jobs/fg/bg print a message. Ctrl+C reaches the child as
  `CTRL_C_EVENT` through ConPTY.
- Environment: `USER` → `USERNAME` fallback (statusbar.go:164, sysinfo). `HOME` → `os.UserHomeDir`.
- History: the zsh-format file stays under `%AppData%\misa`. Import PSReadLine history
  (`%AppData%\Microsoft\Windows\PowerShell\PSReadLine\ConsoleHost_history.txt`) read-only, like bash/zsh.
- File browser: drive roots (`GetLogicalDrives`), hidden/system attributes, open with `ShellExecuteW`.
- Package manager menus: built-in `winget` definition, plus `scoop`/`choco` as external YAML examples
  (data-driven, `internal/pkgmanager`).
- Help: tldr's `windows` platform pages. No man pages: fall back to `--help` output.
- Run S0's chosen route (newer ConPTY or in-process `ui.RunOn`) here if needed.

### W5. Remaining subsystems (milestone M-c)

- **sysinfo:**
  - OS from the registry (`CurrentVersion`: ProductName, DisplayVersion, CurrentBuildNumber, UBR). Build ≥
    22000 is Windows 11 even though ProductName says 10. That also picks the distro art.
  - CPU from `HARDWARE\DESCRIPTION\System\CentralProcessor\0`.
  - memory `GlobalMemoryStatusEx`, uptime `GetTickCount64`, disks `GetLogicalDrives` +
    `GetDiskFreeSpaceExW`, GPU via `EnumDisplayDevicesW`, shell from the parent process
    (`CreateToolhelp32Snapshot`).
- **procmon:**
  - process list `CreateToolhelp32Snapshot`; CPU% from `GetProcessTimes` deltas over `GetSystemTimes`;
    memory `K32GetProcessMemoryInfo`
  - kill `TerminateProcess`, nice `SetPriorityClass`
  - the htop/btop/top skins only show what's available (no signals menu beyond Terminate)
- **lscore:**
  - mode column synthesized from attributes (d, readonly, hidden, system, reparse point)
  - executable by PATHEXT extension
  - owner via `GetNamedSecurityInfo` + `LookupAccountSid` (cached, off by default if slow on network drives)
  - git status unchanged
- **sshc:**
  - raw mode through `charmbracelet/x/term` (which handles the Windows console)
  - resize polling only
  - agent over the Windows OpenSSH named pipe `\\.\pipe\openssh-ssh-agent`: `CreateFile` → `os.NewFile`;
    `agent.NewClient` takes an `io.ReadWriter`, so no go-winio
  - config/known_hosts under `%USERPROFILE%\.ssh`, system config `%ProgramData%\ssh\ssh_config`
  - Pageant: not initially
- **builtins:** go through `tests/` results on Windows and decide per command: fix, keep with a documented
  difference, or don't register.

### W6. ugh (milestone M-d)

Once part 2's UW2 lands: drop `nough` from the Windows build instructions and run `ugh` and `ctrl+w a`
in the GUI. The embedded bash tool uses ugh's Windows shell choice (Git Bash, else mvdan interp). Misa can
pass its builtins to ugh's interp through the `ExecHandler` hook from UW3, so the agent has `ls`/`cat`/`grep`
without Git for Windows.

## Testing

- Every commit: the W0 cross-vet/build gates, plus `GOOS=windows go test -c ./...` per package (compiles the
  tests).
- Pure logic (key tables, UTF-16, font registry parsing, msh, dispatch planning with Windows paths) is tested
  on Linux behind build-tag-free helpers.
- Real runs need a Windows 11 machine or VM. ConPTY, DPI, IME and
  clipboard tests are `//go:build windows` and run there. Wine's ConPTY and DWM support is too partial to
  trust for W1/W3.

## Risks

- **ConPTY fidelity** (S0): the biggest unknown; mitigations above.
- **Double ConPTY** (GUI → Misa → pane programs): extra latency and CPU. The in-process route removes the outer
  layer.
- **Unsigned Go binaries** trip SmartScreen and some antivirus heuristics. Signing is a release concern, not
  a build one.
- **Colour emoji:** sfnt can't rasterize COLR/CBDT, so emoji are monochrome, as on Wayland today.
- **Process-tree semantics:** job objects kill everything a command started, which is stricter than Unix
  sessions (a `start`ed GUI app dies with its pane). Allow breakaway (`JOB_OBJECT_LIMIT_BREAKAWAY_OK`) only
  if that turns out to annoy.

# Part 2: ugh

## Goal

`ugh` builds and runs on windows/amd64 and windows/arm64, both standalone (`ugh.exe` in Windows Terminal or
conhost) and embedded in Misa through `ughtui`. Unix behaviour is unchanged.

Constraints, the same as part 1:
- **no cgo:** `golang.org/x/sys/windows` and lazy DLL procs only
- cross-compiling with `GOOS=windows` from Linux keeps working
- every Windows file is build-tagged

## Where things stand (2026-10-13, `GOOS=windows go build -gcflags=-e` per package; file names are under `internal/ugh/`)

| package | failure |
|---|---|
| `internal/ugh/admission` | `lock.go`, `manager.go`, `waiter.go`: raw `int` fds through `syscall.Open/Flock/Ftruncate/Seek/Write/Close` (Windows fds are `syscall.Handle`) |
| `internal/ugh/policy` | `migration.go:223`: `syscall.Flock` |
| `internal/ugh/auth` | `codex.go:469`: `Flock`; `file.go:71`: `syscall.Stat_t` uid ownership check; `file.go:106`: `O_NOFOLLOW`/`O_NONBLOCK` open |
| `internal/ugh/worktree` | `lock.go`: `Flock` on int fds; `process.go`: `syscall.Kill(pid, 0)` / `Kill(-pgid, 0)` liveness, `/proc` start times |
| `internal/ugh/tools` | `bash.go` is `//go:build unix`, so `bashTool` is undefined (`tools.go:35`) |
| `cmd/ugh` | blocked by the above; `login.go:100`: `SysProcAttr{Setsid: true}` |
| `internal/ugh/ughtui`, `internal/ugh/tui`, `internal/ugh/agent`, `internal/ugh/provider` | only through the packages above |

Builds but is wrong on Windows:
- **`HOME` read directly:** `config.go:419`, `auth/file.go:40`, `auth/importpi.go:65`, `perm/log.go:42`,
  `perm/rules.go:68,290,296`, `perm/static/static.go:174`, `session/session.go:54`, `usage/ledger.go:31`.
  `HOME` is usually unset on Windows, which makes these paths relative.
- **Path containment hard-coded to `/`:** `p == d || strings.HasPrefix(p, d+"/")` in `perm/rules.go:296`,
  `perm/static/static.go:137,142,222`, `perm/static/shell.go:948`, `perm/static/triage.go:116`, and the
  `/proc` cwd scan in `worktree/process.go`. These are permission and safety checks, so a miss is a security
  bug, not a cosmetic one.
- **`xdg-open`:** `cmd/ugh/login.go:73`, `loginkey.go:103`.
- **`auth.RuntimeDir`:** falls back to `os.TempDir()/ugh-<uid>`; `os.Getuid()` is −1 on Windows.

Already fine:
- `signal.NotifyContext(…, os.Interrupt, syscall.SIGTERM)`, since `syscall.SIGTERM` exists on Windows
- `exec.Command("git", …)` and `rg` lookups (PATHEXT)
- Bubble Tea v2 console I/O
- go-keyring, which has a wincred backend

## Decisions

- **UD1. One file-lock helper, `internal/ugh/flock`.** It has `Lock`, `TryLock` and `Unlock` on `*os.File`:
  `flock(2)` on Unix, `LockFileEx`/`UnlockFileEx` on Windows. All four call sites use it, and the raw-fd code
  moves to `*os.File` (`Truncate`, `Seek`, `Write`, `Close`), which is portable without per-OS files.
  - **Windows locks are mandatory byte-range locks.** Locking the file's contents would make other processes'
    reads of a slot's diagnostics JSON fail. Lock one byte far past EOF (offset `1<<62`, length 1) instead,
    which gives flock-like advisory behaviour.
  - `TryLock` maps `ERROR_LOCK_VIOLATION`/`ERROR_IO_PENDING` to the existing "locked" errors
    (`isLockedError`).
- **UD2. Private files: ownership and ACL instead of mode bits.** On Unix, `auth/file.go` checks 0700/0600
  and uid ownership. On Windows (`private_windows.go`):
  - check that the owner SID (`GetNamedSecurityInfo(OWNER_SECURITY_INFORMATION)`) is the current user's token
    SID
  - create the directory and file with a **protected DACL** that grants only the current user, which is the
    equivalent of 0700/0600
  - refuse reparse points (the `Lstat` mode check plus opening with `FILE_FLAG_OPEN_REPARSE_POINT` through
    `windows.CreateFile` → `os.NewFile`) instead of `O_NOFOLLOW`
- **UD3. Keyring size limit.** wincred caps a credential blob at 2560 bytes (`CRED_MAX_CREDENTIAL_BLOB_SIZE`);
  OAuth/Codex token documents can exceed it. Measure the real document size first. If it can exceed the limit,
  the Windows default becomes the `file` backend (made safe by UD2), or the keyring entry is split into chunks.
  Don't let a login fail at the end with "data passed to Set was too big".
- **UD4. The `bash` tool on Windows.** The tool keeps its name and its POSIX-sh contract, because the model's
  habits and `perm/static`'s mvdan-based analysis both assume POSIX shell. Shell choice:
  1. `UGH_SHELL` or a `tools.shell` config key, if set.
  2. Git for Windows `bash.exe`/`sh.exe`, found via PATH and `%ProgramFiles%\Git\bin`. **Skip
     `%SystemRoot%\System32\bash.exe`**: that's the WSL launcher, which runs the command in a different OS
     with different paths.
  3. Otherwise the in-process `mvdan.cc/sh/v3/interp` (already a dependency). That's POSIX sh without
     external coreutils, unless the host provides them through an exec handler (UW3: Misa passes its
     builtins).

  The tool description names the shell actually in use, so the model knows whether `ls`/`grep` exist.
  `$UGH_SCRATCH` is unchanged. The description's "/tmp" wording becomes "the system temp directory".
- **UD5. Process trees: job objects instead of process groups.** Replace `Setpgid` + `kill(-pgid)`:
  - the shell starts `CREATE_SUSPENDED`, is assigned to a fresh job and resumed (raw `CreateProcess`, or
    `exec.Cmd` + `SysProcAttr.CreationFlags` with assignment right after start; the latter has a small race,
    acceptable since the shell spawns nothing that fast)
  - cancel/timeout → `TerminateJobObject`
  - `groupAlive` → `QueryInformationJobObject(JobObjectBasicAccountingInformation).ActiveProcesses > 0`
  - no `KILL_ON_JOB_CLOSE`, so background jobs of a finished command survive, as they do on Unix
- **UD6. Worktree liveness without `/proc` or pgids.**
  - `isPIDAlive`: `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` + `GetExitCodeProcess` (`STILL_ACTIVE`, then
    a wait with timeout 0 to rule out a real exit code of 259)
  - start-time identity: `GetProcessTimes` creation time replaces `/proc/<pid>/stat` starttime
  - a job object doesn't outlive the ugh process that holds its handle, so a group can't be tracked across
    processes: when a command leaves processes running, record their pids
    (`JobObjectBasicProcessIdList`) in `ActivePIDs` with creation times, and leave `ActiveGroups` empty on
    Windows
  - the cwd scan is skipped, the same path as a Unix system without `/proc`
  - Detection is weaker than on Linux; document it in the worktree code.
- **UD7. Paths go through helpers, not string prefixes.**
  - `paths.Home()`: `os.UserHomeDir`
  - `paths.ConfigBase()`: an absolute `XDG_CONFIG_HOME`, else `os.UserConfigDir()` (`%AppData%`); Unix
    behaviour unchanged
  - `paths.StateBase()`: `XDG_STATE_HOME`, else `~/.local/state` on Unix, `%LocalAppData%` on Windows
  - `paths.RuntimeDir()`: `XDG_RUNTIME_DIR`, else `%LocalAppData%\ugh\run` (UD2-private) on Windows
  - `paths.Within(p, dir)`: `filepath.Clean` both, `filepath.Rel`, and on Windows case-insensitive with both
    separators accepted and drive letters and UNC prefixes compared. All the `d+"/"` checks listed above use it.
  - `~` expansion in `perm/rules.go:290` also accepts `~\`.
  - `perm/static`'s sensitive-path list gains the Windows equivalents: `%AppData%\ugh`, `~\.ssh`,
    `%AppData%\gcloud`, `~\.aws`, `%LocalAppData%\Microsoft\Credentials`.
- **UD8. Opening the browser:** `browser.Open(url)`: `xdg-open` on Unix, `open` on darwin,
  `ShellExecuteW(nil, "open", url, …)` on Windows (shell32 lazy proc; not `cmd /c start`, which needs quoting
  of `&` in URLs).
- **UD9. Detaching the login helper:** `login.go`'s `Setsid` moves to `detach_unix.go`. Windows uses
  `SysProcAttr.CreationFlags = CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS`.

## Phases

### UW0. Compile for Windows

- UD1 (`internal/ugh/flock`, fd → `*os.File`), UD9, and `bash_windows.go` as a stub that reports "no shell
  available on Windows yet" (replaced in UW2).
- UD2 for `auth/file.go`, enough to compile (`owned_windows.go` with the SID check; the ACL creation is UW1).
- Retag worktree `process.go` into `process_unix.go` + `process_windows.go` (UD6).
- Gate: `GOOS=windows go vet ./...` and `GOOS=windows GOARCH=arm64 go build ./cmd/ugh` pass. Add both to
  `docs/AGENTS.md`'s build section so Unix-only code can't creep back in.

### UW1. Correct paths, auth and permissions

- UD7 helpers, with every `HOME`/`d+"/"` call site moved to them. Tests table-driven on all OSes (Windows path
  cases run as pure string logic through a `windowsPaths` variant).
- UD2 ACL creation, UD3 measurement and default-backend decision, UD8.
- `auth.RuntimeDir` via `paths.RuntimeDir`.
- Verify: `ugh login`/`ugh auth list` on a Windows VM store and reload credentials, and a second user can't
  read them (`icacls` shows one ACE).

### UW2. Bash tool and process control (W6 depends on this)

- UD4 shell selection + UD5 job objects in `bash_windows.go`. `groupAlive`, timeout and cancel go through the
  job. Output capture/truncation (`bashCapture`, `trimStream`) is shared, moved out of the `unix` file.
- Tests (`//go:build windows`): timeout kills a grandchild (`sh -c "ping -n 100 127.0.0.1 & sleep 100"`), a
  background job of a finished command survives, `UGH_SCRATCH` is set, and selection skips the WSL launcher.
  The interp fallback is tested on every OS (it's pure Go).

### UW3. Embedding hooks for Misa

- `ughtui.Options.ExecHandler` (an `interp.ExecHandlerFunc` middleware), passed through to the bash tool's
  interp fallback, so Misa supplies `ls`/`cat`/`grep`/… from its builtins when Git for Windows is absent.
  It's ignored when a real shell is in use.
- Check that `ughtui` makes no Unix assumptions about the host's terminal (it gets sizes and colours from
  Misa).

### UW4. Windows polish

- `rg` absent: the search tool already falls back. Verify the fallback handles `\` paths and CRLF files.
- `git` worktrees on Windows: long paths (`core.longpaths`) and file locking on worktree removal (an open
  handle in an editor makes `RemoveAll` fail; retry with backoff, report what holds it).
- Line endings: the edit/write tools keep a file's existing CRLF/LF style instead of normalizing to LF.

## Testing

- Every commit: the UW0 cross-vet/build gates and `GOOS=windows go test -c ./...`.
- Pure logic (paths, flock contract via an interface fake, shell selection order, URL opener choice) is tested
  on Linux.
- A Windows 11 VM for UW1/UW2 verification and the `//go:build windows` tests. Wine is too partial for
  job objects and ACLs to trust.

## Risks

- **Weaker worktree liveness** (UD6): processes that change directory into a worktree without being tracked go
  unseen. That matches non-Linux Unix today.
- **Shell availability:** without Git for Windows and outside Misa, the agent has POSIX sh but no coreutils.
  The tool description must say so, or the model will loop on "command not found".
- **Credential storage** (UD3): decide before shipping; a half-written keyring entry is worse than the file
  backend.
- **Path checks:** case-insensitivity, 8.3 short names (`PROGRA~1`), UNC and `\\?\` prefixes can defeat naive
  containment checks. `paths.Within` should resolve with `filepath.EvalSymlinks` and `GetFinalPathNameByHandle`
  when the path exists, and fail closed when it can't.

## F01 delivered (#108)

Package selection and the jobleader boundary. The Windows reduced build (`-tags 'nough noweb'`, both
architectures) now resolves every package: `go list -deps .` passes. It does not compile yet.

- `internal/jobleader`: `doc.go` holds the package comment and `Flag`; `jobleader.go` is `unix`;
  `jobleader_other.go` (`!unix`) runs the command and waits, with `Supported = false` and a no-op
  `SuspendOnCtrlZ`. `ui.Model.jobControl` is `jobleader.Supported && config`, so Windows never wraps a command
  in `--job-leader`.
- Build-tag audit: constraints that selected Windows by negation were retagged `unix && …`
  (`builtins`/`procmon`/`sysinfo` `statfs_unix.go`, `term/session_other.go`, `ughbot/vm/procattr_other.go`,
  `wayland/shm_other.go`). Every `internal/wayland` file is now `unix`, so the package is absent on Windows
  (its tests too). Confirmed portable and left alone: `builtins/utimenow.go` (`!darwin && !windows`),
  `builtins/rename_other.go` (`!linux`, returns `ENOSYS`), `gui/backend_other.go` (`!unix || android`, the
  clean "no backend" error), `cmd/misagui/main.go` (`!android || !cgo`, empty stub), `sysinfo/prop_stub.go`,
  `privdir`/`tools`/`webui` `!unix` siblings (compile-only stubs; real Windows behaviour is A01/A02/W01).
- Package set: `platformset_test.go` (root, no subprocess, `go/build` contexts) pins the Windows product
  set, i.e. the in-module import closure of the root package under `nough noweb`, and fails on drift. It
  also checks that wayland, the `wl_*` gui files, the JNI file, ughbot and `gui/android` stay out of that
  closure, that test imports resolve, and that Linux, Android, OpenBSD, FreeBSD and macOS keep their packages
  and the Unix jobleader.
- Remaining compile errors, `GOOS=windows go build -tags 'nough noweb' .` (identical on arm64), by task:
  - F02: `lscore/fileinfo.go` (`syscall.Stat_t`, `statTimes`), `sysinfo/extended.go` (`syscall.Statfs`,
    `getMountSpace`), `sshclient/resize.go` (`SIGWINCH`), `procmon/htop.go` (the signal table, `Kill`,
    `Setpriority`).
  - P01 (with F04 stubs until P02): `term/terminal.go` (`pollable`, `setWinsize`, `waitProcess`,
    `killSession`, `session`).
  - F03/F04: `internal/builtins` is not reached yet because its dependencies fail first; by grep it still
    needs the items listed for `builtins` above.
  - Not in the reduced build, so not F01: the agent packages (`ugh/admission`, `auth`, `policy`, `tools`,
    `worktree`) fail on Unix-only calls (A01–A06).

## F02 delivered (#108)

Filesystem and system metadata compile boundary. `lscore`, `sysinfo`, `procmon` and `sshclient` now build and
vet for `windows/amd64` and `windows/arm64`. Unix (Android included) output is unchanged.

- `lscore`: `meta.go` holds the portable `Fields` flags and `FileInfo.Missing` (zero means every field is
  present, so literals in tests and Unix behaviour are unchanged). `meta_unix.go` reads `Stat_t`;
  `meta_windows.go` reads link count, volume serial and file index with `GetFileInformationByHandle` (opened
  with `FILE_FLAG_OPEN_REPARSE_POINT` for `lstat` semantics) plus access and creation time from the directory
  entry; `meta_other.go` reports everything missing. Windows leaves owner, group, blocks and change time
  unavailable. A missing field prints `?` in `-i`, `-s`, link count, owner and group (the convention
  already used for a dangling link under `-L`); it is never fabricated. `Identify` (symlink-loop identity) has
  the same three providers.
- `sysinfo.mountSpace` and `procmon.diskSpace`: `diskspace_unix.go` wraps `Statfs` and the existing
  per-OS `statfs_*` formulas; `diskspace_windows.go` uses `GetDiskFreeSpaceEx` (sysinfo: bytes available to
  the caller, procmon: total free bytes, matching the `Bavail`/`Bfree` split on Unix); `diskspace_other.go`
  returns an error.
- `procmon.Actions` is the capability table behind the kill dialog (htop `F9`/`k`), btop `k` and htop
  `F7`/`F8`. Unix keeps the full signal menu with `kill` and `setpriority`. Windows offers `Cancel` and
  `Terminate` (`OpenProcess(PROCESS_TERMINATE)` plus `TerminateProcess`) and reports `ErrUnsupported` for
  priority changes. The process list itself still reads `/proc`, so on Windows it is empty until I04.
- `sshclient`: `resizeEvents` is `SIGWINCH` on Unix and no event source elsewhere; `watchSize` (event or
  500 ms poll, apply only on a change) is portable and tested with a fake size source.
- Remaining compile errors for `GOOS=windows go build -tags 'nough noweb' .`: P01 (`term/terminal.go`) and
  F03/F04 (`internal/builtins`: `Stat_t`, signals in `util.go`, `Umask`, `Mkfifo`, `Getpriority`, and so on
  across `arch`, `chown`, `cp`, `du`, `mkdir`, `mv`, `nice`, `proc`, `rm`, `split`, `stat`, `sysutil`,
  `touch`, `truncate`, `umask`, `util`).

## I04 delivered (#108)

Windows process monitor backend. `internal/procmon` now reads an honest Windows process table; nothing reads
`/proc` on Windows and nothing fills a field it could not read. Unix and Android output is unchanged
(byte-identical rows and headers, verified by the new layout tests).

- **Enumeration** — `procs_windows.go`: `CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS)` with
  `Process32First`/`Process32Next` gives pid, parent pid, image name, thread count and `PriClassBase`.
  Enumeration succeeds for every process, including protected ones.
- **Per-process detail** — `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` then `GetProcessTimes`
  (kernel and user in 100 ns units), `GetProcessMemoryInfo` and `QueryFullProcessImageNameW`. x/sys has no
  wrapper for the memory call, so it is resolved from `kernel32.dll` (`K32GetProcessMemoryInfo`, Windows 7+)
  with `psapi.dll` (`GetProcessMemoryInfo`) as the fallback, through `windows.NewLazySystemDLL`. No cgo, no
  new dependency. Working set is RES, pagefile usage is the VIRT equivalent, the image path becomes
  COMMAND.
- **Honest failure** — a process that exits mid-read (`OpenProcess` → `ERROR_INVALID_PARAMETER`) is dropped
  from that snapshot. Access-denied and protected processes (the System process, other users' processes)
  keep their row with the detail fields flagged missing, and PID 0 — the idle process, which has no handle
  to open — keeps its row with no detail for the same reason. Never faked.
- **Missing fields** — `Process.Missing` (`ProcMissing` bit set in `missing.go`) mirrors `lscore`'s
  `FileInfo.Missing`: zero means everything is available, a flagged field renders `?`. Windows has no run
  state, owner or shared-memory counter in this backend, so those three plus %MEM are always flagged; the
  task counts (`N total, … running … sleeping`) and the `Running` tiles are omitted rather than printed as
  zeros. `CellText`/`CellInt`/`CellFloat`/`CellBytes` render every column of `top`, `htop`, `btop` and the
  task manager, so all four skins behave the same.
- **CPU normalization** — per core, Irix mode, exactly as on Unix: `%CPU` is
  `(kernel+user delta) / (wall delta) × 100`, where 100 % is one logical CPU busy, so a process using every
  core of an N-core machine reports N×100 % and a thread-heavy process can exceed 100 % on one core. It is
  deliberately not divided by the logical CPU count, which keeps `top`, `htop`, `btop` and the task manager
  consistent with their Unix output. `GetProcessTimes` already sums all threads, so the same formula gives
  the same scale. `Process.Identity` (start time on Unix, creation time on Windows) makes a recycled PID keep
  `0 %` instead of subtracting the previous owner's CPU time; a counter that goes backwards is ignored.
- **Priority** — Windows has priority classes, not nice values. `priority.go` maps them to the existing
  nice-like UI range (documented Win32 base priorities in brackets): High `-10` [13], Above normal `-5`
  [10], Normal `0` [8], Below normal `10` [6], Idle `19` [4]; Realtime [24] is read but never offered
  (a privilege and a starving-system hazard). PR shows the class base priority, NI the nice equivalent.
  `GetPriorityClass` (with the query handle) is authoritative and overrides the enumerated base priority;
  protected processes keep the enumeration value. `Actions.Levels` plus `PriorityStep` let htop's `F7`/`F8`
  step between classes; away from a class boundary they report "already at the highest/lowest priority"
  instead of a change that did not happen. Setting a class uses `OpenProcess(PROCESS_SET_INFORMATION)` +
  `SetPriorityClass`; privileges or ACLs that refuse it surface in the status line. This supersedes F02's
  temporary `ErrUnsupported` for priority changes.
- **Termination** — unchanged from F02: the signal menu is `Cancel` and `Terminate` only
  (`OpenProcess(PROCESS_TERMINATE)` + `TerminateProcess`). HUP/INT/STOP/CONT are not offered on Windows at
  all rather than failing later.
- **System-wide sections stay unavailable** — `Snapshot.SystemAvailable` is true where procfs supplies
  uptime, load average, memory, per-core CPU, network and disk counters, and false on Windows until the
  native system information provider (I03, PR #127) lands. The four skins render `—` for the uptime/load
  header, the memory bars, the disk and network panels, and the task manager's MEMORY/DISK/NETWORK
  sections, instead of showing zeros. `TakeSnapshot`/`TakeFullSnapshot` and the system-counter readers are
  the platform split (`procfs.go` / `system_windows.go`).
- **Tests** — portable on Linux: `missing_test.go` (cells, flags, snapshot strings, state counts),
  `priority_test.go` (class/base/name mapping, step with and without levels, ordering, Realtime excluded),
  `procrow_test.go` (row seeding, detail application including the 100 ns → tick conversion, recycled PIDs,
  per-core CPU percentages, gone/unreadable processes), `views_test.go` (the exact Unix `top` and `htop` row
  layouts, the count of `?` columns on a bare and a detail Windows row, and all four skins rendering an
  unavailable-systems snapshot without inventing values). Native Windows tests
  (`procs_windows_test.go`, build-tagged) enumerate self, check that a snapshot reports the system sections
  as unavailable, assert no row ever carries a value for a flagged field, and start a test-owned child
  (the test binary re-exec'd as a helper) to change its priority class through Idle/Above normal/High/Normal
  and to terminate it; invalid PIDs must fail. They compile and vet for `windows/amd64` and `windows/arm64`
  but have not been executed (no Windows machine or CI job yet, F05/R02).
- **Temporary limitations** — no per-process run state, owner or shared size; no system memory, uptime,
  load, CPU totals, disk or network counters (`—`, I03); no thread-level CPU; `GOOS=windows go build
  -tags 'nough noweb' .` still stops in `internal/term` (P01) and `internal/builtins` (F03/F04).
