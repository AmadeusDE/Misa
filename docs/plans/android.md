# Plan: Android port (native APK + Termux CLI)

Status: A0 measured 2026-10-27; A1 and A2 landed; A3 in review; later phases not started. Written 2026-10-26 against `master` at `a5982a8`
("Build binaries and AppImages in Actions (#1)"). This file is `docs/plans/android.md`.

Part 1 is the shared groundwork and the Termux CLI (phases A0–A2). Part 2 is the native APK (A3–A6).
Part 3 is CI, releases and F-Droid readiness (A7, plus the CI changes that ship with every phase).

## For the agent doing this work (read first)

- Read `AGENTS.md` (→ `docs/AGENTS.md`) before touching anything. Its rules win over this plan where they
  conflict. Edit `docs/AGENTS.md` directly, never through a symlink.
- Commit this file as `docs/plans/android.md` in the first PR, and keep its **Status** table (end of file)
  current in every later PR.
- One phase per PR, in order. Each PR must leave `ci.yml` green: `gofmt -l .` prints nothing, every existing
  `go build` / `go vet` / tag combination still passes, `go test -count=1 ./...` passes with no user config.
- **Linux, BSD and macOS behaviour must not change.** Every Android-specific file is build-tagged. Remember
  that `GOOS=android` *also satisfies the `linux` build tag*: a file tagged `linux` compiles for Android too.
  Where Android must differ, tag the Linux file `linux && !android` and add an `android` sibling.
- cgo is allowed **only** on Android, and only in files tagged `android && cgo` (see D2). Every other target
  keeps building with `CGO_ENABLED=0`.
- When this plan says "measure" or "verify", do it and write the result into this file (like the
  "Where things stand" table in `docs/plans/windows.md`). Do not guess. If a measurement contradicts a decision
  here, stop and flag it in the PR description instead of silently changing direction.
- Coordinate with `docs/plans/windows.md`: its D4/W2 (split `internal/gui` into an OS-neutral core and
  per-platform backends) is the same refactor as A2 here. Whichever port gets there first does it; the other
  reuses it.

# Goal

1. **Termux CLI:** `misa`, `ugh` (and the multicall builtins) run inside Termux on arm64 phones as a release
   tarball `misa-<tag>-android-arm64.tar.xz`: prompt, mux, job control, builtins, `sshc`, fetch, procmon, ugh.
2. **Native APK "Misa":** a launcher icon that opens Misa full-screen, drawn by Misa's own software renderer
   (the same one behind `misa --gui`), with soft keyboard, hardware keyboard, gamepad and touch input. Primary
   use case: open the app and `sshc` somewhere. Sessions survive the app going to the background.
3. Both built and published by GitHub Actions from the same tree. Distribution: GitHub releases first; keep
   everything F-Droid-compatible so it can be submitted later without rework.

Out of scope for this plan: ughbot (Linux-only, uses SQLite, VMs), 32-bit ABIs (armeabi-v7a, x86), Google
Play submission, a Termux-style package bootstrap inside the APK.

# Where things stand (from reading the code, 2026-10-26)

The table below is the reading from before A0. Measurements from 2026-10-27 follow it. D1–D12 are unchanged; the measurement section ends with the flags those results raise.

| Area | Current state | Android impact |
|---|---|---|
| Build | `CGO_ENABLED=0` everywhere; three binaries; tags `nough`, `nogui`, `noweb` | Android needs cgo (D2); ughbot excluded |
| `internal/gui` | Wayland window: `gui.go` (app), `window.go` (xdg surface, shm buffers), `clipboard.go` (wl data device), `input.go` (xkb keymap via `unix.Mmap`). Renderer (`render.go`, `canvas.go`, `boxdraw.go`, `palette.go`, `font.go`) is OS-neutral and draws premultiplied ARGB8888 | Needs the backend split (A2) and an ANativeWindow backend (A3). Byte order differs from Android's RGBA_8888 |
| `internal/gui/font.go` | Resolves fonts with `fc-match` if on PATH, falls back to Go Mono | No fontconfig on Android: fallback path already exists; add `/system/fonts` lookup |
| `internal/wayland` | `MemfdCreate`, `SCM_RIGHTS` | Not used by the Android backend; must still compile or be excluded under `android` |
| `internal/term` | PTY via creack/pty; `session_linux.go` walks `/proc` to kill sessions; `jobleader` re-execs the Misa binary | `/dev/ptmx` works for apps (Termux proves it). `/proc` of *own* processes is readable. Re-exec needs an executable path (D5) |
| `internal/procmon`, `internal/sysinfo` | `/proc/stat`, `/proc/<pid>`, statfs | Since Android 8, `/proc/stat` is blocked for apps and other apps' `/proc` is hidden: degrade, don't crash |
| `internal/ui` clipboard | `atotto/clipboard` (shells out to xclip/wl-copy) in app.go, copymode, selection, fullshell | Neither exists. Termux: `termux-clipboard-get/set`. APK: JNI ClipboardManager. OSC 52 already exists as fallback |
| `internal/ugh/auth/keyring.go` | `zalando/go-keyring` (D-Bus Secret Service on Linux) | No D-Bus. Needs a file backend (0600, app-private dir), later Android Keystore |
| `internal/pkgmanager` | Data-driven distro package menus | Termux: add a `pkg`/apt definition. APK: hide package menus |
| `internal/ascii/distro` | No Android/Termux art | Add art + detection via `getprop` |
| Networking | Pure-Go resolver reads `/etc/resolv.conf` | Android has no `/etc/resolv.conf` (Termux keeps it under `$PREFIX/etc`), so a pure-Go build falls back to `127.0.0.1:53` and DNS fails. Fixed by D2 |
| Config | `os.UserConfigDir` → `$XDG_CONFIG_HOME` or `$HOME/.config/misa` | Termux sets `HOME`. The APK must set `HOME`, `TMPDIR`, `XDG_*` itself (D10) |

# Where things stand (measured 2026-10-27)

A0 ran on this Linux x86_64 host (`6.12.94+`, Go 1.27.1). No Misa source was changed.
`GOOS=android` still satisfies the `linux` build tag, and that is how every package below was compiled.

## Toolchain

