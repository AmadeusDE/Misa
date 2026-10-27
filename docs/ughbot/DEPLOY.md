# Deploying ughbot

ughbot is a single daemon (`ughbot serve`) plus a small CLI that talks to it over a unix socket.
Everything below runs as an ordinary user; nothing needs root.

## Build and first run

```sh
go build -o ughbot ./cmd/ughbot
install -m 755 ughbot ~/.local/bin/ughbot
ughbot init                 # writes a bot-less ~/.config/ughbot/ughbot.toml if absent
$EDITOR ~/.config/ughbot/ughbot.toml   # optional: web UI, paths, or bots as [bots.NAME] tables
```

`init` never overwrites: files that exist are reported as `kept`. The template defines no bots; most setups create
them in the TUI instead (see Creating bots). `state_dir` (default `~/.local/state/ughbot`) holds the SQLite database
and one session directory per bot, so threads resume when the daemon restarts. Delegation (`spawn`, `council`) is
disabled in bots; `tasks` and `manage` cover handing work off.

Calls the permission classifiers flag (for example a risky `bash` command) are answered by the unattended policy in
the bot's ugh config (`headless_prompt`, default deny) and by the allow rules under `allow.toml` in the config
directory; there is no human to ask.

## Credentials

**OpenRouter.** Put the key in an env file only the user can read:

```sh
install -d -m 700 ~/.config/ughbot
printf 'OPENROUTER_API_KEY=sk-or-...\n' > ~/.config/ughbot/env
chmod 600 ~/.config/ughbot/env
```

Set an account-level spending cap on the OpenRouter key as well. ughbot's ledger refuses requests
that would pass a bot's daily or monthly cap, but the provider-side cap is the backstop if the
daemon, its database or the config is ever wrong.

