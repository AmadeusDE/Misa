# ughbot web UI

The browser is a terminal. The daemon serves the login and enroll pages (HTML, they precede
auth) and the page that hosts the terminal and a generic browser terminal (`internal/webui`) that runs a Bubble Tea program in a PTY,
streams its screen cells as SSE frames (full frame, then per-row damage) and sends back key,
mouse, paste and resize input, encoded server-side. By default the program is the ughbot TUI
(`ughbot tui -c <config> -s <socket>`): bot picker, chat, cost, status and device management, the same as in your
terminal. There are no ughbot-specific HTML pages; every future UI only targets Bubble Tea.

`/` redirects to `/term/`. Each enrolled device gets one terminal that survives a dropped stream
for about a minute and ends with the program, the login, the device or the daemon. At most 8 run
at once.

**Status of testing.** The Go side (login, enrollment, CSRF, the terminal stream and input, CLI)
is covered by tests, including an end-to-end test that drives a real listener, spawns a built
`ughbot tui` and checks the bot picker and chat screens in the streamed frames. The browser
JavaScript (login with IndexedDB, the pure-JS ed25519, the cell painter) has not been run in a
real browser; expect rough edges the first time you use it from Firefox or Chrome.

## Keyboard in the browser

The page forwards every chord a browser lets a page see, including Ctrl+A..Z, Alt+letters, Alt+arrows,
F1-F10, Ctrl+Space, Ctrl+`/` `\` `[` `]` `-`. Browsers keep some chords for themselves (Ctrl+N/T/W,
Ctrl+Tab, Ctrl+PageUp/PageDown, Ctrl+1..9, F11, F12, Ctrl+Shift+C/V/I/J, Cmd/Win chords), so the Bubble Tea
UIs do not bind them. There is no toolbar or key remapping.
Soft keyboards on Android only deliver composition events, so typing there is not supported yet.

## What runs

`[web] command` is an optional argv array; empty means the ughbot TUI from the daemon's own
binary.

```toml
[web]
command = ["ughbot", "tui", "-c", "/etc/ughbot.toml"]   # optional; default is the TUI
```

**Security note.** The default runs only the ughbot TUI, which can talk to the bots and nothing
else. Setting `command = ["misa"]` (or any shell) gives every enrolled device a shell on the
host as the daemon's user, so a stolen device or TOTP secret becomes remote code execution.
That is why the default is the TUI. If you want a shell in the browser, prefer `misa --web`
(see `docs/MISA_WEB.md`) on a separate account or host instead of widening the ughbot daemon.

## Enable

In `ughbot.toml` (or just pass flags):

```toml
[web]
enabled          = true
listen_interface = "wg0"            # production: bind only this interface's address
port             = 8787             # default
# listen_addr    = "127.0.0.1:8787" # local testing; ignored when listen_interface is set
```

With neither `listen_interface` nor `listen_addr` set, the server listens on `0.0.0.0:<port>` (every interface, LAN included; plain HTTP, so keep it off public networks). Startup fails if a named interface has no address. Flags: `ughbot serve --web` enables, `-web-addr host:port` enables and
overrides the config (the interface is ignored). Quick local trial:

```sh
ughbot serve -web-addr 127.0.0.1:8787
```

The TUI is spawned from the running `ughbot` binary, so keep it in place while the daemon runs.
On start it prints the URL and, if no TOTP secret exists, a reminder to create one.

State: device store at `<state_dir>/webui/webui.json` (0600); authorized_keys file at
`<config_dir>/authorized_keys`; CLI device key at `<config_dir>/web-device.key`. The admin
commands edit the store directly and the running daemon notices the change on its next request.

## Enroll a device

1. Create the one-time secret: `ughbot devices totp-init`. It prints an `otpauth://` URI and the
   base32 secret; add either to an authenticator app.
2. Enroll:
   - **Desktop browser**: open `http://<address>:8787/enroll`, enter the six-digit code and a
     device name. The browser generates an ed25519 key, keeps the seed in IndexedDB and uses it
     to sign login challenges from then on. Clearing site data removes the device; enroll again.
   - **Android over WireGuard**: bring the tunnel up, browse to the server's WireGuard address
     (`http://10.x.y.z:8787/enroll`) and do the same. Add the page to the home screen if you like.
   - **CLI / a Misa pane**: `ughbot devices enroll http://<address>:8787` prompts for the code,
     creates `web-device.key` in the config dir if absent, enrolls and verifies a login.
     `-name` sets the device name (default: hostname).
3. Optional: list `ssh-ed25519` public keys in `<config_dir>/authorized_keys`; their holders can
   log in through the signed challenge without TOTP (they appear as device `ak-...`). The file is
   re-read on every login.

## Managing devices from the TUI

Device management is a screen of the ughbot TUI (`Devices`, the last tab; press its number or
`tab` to it), not a web page. It lists devices (name, id, created, last seen, revoked), active
sessions, the event log and a banner for pending enrollment notices. Keys: arrows or `j`/`k`
move, left/right switch devices/sessions/events, `r` rename, `x` revoke (asks `y`; also ends the
device's sessions and its browser terminal), `k` on the sessions view ends a session, `d` dismisses
the first notice, `R` refreshes. It is the shared `internal/webdevtui` component; Misa opens the
same one as the `devices` command (see `docs/MISA_WEB.md`).

The TUI reaches the store through the daemon's unix socket (`/v1/web/*`, no network login). When the
daemon runs without the web UI those endpoints answer "web UI not enabled" and the screen tells you
to start `ughbot serve -web` or set `[web] enabled = true`. Over the web terminal the screen is
reachable by any enrolled device, so an enrolled device can revoke devices; the HTML pages no
longer expose any administration.

## Recovery and maintenance (shell on the server only)

```sh
ughbot devices list
ughbot devices revoke <id|name>        # also ends that device's sessions
ughbot devices rename <id|name> <new>
ughbot devices totp-reset              # lost authenticator; devices and sessions are untouched
ughbot sessions list
ughbot sessions kill <prefix>          # at least 6 characters of the session id
```

Pass `-c config` right after the command name (`ughbot devices -c path list`). These commands do
no network and no login, so shell access to the state directory is the root of trust. The server and
the admin commands lock the store (`webui.json.lock`) around every change, so they can run together.

## Why no HTTPS, and why pure-JS ed25519

The intended deployment is a private WireGuard interface, where the tunnel already authenticates
and encrypts. Browsers expose WebCrypto (`crypto.subtle`) and WebAuthn only in secure contexts,
and plain `http://10.x.y.z` is not one. So the page ships a small pure-JS ed25519 for signing the
login challenge and keeps the seed in IndexedDB. Sessions use an HttpOnly cookie, a per-session
CSRF token (`X-CSRF-Token`, taken from the `csrf-token` meta tag), and a CSP of
`default-src 'self'` (no inline scripts). Binding to a non-private interface over plain HTTP is
not supported.

**Passkeys / WebAuthn are pinned** until HTTPS exists. Adding a TLS listener (`[listen] tls`,
reserved) is the planned path; the device store and login flow would not change.