| Piece | Measured |
|---|---|
| NDK | r30 LTS, `Pkg.Revision` `30.0.16248370`, unpacked at `/opt/android/android-ndk-r30`. Linux zip SHA1 `5107f898313790e449e87eee2183d9a20602dee9` |
| Clang | 21.0.0 (`r574158c`). Wrappers `aarch64-linux-android24-clang` and `x86_64-linux-android24-clang` (target triple `aarch64-unknown-linux-android24` / `x86_64-unknown-linux-android24`) |
| NDK platforms | `meta/platforms.json`: min 21, max 37. Aliases in that file stop at `VanillaIceCream` = 35 |
| SDK | command-line tools `Pkg.Revision` 22.0; platform-tools; emulator 37.2.12.0 (build 16428233); `platforms;android-34`; `build-tools;36.0.0`; `system-images;android-34;aosp_atd;x86_64` |
| Java | OpenJDK 21.0.12.1 |

`CC` for every `CGO_ENABLED=1` build was the API-24 wrapper above. A scratch CI job was not added; the CI section below still says the Android job lands with A1.

## Go 1.27.1 and the Android API floor

Go's minimum-requirements note for this toolchain does not name an Android API level (the Linux kernel floor published for Go 1.24+ is 3.2). The NDK itself refuses platforms below 21. The runtime still special-cases older Android: `os/pidfd_linux.go` ignores `SIGSYS` from pidfd syscalls on Android because "In Android version < 12, pidfd-related system calls are not allowed by seccomp" (issue #69065). That is API 31, above `minSdk` 26, and the workaround exists so the runtime can run below it.

**Flag:** D4's `minSdk` = 26 does not need to rise for Go 1.27.1. D4's parenthetical "35 or 36 at time of A3" is a placeholder for A3. At A0 time the pinned NDK already advertises platform 37, and its alias table names releases through API 35. A3 should read the then-current stable API, not freeze 35 or 36 from this sentence.

## `go build ./...`

Tags are `default` (no `-tags`), `-tags nough`, `-tags nogui`, `-tags noweb`.

| `GOARCH` | `CGO_ENABLED` | `go build ./...` |
|---|---|---|
| arm64 | 1 | exit 0 for all four tag sets |
| arm64 | 0 | exit 0 for all four tag sets |
| amd64 | 1 | exit 0 for all four tag sets |
| amd64 | 0 | exit 1 for all four tag sets. The log is the line below, once per main package (7 times), then `all_exit:1` |

```
android/amd64 requires external (cgo) linking, but cgo is not enabled
```

That string comes from the Go tool, not from Misa source. `internal/platform.MustLinkExternal` returns true for `android` whenever cgo is on, and for `android` with cgo off whenever `goarch != "arm64"`. `DefaultPIE` is true for `android`, and `InternalLinkPIESupported` lists `android/arm64` and does not list `android/amd64`. So an arm64 PIE links with the internal linker and no cgo; an amd64 PIE does not. `go build -buildmode=c-shared` is supported for both `android/arm64` and `android/amd64` (`BuildModeSupported`).

`go vet` for `GOOS=android` was not run.

## Per package

`go list ./...` reports 84 packages. For arm64 (cgo 0 and 1) and for amd64 cgo 1, each package was built on its own, for each of the four tag sets. Those twelve result files are byte-identical (sha256 prefix `06993752a96f`).

`go list ./...` with `GOOS=android GOARCH=amd64 CGO_ENABLED=0` aborts while loading main packages (the external-link error), so that enumeration was empty. The same 84 import paths were then built one by one with `GOARCH=amd64 CGO_ENABLED=0` and `-o /dev/null`. Libraries compile. The seven main packages fail with the external-link line. Tag sets do not add or remove packages in this module, so the amd64 cgo 0 column is the default-tag rebuild.

`go build ./...` skips a package that has no non-test Go files and still exits 0. A lone `go build` of that package prints `no non-test Go files` and exits 1. Those six rows are that message on every arch and cgo value.

| package | arm64 cgo 0/1 and amd64 cgo 1, all four tag sets | amd64 `CGO_ENABLED=0` |
|---|---|---|
| `.` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `cmd/getlimits` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `cmd/strerror` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `cmd/ugh` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `cmd/ughbot` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `internal/ascii` | OK | OK |
| `internal/builtins` | OK | OK |
| `internal/chess` | OK | OK |
| `internal/color` | OK | OK |
| `internal/companion` | OK | OK |
| `internal/config` | OK | OK |
| `internal/editor` | OK | OK |
| `internal/fileops` | OK | OK |
| `internal/gitinfo` | OK | OK |
| `internal/gradle` | OK | OK |
| `internal/gui` | OK | OK |
| `internal/help` | OK | OK |
| `internal/jobleader` | OK | OK |
| `internal/justfile` | OK | OK |
| `internal/launcher` | OK | OK |
| `internal/lscore` | OK | OK |
| `internal/makefile` | OK | OK |
| `internal/mines` | OK | OK |
| `internal/mweb` | OK | OK |
| `internal/pkgmanager` | OK | OK |
| `internal/procmon` | OK | OK |
| `internal/sshclient` | OK | OK |
| `internal/sysinfo` | OK | OK |
| `internal/term` | OK | OK |
| `internal/termkeys` | OK | OK |
| `internal/thefuck` | OK | OK |
| `internal/ugh` | `no non-test Go files` | `no non-test Go files` |
| `internal/ugh/admission` | OK | OK |
| `internal/ugh/agent` | OK | OK |
| `internal/ugh/agent/host` | OK | OK |
| `internal/ugh/auth` | OK | OK |
| `internal/ugh/config` | OK | OK |
| `internal/ugh/confine` | OK | OK |
| `internal/ugh/llm` | OK | OK |
| `internal/ugh/llm/chat` | OK | OK |
| `internal/ugh/llm/codex` | OK | OK |
| `internal/ugh/perm` | OK | OK |
| `internal/ugh/perm/model` | OK | OK |
| `internal/ugh/perm/static` | OK | OK |
| `internal/ugh/policy` | OK | OK |
| `internal/ugh/privdir` | OK | OK |
| `internal/ugh/provider` | OK | OK |
| `internal/ugh/provstate` | OK | OK |
| `internal/ugh/roots` | OK | OK |
| `internal/ugh/session` | OK | OK |
| `internal/ugh/toolargs` | OK | OK |
| `internal/ugh/tools` | OK | OK |
| `internal/ugh/tui` | OK | OK |
| `internal/ugh/ughtui` | OK | OK |
| `internal/ugh/usage` | OK | OK |
| `internal/ugh/worktree` | OK | OK |
| `internal/ughbot/api` | OK | OK |
| `internal/ughbot/bot` | OK | OK |
| `internal/ughbot/client` | OK | OK |
| `internal/ughbot/config` | OK | OK |
| `internal/ughbot/daemon` | OK | OK |
| `internal/ughbot/fakellm` | OK | OK |
| `internal/ughbot/ledger` | OK | OK |
| `internal/ughbot/sandbox` | OK | OK |
| `internal/ughbot/sched` | OK | OK |
| `internal/ughbot/store` | OK | OK |
| `internal/ughbot/tui` | OK | OK |
| `internal/ughbot/vm` | OK | OK |
| `internal/ughbot/webhost` | OK | OK |
| `internal/ui` | OK | OK |
| `internal/vi` | OK | OK |
| `internal/wayland` | OK | OK |
| `internal/webadmin` | OK | OK |
| `internal/webdevtui` | OK | OK |
| `internal/webui` | OK | OK |
| `internal/xkb` | OK | OK |
| `testapi` | OK | OK |
| `tests/diffutils/cmp` | `no non-test Go files` | `no non-test Go files` |
| `tests/diffutils/diff` | `no non-test Go files` | `no non-test Go files` |
| `tests/native` | `no non-test Go files` | `no non-test Go files` |
| `tests/report` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `tests/toybox` | `no non-test Go files` | `no non-test Go files` |
| `tests/toybox/testcmd` | OK | `android/amd64 requires external (cgo) linking, but cgo is not enabled` |
| `tests/uutils` | `no non-test Go files` | `no non-test Go files` |