**Codex (ChatGPT subscription).** Bots run with their own config root, so their credentials live in
`~/.config/ughbot/credentials.json` (a file store, chosen automatically for an embedded instance;
`[auth] store = "file"` in the ugh config pins it for the user's own ugh). Sign in with ugh's
device-code flow, which needs no browser on the server, using the file store, then give the daemon
a copy:

```sh
ugh login codex --device        # prints a URL and a code; approve it from any browser
install -m 600 ~/.config/ugh/credentials.json ~/.config/ughbot/credentials.json
```

(If your ugh uses the desktop keyring, set `[auth] store = "file"` in `~/.config/ugh/config.toml`
before logging in so the credentials land in that file.)

Codex requests carry no per-token price; the ledger passes them through at $0 and only refuses a
bot that is already over a cap from other spend.

## systemd user unit

`~/.config/systemd/user/ughbot.service`:

```ini
[Unit]
Description=ughbot daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%h/.config/ughbot/env
ExecStart=%h/.local/bin/ughbot serve -c %h/.config/ughbot/ughbot.toml
Restart=on-failure
RestartSec=5
TimeoutStopSec=20

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now ughbot
loginctl enable-linger "$USER"      # keep it running when you are logged out
journalctl --user -u ughbot -f
```

The socket is `$XDG_RUNTIME_DIR/ughbot.sock` (mode 0600). A second `ughbot serve` on the same
socket refuses to start. On SIGTERM (`systemctl --user stop`) or SIGINT the daemon shuts down cleanly:
mail a bot had claimed but not finished returns to its queue and is delivered on the next start.

## Using it

```sh
ughbot status              # version, uptime, bot states
ughbot bots
ughbot send <bot> "summarise yesterday"
ughbot approvals           # requests from bots waiting on you; show/approve/deny <id>, -a lists decided ones too
ughbot routines            # scheduled mail to bots; add -bot B -cron "0 9 * * 1" prompt..., pause|resume|delete <id>
ughbot vm prepare|status|try   # the VMs code tasks run their tools in (see Code tasks)
ughbot repos               # code dirs bots were granted through approvals; revoke <bot> <dir>
ughbot tui                 # bots, chat, approvals, routines, repos, cost, status, devices; tab, [ ] or 1-9 switch screens (ctrl+o inside the chat), ? help, ctrl+c leaves
ughbot attach <bot>        # the same UI opened on that bot's chat; works inside a Misa pane
ughbot cost --by bot       # settled spend from the ledger (also --by day)
```

The TUI binds no browser-reserved chords (ctrl+n/t/w, ctrl+tab, ctrl+pgup/pgdn, ctrl+1-9, F5, F11,
F12, alt+left/right), so it behaves the same in the web terminal; the list is tested in
`internal/ughbot/tui/browserreserved_test.go`. Scroll the chat with pgup/pgdn.

`tui` and `attach` reconnect on their own and resumes from the last event it saw.

## Creating bots

`ughbot serve` starts with no bots. Open `ughbot tui` and go to the Bots screen:

- `p` opens the form filled from the **bot-manager preset**: a bot that designs and creates the other bots. It holds
  `create_bots` and may message and manage every bot, has no files or shell, and starts with $1/day and $20/month
  caps. You pick its name and model (left blank on purpose) and review everything before `ctrl+s`; nothing exists
  until you save. Talk to it in the chat and it proposes bots, each of which waits for your approval.
- `n` opens a blank form.

Bots made there are stored in `ughbot.db`, start immediately and can be edited (`e`) or deleted (`x`) without a
restart. Bots defined as `[bots.NAME]` in `ughbot.toml` show as `(toml)` and are changed in that file; a name used by
both is an error.

### Capabilities

A bot gets the tools of the capabilities it holds and nothing else (the Bots list shows them under each bot):

| capability    | tools                                   |
|---------------|-----------------------------------------|
| `files`       | `"read"`: read, ls, grep, find. `"write"`: also write, edit |
| `shell`       | bash                                    |
| `tasks = N`   | start_task: N separate task sessions at once, each with a slice of the bot's budget; results come back as mail |
| `message`     | message_bot, ask_bot, list_bots, for the bots named (`"*"` for all) |
| `manage`      | manage_bot (status, steer, interrupt, start_task as that bot) and list_bots, for the bots named |
| `create_bots` | create_bot, update_bot, delete_bot, show_bot; **every request waits in the approval queue** |
| `schedule`    | schedule_add and schedule_cancel act directly (see Routines); without it they still work, through approval |
| `code = [dirs]` | code_task, code_tasks, code_steer: ugh coding sessions in those directories, tools in a VM (see Code tasks) |

Every bot can `reply` to a question another bot asked it, and has schedule_add, schedule_list and schedule_cancel. Capabilities are checked again on every call, so a model
that names a tool it was not given gets a refusal. Memory and prompts can never add a capability. A bot that deals
with other bots has `You are the bot named "NAME"` appended to its prompt.

Bots run with ugh's permission settings from `config.toml` in the ughbot config dir (default: classify mode). With
nobody attached, a call the classifiers flag is answered by `[permissions] headless_prompt` (deny by default).

### Approvals

`create_bot`, `update_bot` and `delete_bot` never act directly. They are checked against the current bots, then stored
with a SHA-256 of the exact action; approving re-checks that hash and runs exactly what was stored. One decision
wins, a request nobody answers expires after 72 hours and counts as denied, and the asking bot gets the outcome as mail
from `approval #N`. Decide in the TUI's Approvals screen (the header shows how many are pending; `enter` shows the full
request including the prompt, `a`/`d` then `y`) or with `ughbot approvals approve|deny <id>`. The record names who
decided: `cli`, `tui`, or `web:<device>` from the browser terminal.

## Routines

