# Plan: web terminal (`internal/webui`)

`internal/webui` serves a Bubble Tea program in a browser: login and enrollment are HTML, everything else is
terminal cells streamed over server-sent events, with input sent by POST. `misa --web` (`internal/mweb`)
runs Misa in it and listens on loopback by default; ughbot's web UI runs the ughbot TUI and listens on every
interface by default, for a home server reached over a VPN. User docs: `docs/MISA_WEB.md`,
`docs/ughbot/WEB.md`.

## Done

- Enrollment with a single-use TOTP code plus a device ed25519 key generated in the browser; challenge login;
  HttpOnly, SameSite=Strict session cookies; CSRF tokens on mutating requests; CSP with no inline scripts.
- Revocation checked on every request; the terminal of a revoked device closes within the auth-check interval.
- Rate limits per client (IPv6 grouped by /64) for challenges, logins and enrollment; challenges are capped per
  client, so one client can't lock others out.
- The device store is locked across processes (Unix) and saved through a temporary file and rename.
- Control characters are rejected in device names, `next` redirects and admin-terminal output.

## Next

1. **Re-login while a terminal is open (W9).** A terminal session keeps the request that created it
   (`termSession.authReq`) for its periodic auth check. If the user logs in again within the check interval,
   the browser attaches to the old session, which then closes when the old cookie's session is checked.
   Check auth against the cookie of the request currently streaming, or move the session to the new login.
2. **CSRF on the stream (W10).** `GET …/stream` starts a PTY without a CSRF token. SameSite=Strict keeps
   cross-site requests from carrying the cookie, so this is defence in depth: require the token as a query
   parameter, or start the PTY only on the first POST.
3. **HTTPS.** A TLS listener (`[listen] tls`, reserved in ughbot's config; a flag for `misa --web`) with a
   user-supplied certificate. Then set `Secure` on cookies, and use WebCrypto instead of the bundled pure-JS
   ed25519 when the page is in a secure context.
4. **Passkeys.** Once HTTPS exists, WebAuthn as a second way to enroll and log in. The device store already
   keys devices by public key, so a passkey is one more key type.
5. **Push notifications** for new enrollments and pending ughbot approvals (ntfy or Web Push, which also needs
   HTTPS). Today new enrollments show as a banner and in the event log.

## Rules

- `internal/webui` stays generic: it must not import `internal/ui`, `internal/gui` or `internal/ugh`.
- Login and enrollment are the only HTML pages; every other screen is a Bubble Tea program.
- Rate limits key on the connection's remote address, never on forwarded headers.
- Run `go test -race ./internal/webui/ ./internal/mweb/` after changing sessions, limits or the store.