## Dependencies named in A0

From `go list -deps` of `.`, filtered to `charm.land/`, `github.com/creack/pty`, and `golang.org/x/sys`. Versions in `go.mod`: bubbles v2.1.0, bubbletea v2.0.7, glamour v2.0.1, lipgloss v2.0.4, creack/pty v1.1.24, x/sys v0.48.0. Each import path below is `OK` for arm64 and amd64, `CGO_ENABLED` 0 and 1 (20/20 each):

`charm.land/bubbles/v2/{cursor,help,internal/memoization,internal/runeutil,key,list,paginator,spinner,textarea,textinput,viewport}`, `charm.land/bubbletea/v2`, `charm.land/glamour/v2`, `charm.land/glamour/v2/ansi`, `charm.land/glamour/v2/internal/autolink`, `charm.land/glamour/v2/styles`, `charm.land/lipgloss/v2`, `charm.land/lipgloss/v2/table`, `github.com/creack/pty`, `golang.org/x/sys/unix`.

`creack/pty` v1.1.24 selects `pty_linux.go` (`//go:build linux`) for Android. `golang.org/x/sys/unix` v0.48.0's Linux files (`zsyscall_linux.go`, `zsyscall_linux_arm64.go`, and the unix/linux-family files that define `Mmap` and `UnixRights`) are tagged `linux`, so they compile for `GOOS=android`.

## Linkage, page size, c-shared

Default `go build` (unstripped). `readelf -W -l` LOAD `p_align`, interpreter, and `DT_NEEDED`:

| binary | interp | `DT_NEEDED` | LOAD `p_align` |
|---|---|---|---|
| arm64 cgo 1 `misa` | `/system/bin/linker64` | `liblog.so`, `libdl.so`, `libc.so` | `0x4000` |
| arm64 cgo 0 `misa` | `/system/bin/linker64` | none | `0x10000` |
| amd64 cgo 1 `misa` | `/system/bin/linker64` | `liblog.so`, `libdl.so`, `libc.so` | `0x4000` |
| hello, same three ways | same pattern as `misa` | same pattern as `misa` | same (`0x4000` cgo, `0x10000` arm64 cgo 0) |

Both alignments are multiples of 16 KiB. The cgo binaries are what D2 ships; NDK lld already emits `0x4000` here, with no extra `-Wl,-z,max-page-size`. The arm64 internal linker (cgo off) uses `0x10000`, which is the arm64 `FlagRound` default in this Go toolchain. amd64 cgo 0 produced no binary to measure.

A throwaway module `hello` (`fmt` + `net.LookupHost`) links for arm64 cgo 0, arm64 cgo 1, and amd64 cgo 1. amd64 cgo 0 fails with the same external-link line.

`-buildmode=c-shared` of that hello module, cgo 1, API-24 clang:

| output | bytes | LOAD `p_align` | `DT_NEEDED` |
|---|---|---|---|
| `libhello-arm64.so` | 3425968 | `0x4000` | `liblog.so`, `libdl.so`, `libc.so` |
| `libhello-amd64.so` | 3603856 | `0x4000` | `liblog.so`, `libdl.so`, `libc.so` |

`nm -D` on the arm64 `.so` shows `getaddrinfo@LIBC`, `getnameinfo@LIBC`, `res_search@LIBC`, and `__android_log_vprint`. The build logs are empty (exit 0). No `.h` file was written next to either `.so`.

## Sizes (bytes, unstripped `go build`, unless noted)

amd64 cgo 0 has no linked binary. `ugh` / `ughbot` barely move across tags; the delta from the default-tag size is in parentheses where it is not zero.

| binary | tags | arm64 cgo 1 | arm64 cgo 0 | amd64 cgo 1 |
|---|---|---:|---:|---:|
| `misa` | default | 38475064 | 38739466 | 41204224 |
| `misa` | `nough` | 27200896 | 27517726 | 29179608 |
| `misa` | `nogui` | 36916056 | 37146084 | 39604816 |
| `misa` | `noweb` | 36616280 | 36959785 | 39225360 |
| `ugh` | default | 28590296 | 28851787 | 30613872 |
| `ugh` | other three tags | 28590296 | 28851787 | 30613904 (+32) |
| `ughbot` | default | 31240328 | 31547185 | 33098368 |
| `ughbot` | other three tags | 31240304 (−24) | 31547185 | 33098280 (−88) |

`GOOS=android GOARCH=arm64 CGO_ENABLED=1` with `-trimpath -ldflags '-s -w'`: `misa` 29174840 bytes, `misa -tags nough` 19972216 bytes. The "~34 MB / ~19 MB" figures in `docs/AGENTS.md` are the Linux builds. The `nough` Android number matches ~19 MB; the default Android binary is smaller than ~34 MB. D1 still excludes `ughbot` from the tarball and the APK; it compiled, and that does not put it in the deliverable.

## Runtime (API 34 x86_64 emulator)

Nested KVM did not boot a guest. `/dev/kvm` is present; opening it succeeded after it was mode `0666`. `qemu-system-x86_64` with `-accel on` did not come online, and host `dmesg` at that moment was:

```
kernel BUG at arch/x86/kvm/x86.c:702!
RIP: 0010:kvm_spurious_fault+0xe/0x10
kvm_arch_vcpu_create+0x1e6/0x300
```