A routine mails a prompt to a bot, as mail from `routine #N`, either on a 5-field cron spec (`0 9 * * 1`, names like
`mon`/`jan`, ranges, lists, steps, and `@hourly`/`@daily`/`@weekly`/`@monthly`/`@yearly`) in a timezone (default:
the daemon's), or once at a time (`in 2h`, `2026-10-20 09:00`, RFC 3339). The bot then does the work with its own tools
and budget.

- Make them in the TUI Routines screen (`n`; the form shows when the routine first fires, or what is wrong with the
  time), with `ughbot routines add`, or let bots make them with `schedule_add` for themselves or bots they may message.
- A bot with the `schedule` capability adds and cancels directly. If the permission classifiers flag such a call, it
  is not denied but deferred to the approval queue as the exact stored call, and runs as stored once approved. A bot
  without the capability can still use the tools, but every add or cancel is an approval request.
- Missed fires (the daemon was down): `missed = once` (default) fires once when it comes back, never once per missed
  time; `missed = skip` waits for the next time. Firing and its mail are one database transaction, so a restart never
  fires a routine twice or loses a fire.
- Pausing and resuming continue from now without catching up. A routine whose bot was deleted pauses itself. A
  one-shot routine is `done` after it fires.

## Code tasks

A bot with `code = ["/path/to/repo", ...]` can start a **code task**: a full ugh session (subagents, git worktrees,
council) on the bot's model whose tools (bash, read, write, edit, ls, grep, find, and git) run in a throwaway
Linux VM. The agent itself stays in the daemon, so the ledger, the bot's caps and the session files stay on the host,
and API keys never enter the VM.

- **One-time setup.** It needs `qemu-system-x86_64` and `qemu-img`, and `/dev/kvm` for speed (without it qemu
  emulates, which is slow). Run `ughbot vm prepare` (a few minutes) to download the image in `[vm] image` (default:
  the Arch Linux cloud image), install `[vm] packages` and run `[vm] setup`, and keep the result as the base in
  `state_dir/vm`. Run it again whenever you change those settings; running VMs keep the base they started from.
  `ughbot vm try ~/src/repo` boots one, runs a few checks in the shared directory, and throws it away.
- **What the VM sees.** Only the task's directory (mounted over 9p at the same path, so its edits land in your
  real files, owned by you) and the task's worktree directory. Nothing else from the host: not your home directory,
  `~/.ssh`, or ughbot's state. Each task boots a fresh copy-on-write overlay of the base and deletes it at the end.
  Internet is on unless `[vm] network = false`. Commits made in it use your git identity from that repository.
- **Which directories.** `code` entries must be absolute. ughbot refuses your home directory or anything above it,
  and anything in or around its state dir, its config dir or `~/.ssh`. One code task works in a directory at a time,
  across all bots.
- **Talking to it.** `code_task(dir, prompt)` returns at once. The session's questions (ask_user) and any call
  the permission classifiers flag reach the bot as `[question #N from code:c1]`, and the bot answers with
  `reply`; for a flagged call, "allow" lets it run once and anything else denies it. `code_steer` sends a message
  or stops the task; `code_tasks` lists them. The final report arrives as mail from `code:c1` with the branch,
  anything left uncommitted, and any worktrees still present.
- **Budget.** The bot never sets one. A code task and all its subagents spend from the bot's daily and monthly
  caps, recorded as task `code:c1`.
- A daemon restart ends running code tasks, and the VMs are cleaned up on the next start.

### Asking for a repo

A bot can ask for a code directory with `request_repo`, for itself or for a bot it manages (so a manager can
equip the bot that runs the code tasks). Bots with `code` dirs or a `manage` list get the tool. Every request
waits in the approval queue; nothing is granted or cloned before you approve it.

- `dir`: a directory already on this host. It gets the same checks as a `code` entry and must exist.
- `url`: a git repository (https, http, ssh, git, or `user@host:path`; never `file://` or local paths) to clone
  into `state_dir/repos/<name>`, where the name is the request's `name` or the URL's last element. The approval
  shows the exact destination. The clone runs on the host as you, with your git credentials and ssh agent, without
  submodules or terminal prompts, starts once you approve, and its outcome arrives as mail (and in the approval's
  result). Clones under `state_dir/repos` are the one place inside the state dir a code directory may be.
- Granted directories live in their own table, not in the bot's definition, so bots from ughbot.toml can get them
  too. `ughbot repos` and the TUI Repos screen list them; `ughbot repos revoke <bot> <dir>` (or `x` there) takes
  one back and stops the bot's code tasks in it. Files, including clones, stay on disk.
- A bot that had no code dirs is restarted when its first one is granted, to get the code tools; that ends its
  current turn. When a manager asked, the bot that got the directory is told by mail.
