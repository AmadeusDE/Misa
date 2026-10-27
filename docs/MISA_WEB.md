# misa --web

`misa --web` serves Misa in a browser. It is the same web shell as the ughbot daemon
(`internal/webui`: TOTP enrollment, ed25519 device login, sessions, the browser terminal), with a
terminal that runs `misa` itself and so inherits your `config.yaml`. The glue is `internal/mweb`;
`internal/ui` never imports the web code. Build without it with `-tags noweb`.

**A login is a shell on this host**, as the user who ran `misa --web`. Bind it to a private
interface (WireGuard), keep the TOTP secret and the state directory private, and revoke lost
devices. `ughbot`'s web UI runs only its own TUI for this reason.

## Run

```sh
misa --web --web-addr 127.0.0.1:8787     # development: a fixed address
misa --web --web-interface wg0           # production: only wg0's address, port 8787
misa --web --web-interface wg0 --web-port 9000
```

With no address or interface (on the command line or in `config.yaml`) it listens on `127.0.0.1:8787`
(or `--web-port`), reachable only from the same machine; name an address or interface to serve other
devices. An address on the command line replaces a
configured interface and vice versa. Defaults can live in `config.yaml`:

```yaml
web:
  interface: wg0     # or addr: 127.0.0.1:8787
  port: 8787
  command: [misa]    # optional argv for the terminal; default is misa itself
```

The terminal sets `MISA_WEB_CHILD=1`, and `misa --web` refuses to run when it is set, so a pane
cannot start another server. `/` redirects to `/term/`.

## Keyboard in the browser

The page forwards every chord a browser lets a page see, including Ctrl+A..Z, Alt+letters, Alt+arrows,
F1-F10, Ctrl+Space, Ctrl+`/` `\` `[` `]` `-`. Browsers keep some chords for themselves (Ctrl+N/T/W,
Ctrl+Tab, Ctrl+PageUp/PageDown, Ctrl+1..9, F11, F12, Ctrl+Shift+C/V/I/J, Cmd/Win chords), so the Bubble Tea
UIs do not bind them. There is no toolbar or key remapping.
Soft keyboards on Android only deliver composition events, so typing there is not supported yet.

## Files

| What | Where |
| --- | --- |
| device store | `$XDG_STATE_HOME/misa/webui` (default `~/.local/state/misa/webui`) |
| `authorized_keys`, CLI device key `web-device.key` | `$XDG_CONFIG_HOME/misa` (default `~/.config/misa`) |

Both directories are created 0700.

## Enroll and recover

```sh
misa --web-admin totp-init                 # prints a QR, the otpauth URI and the secret
misa --web-admin devices enroll [-name n] http://<addr>:8787
misa --web-admin devices list
misa --web-admin devices revoke <id|name>
misa --web-admin devices rename <id|name> <new>
misa --web-admin totp-reset                # lost authenticator
misa --web-admin sessions list
misa --web-admin sessions kill <prefix>
```

Inside a Misa pane (a build without `-tags noweb`) the `devices` command opens the device screen
on the same store: devices, sessions, event log and enrollment notices, with `r` rename, `x` revoke
(confirmed), `k` end a session, `d` dismiss a notice, `R` refresh, `q` leave. `devices list|revoke|
rename|enroll ...` with arguments runs the `--web-admin devices` commands. The browser has no
device page; it only shows login, enroll and the terminal.

Browsers enroll at `/enroll` with the six-digit code. The admin commands edit the store directly,
need no login, and work whether or not the server is running. Details of the login flow, keys in
`authorized_keys` and why there is no HTTPS are in `docs/ughbot/WEB.md`.