The same AVD then booted with `-accel off` (TCG): `emulator-5554`, `Linux localhost 6.1.23-android14`, `ro.build.version.sdk` 34, release 14, `ro.product.cpu.abi` `x86_64`, `ro.build.version.min_supported_target_sdk` 28. Only x86_64 binaries were executed. Termux is not installed on this image. `sys.boot_completed` was not a useful signal; `adb shell`, `pm`, and the activity ran anyway.

What ran, in the shell domain (`/data/local/tmp`, context `u:r:shell:s0`), using the amd64 cgo 1 binaries:

| check | result |
|---|---|
| `/data/local/tmp/misa echo hello-from-shell` | stdout `hello-from-shell`, exit 0 |
| copy of that binary named `true` | exit 0, empty stdout (argv0 basename `true` hits the builtin) |
| `/etc/resolv.conf` and `/system/etc/resolv.conf` | both absent |
| `/system/usr/share/zoneinfo/tzdata` | present, 429558 bytes. This path is what `time/zoneinfo_android.go` reads, and that file is `GOOS=android`, not cgo |
| `hello` `LookupHost("example.com")` (cgo resolver; eth0 `10.0.2.15/8`) | `[172.66.147.243 104.20.23.154 2606:4700:10::ac42:93f3 2606:4700:10::6814:179a] <nil>`, exit 0 |
| `GODEBUG=netdns=2` on that lookup | `hostLookupOrder(example.com) = cgo`, same addresses |
| `GODEBUG=netdns=go` | `lookup example.com on [::1]:53: read udp [::1]:56928->[::1]:53: read: connection refused` |
| `x509.SystemCertPool` (cgo amd64 probe, `len(pool.Subjects())`) | `pool=134 err=<nil>`. `/system/etc/security/cacerts` has 134 entries. `crypto/x509/root_linux.go` appends that directory when `goos.IsAndroid == 1` |
| `tls.Dial("tcp", "example.com:443")` with `ServerName` `example.com` | `tls-ok example.com`, exit 0 |
| `/proc/stat` from `adb shell` | readable (`cpu  …`), exit 0 |
| `/dev/ptmx` | `crw-rw-rw-` `u:object_r:ptmx_device:s0` |

**Flag:** D2's DNS reason holds on this image. With cgo, `LookupHost` and a TLS handshake to `example.com:443` succeeded, and `GODEBUG=netdns=2` selected the cgo resolver. With `GODEBUG=netdns=go`, the lookup used `[::1]:53` and was refused. There is no `/etc/resolv.conf`. tzdata comes from `GOOS=android` (`time/zoneinfo_android.go`) with or without cgo. cgo is also what links `android/amd64` (see `MustLinkExternal` above). arm64 still links with `CGO_ENABLED=0`; that binary has no `libc` `DT_NEEDED` and uses the pure-Go resolver.

## Throwaway APK (D5)

Hand-built, not Gradle. Package `de.amadeus.misa.a0`, `minSdkVersion` 26, `targetSdkVersion` 34, `compileSdkVersion` 34, `android:extractNativeLibs="true"`, `android:debuggable="true"`, launcher activity `.Main`. `classes.dex` from `d8 --min-api 26` (javac `--release 17` against `android.jar` of platform 34). Native payload is the amd64 cgo 1 `misa` binary stored as `lib/x86_64/libmisa.so` (41204224 bytes uncompressed; zipalign reports it compressed inside the APK). APK size 18895366 bytes. `zipalign -c -p 4` succeeded. `apksigner verify`: v2 true, v3 true, v1 false, one signer (debug keystore). Installed with `adb install` and started.

`run-as de.amadeus.misa.a0 cat files/a0-result.txt`:

```
nativeLibraryDir=/data/app/~~QenhuRNvMqTmnoE801uvjw==/de.amadeus.misa.a0-Gs0f5lhxiSX3OtCwuCYkzQ==/lib/x86_64
lib exists=true canExecute=true len=41204224
symlink=/data/user/0/de.amadeus.misa.a0/files/bin/echo -> /data/app/~~QenhuRNvMqTmnoE801uvjw==/de.amadeus.misa.a0-Gs0f5lhxiSX3OtCwuCYkzQ==/lib/x86_64/libmisa.so
direct exit=0 out=direct-ok\n
symlink-echo exit=0 out=via-symlink\n
symlink-true exit=0 out=
ptmx open ok fd
procstat ERR FileNotFoundException: /proc/stat: open failed: EACCES (Permission denied)
cacerts dir=true count=134
getprop exit=0 out=34\n
DONE
```

Direct exec was `libmisa.so echo direct-ok` (basename is not a builtin, so `argv[1]` selects `echo`). The `files/bin/echo` symlink was executed with argument `via-symlink` (basename `echo` selects the builtin). `files/bin/true` exited 0 with empty output.

Labels on device:

| path | mode | SELinux |
|---|---|---|
| `nativeLibraryDir/libmisa.so` | `-r-xr-xr-x` owner `system` | `u:object_r:apk_data_file:s0` |
| `files/bin/echo` and `files/bin/true` | symlink, owner `u0_a108` | `u:object_r:app_data_file:s0:c108,c256,c512,c768` |

AOSP `master` sepolicy fetched the same day (`app.te`, `app_neverallows.te`, `untrusted_app_27.te`, `global_macros`, `file_contexts`):

- `file_contexts`: `/data/app(/.*)?` → `u:object_r:apk_data_file:s0` (matches the extracted library).
- `app.te`: `allow appdomain apk_data_file:file { getattr open read ioctl lock map x_file_perms }`. `global_macros` defines `x_file_perms` as `{ getattr execute execute_no_trans map }`.
- `app_neverallows.te`: `neverallow` of `execute_no_trans` on `{ app_data_file privapp_data_file }:file` for `all_untrusted_apps` except `untrusted_app_25`, `untrusted_app_27`, and `runas_app`. The comment says this is the W^X block, with an exception for `targetApi <= 28`.
- `untrusted_app_27.te`: domain for `25 < targetSdkVersion <= 28`, and it `allow`s `app_data_file:file execute_no_trans`. This APK's targetSdk is 34, so that exception is not the domain it ran in.

The symlink lives in `app_data_file` and its target is `apk_data_file`. Exec of the symlink succeeded on this image (permissive was not required for the exit 0). `os.Executable()` / `/proc/self/exe` after the symlink exec was not printed.

**Flag:** on API 34, targetSdk 34, `untrusted_app`, D5 works: exec of `nativeLibraryDir/libmisa.so` and exec through a `filesDir/bin/<builtin>` symlink both exited 0, and the basename selected the builtin. D5's symlink plan stays; `builtin_mode: inprocess` stays the fallback D5 already names. This is one emulator image (x86_64, API 34, debuggable extractNativeLibs APK). Termux and targetSdk 37 are still unmeasured.

