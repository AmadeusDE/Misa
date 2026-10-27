# Security

## Reporting

Please report vulnerabilities privately through GitHub's
[private vulnerability reporting](https://github.com/AmadeusDE/Misa/security/advisories/new) for this
repository, not in public issues. Include what you found, how to reproduce it, and what an attacker gains.

## What counts

The parts of Misa that handle untrusted input are the most interesting:

- **The terminal emulator** (`internal/term`): output from any program a user runs is untrusted. A byte
  sequence that crashes Misa, holds memory without bound, or affects other panes or the host terminal is a bug.
- **The web terminal** (`internal/webui`, `misa --web`, the ughbot web UI): authentication (device keys
  plus TOTP enrollment), sessions, rate limits, and anything that reaches the shell without a login.
- **ugh's permission system** (`internal/ugh/perm`): a way to make a tool call run without being judged,
  judged on different values than it runs with, or judged on a path other than the one it touches.
- **Configuration and instruction files** read from places a cloned repository controls.
- **sshc** (`internal/sshclient`): host key checking and anything a server can do to the local terminal.

## Known limits

- The web terminal speaks plain HTTP. Keep it on loopback, a VPN such as WireGuard, or behind a TLS proxy.
  `misa --web` listens on `127.0.0.1` unless told otherwise; ughbot's web UI listens on every interface by
  default, as it is meant for a home server.
- ugh's model classifiers judge intent and can be wrong; the static rules and the user prompt are the
  backstop. With no working classifier, flagged calls go to the user (or are refused when headless).
- ughbot is experimental and should not be exposed to untrusted networks.