`/proc/stat` is readable for `shell` and `EACCES` for the app. That matches the procmon note in the code-reading table: degrade, do not crash. The app opened `/dev/ptmx` (`FileInputStream`). `pty.Start` (creack/pty), mux panes, `^Z`/`fg`, `sshc`, `fetch`, procmon, and `ugh` against its API were not run.

## What A1 still has to measure

| Item | Why it is not in this file as a pass |
|---|---|
| Termux on a device or emulator | not installed here |
| arm64 execution | host has no ARM guest; only the x86_64 ATD image ran binaries |
| `pty.Start`, mux, job control, `sshc`, `fetch`, procmon, `ugh` TLS to its own API | not run. PTY evidence is the open of `/dev/ptmx` plus `creack/pty` compiling. `example.com:443` TLS succeeded from `/data/local/tmp`, which is the shell domain |
| `os.Executable()` after a multicall symlink | not printed |
| targetSdk = current stable, and a non-debuggable APK | this APK is targetSdk 34, debuggable |
| KVM emulator in CI | nested KVM hit the host BUG above; TCG booted this AVD. CI hardware may differ |
| `go vet` for `GOOS=android` | run in A1. `CGO_ENABLED=1` vets both arches. `CGO_ENABLED=0` vets arm64. android/amd64 still cannot load a test binary without cgo, so that arch typechecks libraries with `go build` |

A0 found no Misa compile error on the cgo builds, either arch, any of the four tag sets. A1's "fix every compile failure" list from this matrix is empty. Runtime work in A1 is unchanged. `internal/wayland` and `internal/gui` are in the OK set, including amd64 `CGO_ENABLED=0` (they are libraries). Excluding them from the Android backend is an A2 decision. `internal/ughbot/...` compiled too; D1 still leaves `ughbot` out of both deliverables.

# Decisions

- **D1. Two deliverables, one tree.**
  (a) `android/arm64` CLI tarball for Termux (`misa`, `ugh`; no `ughbot`).
  (b) `Misa-<tag>-<abi>.apk` (and a universal APK) containing the native GUI library plus the same CLI binary.
  The APK *contains* the CLI build; nothing is built twice in different ways.

- **D2. `GOOS=android` with cgo through the NDK, for both deliverables.** Reasons: bionic `getaddrinfo` fixes
  DNS in both Termux and the APK; Go's `time` package finds Android's tzdata only under `GOOS=android`; the APK
  needs `-buildmode=c-shared` and the NDK headers (ANativeWindow, JNI) anyway. Rules:
  - `import "C"` appears only in files tagged `android && cgo`. Add a CI guard step that fails otherwise.
  - Everything that *can* be pure Go stays pure Go and is tagged plain `android`.
  - The CLI targets NDK API level **24** (Termux's floor, Android 7). The APK's GUI library targets the APK's
    `minSdk` (D4). Pin one NDK release (latest LTS at time of A0) in a single place (`android/ndk.version` or
    the workflow env) and use it everywhere.

- **D3. ABIs: `arm64-v8a` required, `x86_64` second.** x86_64 exists so the APK and tests run on the CI
  emulator and on Chromebooks. No 32-bit for now.

- **D4. `targetSdk` = current stable (35 or 36 at time of A3), `minSdk` = 26.** Targeting current keeps F-Droid
  (and Play, if ever) open. Android 10+ W^X forbids executing files from the app's writable data dir when
  targetSdk ≥ 29; D5 handles that instead of pinning an old targetSdk the way Termux does. Confirm Go 1.27.1's
  minimum supported Android version during A0 and raise `minSdk` if needed.

- **D5. The APK execs the CLI binary from `nativeLibraryDir`.** The Misa CLI executable is packaged as
  `jniLibs/<abi>/libmisa.so` (it must be named `lib*.so`) with `android:extractNativeLibs="true"` /
  `packaging { jniLibs { useLegacyPackaging = true } }`. Files there are labelled executable by SELinux and are
  allowed under W^X. The GUI library (`libmisagui.so`, `-buildmode=c-shared`) runs that path on a PTY exactly
  as `gui.Run` does on Linux, so `os.Executable()`, `jobleader` re-exec and `ui.BuiltinExecutable` all work.
  - Multicall names: create `filesDir/bin/<builtin>` symlinks → `nativeLibraryDir/libmisa.so` on first run and
    after every app update (`nativeLibraryDir` changes on update). **Verify in A0** that exec through such a
    symlink is permitted; if not, default `builtin_mode: inprocess` on Android and skip the symlinks.

- **D6. Thin Java shell, no Kotlin, no AndroidX, no Compose.** Gradle + Android Gradle Plugin with a checked-in,
  checksum-validated wrapper. All logic stays in Go; Java only owns Activity/Service lifecycle, the
  `SurfaceView`, the `InputConnection`, and JNI shims for system services. Keeping the Java side tiny keeps it
  F-Droid friendly and keeps agents in Go.

- **D7. Rendering: `ANativeWindow` software blit.** `ANativeWindow_fromSurface` →
  `ANativeWindow_setBuffersGeometry(w, h, WINDOW_FORMAT_RGBA_8888)` → `ANativeWindow_lock` (pass the union of
  the renderer's damage rects as `inOutDirtyBounds`) → copy rows → `unlockAndPost`. The canvas is premultiplied
  ARGB (BGRA in memory); either swizzle during the copy or make pixel order a canvas parameter chosen by the
  backend. Benchmark both with the existing `render_bench_test.go` and pick the faster. Respect the buffer's
  `stride`. No GL/Vulkan.

- **D8. Input lives in Go; Java only forwards events.**
  - Soft keyboard: custom `InputConnection` (`commitText`, `setComposingText`, `deleteSurroundingText`,
    `sendKeyEvent`), `inputType = TYPE_NULL`-style raw mode with an optional "IME-friendly" mode for swipe
    keyboards. Forward text and keys over JNI into the same encoders `gui` uses (`ui.KeyBytes`, `PasteBytes`).
  - Hardware keyboards: map `KeyEvent` keycodes + meta state onto the existing key model (`internal/termkeys`).
  - **Gamepad is first-class** (Misa is built for controller use): map `KEYCODE_BUTTON_*`, d-pad and stick
    `MotionEvent` axes onto the same controller bindings Misa uses on the Steam Deck. One mapping table, shared.
  - Touch: tap = click (`MouseBytes`), vertical drag = scroll/wheel, long-press = selection + copy, pinch = font
  size, two-finger tap = paste. Mouse/trackpad events pass through to programs like on Wayland.
  - Extra-keys row (Esc, Ctrl, Alt, Tab, arrows, `|`, `~`, `/`) as a plain Java `LinearLayout` above the IME,
    toggleable; it sends key events through the same JNI path.

- **D9. Platform services behind small Go interfaces, three implementations each** (Linux/BSD as today, Termux,
  APK via JNI): clipboard, secret storage for ugh auth, open-URL, notifications (APK only). Termux detection:
  `TERMUX_VERSION` / `PREFIX` env. APK detection: an env var the APK sets (`MISA_ANDROID_APP=1`).
  ugh secrets: v1 = file backend, `0600`, under the app's private dir (the app sandbox is the protection);
  v2 = Android Keystore-wrapped key via JNI.

- **D10. The APK builds the child environment.** `HOME=filesDir/home`, `TMPDIR=cacheDir`,
  `XDG_CONFIG_HOME=$HOME/.config`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME=cacheDir`,
  `PATH=filesDir/bin:/system/bin`, `TERM` as `gui` already sets it, `LANG=C.UTF-8`, `SHELL` = the Misa path,
  plus `MISA_ANDROID_APP=1` and `ANDROID_DATA`/`ANDROID_ROOT` passed through. Config search stays as today
  (`~/.config/misa`, or next to the binary — the latter is read-only in the APK, so the APK path is `$HOME`).

- **D11. Sessions live in a foreground Service, not the Activity.** The Go side keeps the `term.Terminal` and
  PTY alive when the surface is destroyed and re-attaches on `surfaceCreated` (full redraw). The service shows
  a persistent notification ("N sessions", "Exit"). Android 14+ requires a foreground service type: use
  `specialUse` with a justification property (as Termux-like apps do) and request `POST_NOTIFICATIONS` on 13+.
  Ending the last session stops the service.

- **D12. Feature matrix on Android.**

  | Feature | Termux CLI | APK |
  |---|---|---|
  | Prompt, mux, jobs, builtins, vi, mines, chess, fetch | yes | yes |
  | `sshc` | yes | yes (headline feature) |
  | procmon (htop/btop clones) | degraded: own processes, no `/proc/stat` CPU% | same |
  | ugh (embedded + `ugh` CLI) | yes, file secret backend | yes, file → Keystore |
  | Package menus | Termux `pkg` definitions | hidden |
  | `misa --gui` (Wayland) | compiled, errors cleanly without `WAYLAND_DISPLAY` | n/a — the APK *is* the GUI |
  | `misa --web` | yes (localhost) | built, not exposed in v1 |
  | ughbot | no | no |

# Part 1: groundwork and Termux CLI

## A0. Measure (no product code)

1. Install the pinned NDK locally and in a scratch CI job. Build per package:
   `GOOS=android GOARCH=arm64 CGO_ENABLED=1 CC=$NDK/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android24-clang go build ./...`
   and the same with `-tags nough`, `-tags nogui`, `-tags noweb`, and for `GOARCH=amd64` (`x86_64-linux-android24-clang`).
   Also try `CGO_ENABLED=0` to record exactly what breaks without cgo (expected: DNS at runtime, maybe link).
   Record failures in a table in this file, like `windows.md` does.
2. On a real arm64 device or an emulator over `adb`: push the binary to `/data/local/tmp`, run it under
   `adb shell -t`, and inside Termux. Check: PTY + mux panes, `^Z`/`fg`, `sshc` to a hostname (DNS), `fetch`,
   procmon, `ugh` reaching its API (TLS roots: Go reads `/system/etc/security/cacerts` on Android — confirm).
3. In a throwaway APK (can be hand-built), verify D5: exec `nativeLibraryDir/libmisa.so` from the app process
   and exec it through a symlink in `filesDir/bin`.
4. Confirm Go 1.27.1's minimum Android API, and check `charm.land/*`, `creack/pty`, `golang.org/x/sys` compile
   for android/arm64 and android/amd64.

Measured 2026-10-27. Results, the exact per-package table, and the flags are in
**Where things stand (measured 2026-10-27)** above. The exit criterion is met for everything this host
could run (NDK matrix, SDK, TCG emulator, throwaway APK). The rows under "What A1 still has to measure"
are the checks that need a device, Termux, or the CI emulator.

## A1. Termux CLI port

- Fix every compile failure from A0 with build tags, smallest change first.
- `sysinfo`/`fetch`: Android detection (`getprop ro.build.version.release`, `ro.product.model`,
  `ro.product.manufacturer` via `/system/bin/getprop`, or `__system_property_get` in an `android && cgo` file).
  Add Android and Termux ASCII art under `internal/ascii/distro`.
- `procmon`/`sysinfo`: treat `EACCES` on `/proc/stat` and missing pids as "unavailable", show "—", never error
  out. Unit-test with a fake procfs root if the package allows it.
- Clipboard: introduce the D9 interface; Termux implementation shells out to `termux-clipboard-get/set` when
  present; otherwise OSC 52. Replace direct `atotto/clipboard` calls in `internal/ui` with the interface (Linux
  implementation still uses atotto, so no behaviour change).
- ugh secrets: file backend selected on Android (and usable elsewhere via config if useful), `0600`, atomic
  write. Tests must not touch the real home (AGENTS.md rule).
- `pkgmanager`: Termux `pkg` definition (search/install/remove/upgrade).
- `term/session_linux.go`: confirm the `/proc` walk works for own processes on Android; if it fails, tag it
  `linux && !android` and use the `session_other.go` strategy on Android.
- `justfile`: `android-cli` recipe (needs `ANDROID_NDK_HOME`), output `dist/misa-android-<arch>/`.

Exit: the A0 device checklist passes in Termux; Linux behaviour unchanged; CI builds android/arm64 and
android/amd64 (see CI section).

## A2. Split `internal/gui` into core + backends (shared with Windows W2)

- Core (OS-neutral): app state, `term.Terminal` ownership, snapshot → renderer → canvas, damage tracking, OSC
  queue, font library, key/mouse/paste translation into PTY bytes, clipboard *interface*.
- Backend interface (sketch — refine in the PR):
  `Size() (w, h int, scale float64)`, `Present(pixels []byte, stride int, damage []image.Rectangle)`,
  `SetTitle(string)`, `Events() <-chan Event` (resize, key, text, pointer, scroll, focus, close),
  `Clipboard() Clipboard`.
- Move the Wayland code (`window.go`, wl clipboard, xkb `input.go`) into the Wayland backend with no behaviour
  change. All existing `internal/gui` tests and benchmarks keep passing; add a fake backend for core tests.
- Font lookup: add `/system/fonts` (and `$PREFIX/share/fonts` in Termux) to the non-fontconfig path.

Exit: Linux `misa --gui` identical (compare screenshots/bench numbers in the PR); fake-backend tests in CI.

# Part 2: native APK

## A3. Android GUI backend + c-shared library

- `internal/gui/android/` (tag `android && cgo`): ANativeWindow presentation per D7, display density → font
  size, insets (status/nav bar, IME) → terminal rows/cols, event queue fed from JNI.
- `cmd/misagui/` (or `android/go/`) with `-buildmode=c-shared` exporting the JNI entry points:
  `onCreate(env vars, exe path)`, `onSurfaceCreated(surface)`, `onSurfaceChanged(w, h)`,
  `onSurfaceDestroyed()`, `onText(string)`, `onKey(code, meta, down)`, `onMotion(...)`, `onPinch(scale)`,
  `onLowMemory()`, `newSession()`, `closeSession(id)`. Use `RegisterNatives` from `JNI_OnLoad` in a small C
  file rather than long `Java_…` export names.
- Session manager in Go: several `term.Terminal`s (tabs), each running `libmisa.so` on a PTY; survives surface
  loss (D11).

## A4. APK shell (`android/` Gradle project)

- `android/app/src/main/java/.../`: `MisaActivity` (SurfaceView, InputConnection, extra-keys row, IME show/hide,
  orientation + resize), `SessionService` (foreground, notification, owns the Go library lifetime),
  `Bootstrap` (D10 env, `filesDir/bin` symlinks per D5, first-run default config, re-run on app update),
  `Native` (JNI declarations).
- Manifest: `INTERNET`, `FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_SPECIAL_USE`, `POST_NOTIFICATIONS`,
  `extractNativeLibs`, `windowSoftInputMode=adjustResize`, gamepad-friendly
  (`android.hardware.gamepad` *not required*, `touchscreen` *not required* so it installs on TV/handhelds).
  Optional, user-triggered: storage access (a "Link shared storage" action that symlinks `~/storage` to
  `/sdcard` after the permission prompt), off by default.
- Gradle builds call a script (`android/build-go.sh`) that cross-compiles both Go artifacts per ABI into
  `jniLibs/<abi>/` (`libmisa.so` = CLI executable, `libmisagui.so` = c-shared). Gradle never invokes Go
  implicitly in a way that hides failures; the script is runnable on its own.
- Icon: generate adaptive icon from `packaging/appimage/misa.svg`.
- `justfile`: `apk` (debug) and `apk-release` recipes.

## A5. Platform services over JNI

- Clipboard (ClipboardManager), open URL (`ACTION_VIEW` — used by ugh auth flows and links in panes),
  ugh secret storage v2 (Keystore-wrapped key), share-sheet target ("Open with Misa" for text/files: paste
  text into the active pane, or `cd` to a shared file's copy), haptic tick on bell (configurable).

## A6. Controller, touch and "phone ssh" UX

- Gamepad mapping parity with the Steam Deck bindings, tested on a Bluetooth controller.
- Launcher static shortcuts: "New session", "SSH…" (asks for a host from `~/.ssh/config` entries).
  Deep link `misa://ssh/<user@host>` → new session running `sshc user@host` (same as `misa --ssh`).
- Settings screen entries surfaced through Misa's own settings TUI: font size, extra-keys row on/off, IME mode,
  keep-screen-on, vibrate on bell.
- Rotation and split-screen/freeform resize keep the session and reflow correctly.

# Part 3: CI, releases, F-Droid

## CI changes (`.github/workflows/ci.yml`)

Add a job `android` (runs on `ubuntu-24.04`; keep it separate so the existing `check` job is untouched):
1. `actions/checkout@v5`, `actions/setup-go@v6` (`1.27.1`), `actions/setup-java@v4` (Temurin 17) for A4+.
2. NDK: use the pinned version from the single source of truth (D2). Install with
   `sdkmanager "ndk;<version>"` (the runner image ships the SDK) and export `ANDROID_NDK_HOME`. Cache it.
3. Guard: fail if any `.go` file containing `import "C"` lacks an `android` build constraint.
4. Build: `misa` and `ugh` for android/arm64 and android/amd64, default tags and each of `nough`, `nogui`,
   `noweb`; once A3 lands, also the c-shared GUI library.
5. Vet: `go vet ./...` for `GOOS=android` with cgo (both arches) and with `CGO_ENABLED=0` (catches pure-Go
   files tagged `android`).
6. Also add `android` to the existing cross-vet loop only if it works without cgo; otherwise the new job covers it.
7. From A4: `gradle/actions/wrapper-validation`, then `./gradlew assembleDebug lint` with Gradle caching.

Add a job `android-device` (path-filtered to `android/**`, `internal/gui/**`, `internal/term/**`, `**/*android*`,
plus nightly `schedule` and `workflow_dispatch`, since emulators are slow):
- `reactivecircus/android-emulator-runner` (x86_64 image, API = minSdk and API = targetSdk), with KVM enabled.
- `go test -c` the portable packages for android/amd64, `adb push`, run them on the emulator
  (`internal/term`, `internal/builtins`, `internal/gui` core, `internal/sshclient` with a local test server).
- From A4: install the debug APK, launch it, wait, `adb shell screencap` and upload the screenshot as an
  artifact; fail if the activity crashed (`adb logcat` check for `FATAL EXCEPTION` / Go panics).

## Build/release changes (`.github/workflows/build.yml`)

- New job `android` (needs nothing else): builds
  - `android-arm64/` and `android-amd64/` CLI dirs (`misa`, `ugh`, `LICENSE`, `README.md`,
    `THIRD_PARTY_NOTICES.md`) — same layout as the other platforms;
  - from A4, APKs: per-ABI `Misa-<version>-arm64-v8a.apk`, `Misa-<version>-x86_64.apk`, and a universal APK.
  Upload as artifacts `android-<arch>` and `apk`. Add it to `bundle.needs`.
- **Fix the release step:** it currently runs `chmod +x "$dir"/misa "$dir"/ugh "$dir"/ughbot` for every
  non-AppImage dir, which fails for Android dirs (no ughbot). Change to
  `chmod +x "$dir"/misa "$dir"/ugh; [ -e "$dir/ughbot" ] && chmod +x "$dir/ughbot"`, and add an `apk)` case
  that moves `*.apk` into `release/`. APKs are included in `SHA256SUMS`.
- **Signing:**
  - Tags (`v*`): release-signed with a keystore from repository secrets: `ANDROID_KEYSTORE_B64`,
    `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD`. Decode into `$RUNNER_TEMP`, never
    the workspace; sign with `apksigner` (v2+v3 schemes); `apksigner verify` before upload. Only the tag job
    gets the secrets (`environment: release`, so fork PRs never see them).
  - `master`/PR builds: debug-signed, artifact only. Name them `-debug.apk` so nobody confuses them.
  - Add `*.jks`, `*.keystore`, `android/local.properties` to `.gitignore`; check `.gitleaks.toml` covers keystores.
  - **Dakota action item:** generate the release keystore once, back it up offline, add the four secrets.
    Losing it means users can't upgrade in place.
- **Versioning:** `versionName` = `git describe --tags --always`; `versionCode` from the tag
  (`vMAJOR.MINOR.PATCH` → `MAJOR*1_000_000 + MINOR*1_000 + PATCH`, ×10 + ABI digit for split APKs) and from
  commit count on non-tag builds. Compute it in one script used by Gradle and CI.
- **Reproducibility (needed for F-Droid's reproducible-build verification):** `-trimpath`,
  `-ldflags='-s -w -buildid='`, pinned NDK and build-tools versions, `SOURCE_DATE_EPOCH` from the commit,
  no timestamps in assets, deterministic zip ordering (AGP default). Add a CI step that builds the release APK
  twice and diffs the unsigned outputs.
- Pin every third-party action to a major version consistent with the repo (`@v4`/`@v5`/`@v6`); the AppImage
  job's checksum-pinning style is the model for any downloaded tool.

## A7. F-Droid readiness (no submission yet)

- `fastlane/metadata/android/en-US/` (title, short/full description, screenshots from the emulator job,
  changelogs per versionCode).
- No proprietary dependencies (no Play Services, no Firebase, no AndroidX needed). OSL-3.0 is an accepted
  license.
- Document the build for F-Droid's `srclibs` (Go toolchain + NDK) in `docs/ANDROID.md`.
- Anti-features to declare honestly: `NonFreeNet` does not apply unless ugh's default providers count; decide
  at submission time.

## Docs to update along the way

- `README.md`: Download (Android tarball, APKs) and Build (Android section, NDK requirement — the "no other
  build tools and no cgo" sentence must gain "except Android").
- `docs/AGENTS.md`: Android build/vet commands, the `android`-implies-`linux` tag rule, the cgo rule,
  architecture entries for `internal/gui/android`, `cmd/misagui`, `android/`.
- `CONTRIBUTING.md`: how to run the emulator tests locally.
- New `docs/ANDROID.md`: user docs (Termux install, APK install, permissions, storage, controller mapping).

# Risks and open questions

- **Exec through symlinks under W^X** (D5) — measured in A0 on API 34 targetSdk 34 (x86_64, debuggable
  extractNativeLibs APK): both the direct `libmisa.so` exec and the `filesDir/bin` symlink exec exited 0.
  The in-process fallback stays a fallback. Termux and a current-stable targetSdk were not measured.
- **Soft keyboards vary** (Gboard swipe, Samsung, SwiftKey) — raw `InputConnection` handling needs testing on
  at least two IMEs; keep an "IME-friendly" mode.
- **Background killing / Doze:** foreground service mitigates; long ssh sessions may still drop on aggressive
  OEM ROMs. Document "disable battery optimisation" for the app.
- **Phantom process killer (Android 12+)** can kill child processes beyond a limit; Misa's own children are
  usually few, but heavy mux use could hit it. Document the adb workaround; measure in A6.
- **procmon usefulness** is limited by Android's `/proc` restrictions; that's expected, not a bug.
- **Binary size:** CLI ~34 MB with ugh; APK ships it twice per ABI only if both executable and c-shared embed
  everything — measure, and consider building `libmisagui.so` with `nough` (the agent runs in the child anyway).
  A0 sizes are in the measurement section. Stripped arm64 cgo `misa` is 29174840 bytes; `-tags nough` is
  19972216 bytes. The ~34 MB / ~19 MB pair in `docs/AGENTS.md` is the Linux build.

# Status

| Phase | State | PR | Notes |
|---|---|---|---|
| A0 Measure | done | [#2](https://github.com/AmadeusDE/Misa/pull/2) | NDK r30 matrix, API floor, emulator, throwaway APK. Leftovers for A1 are listed in the measurement section. Decisions flagged, not rewritten |
| A1 Termux CLI | done | [#3](https://github.com/AmadeusDE/Misa/pull/3) | sysinfo/fetch, procmon degrade, clipboard, file secrets on Android, pkg, session walk. Termux-on-device still unmeasured; the emulator job covers amd64 |
| A2 gui core/backend split | done | [#7](https://github.com/AmadeusDE/Misa/pull/7) | shared with Windows W2. Core in `internal/gui` (`app.go`, `backend.go`, renderer, fonts). Wayland is `wl_*.go` with `unix && !android` (Android matches both `linux` and `unix`). `misa --gui` on Android errors cleanly. Pixel format is `FormatBGRA8888` or `FormatRGBA8888`; the core draws in the backend's order |
| A3 Android backend + c-shared | in review | [#5](https://github.com/AmadeusDE/Misa/pull/5) | `internal/gui/android` (ANativeWindow RGBA_8888, density, insets, sessions) and `cmd/misagui` (`libmisagui.so`, `RegisterNatives` on `app/misa/Native`). API 26 clang, `-tags nough`. Pure-Go tests run on Linux and in `android-device` |
| A4 APK shell | not started | | |
| A5 Platform services | not started | | |
| A6 Controller/touch/ssh UX | not started | | |
| A7 F-Droid readiness | not started | | |
| CI: android job | done | [#3](https://github.com/AmadeusDE/Misa/pull/3) | NDK r30, `import "C"` guard, misa and ugh on both arches and the tag set, vet with cgo and cgo-less arm64. A3 adds `libmisagui.so` (`-buildmode=c-shared`) for both ABIs |
| CI: android-device job | done | [#3](https://github.com/AmadeusDE/Misa/pull/3) | Path filter, nightly schedule, `workflow_dispatch`. API 34 `aosp_atd` and API 26. APK checks stay with A4 |
| Release: tarball + APK + signing | tarball done | [#3](https://github.com/AmadeusDE/Misa/pull/3) | `android-arm64` and `android-amd64` tarballs (`misa`, `ugh`, no `ughbot`). APK and signing stay with A4 |
