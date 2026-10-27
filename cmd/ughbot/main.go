// Command ughbot runs the ughbot daemon and talks to it.
package main

import (
	"fmt"
	"io"
	"os"
)

const usageText = `usage: ughbot <command> [options]

  serve [-c config] [--web] [-web-addr host:port]
                               run the daemon (and the web UI)
  init [-d dir]                write a bot-less config template if absent
  bots                         list configured bots
  status                       daemon version, uptime and bot states
  cost [--by bot|day]          spend from the ledger
  send <bot> <text>            queue a message for a bot
  approvals [list [-a]|show <id>|approve <id>|deny <id>]
                               requests from bots that wait on you
  routines [list|add|pause|resume|delete]
                               scheduled mail to bots (ughbot routines -h)
  repos [list [-bot name]|revoke <bot> <dir>]
                               code dirs bots were granted through approvals
  tui                          bots, chat, cost and status in one terminal UI
  attach <bot>                 the same UI, opened on a bot's chat
  devices list|revoke|rename|totp-init|totp-reset|enroll
                               web UI devices and two-factor secret (see docs/ughbot/WEB.md)
  sessions list|kill <prefix>  web UI logins
  vm prepare|status|try [dir]  the throwaway VMs code tasks run their tools in

client commands take -c config and -s socket to find the daemon.
`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ughbot:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return fmt.Errorf("no command")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "serve":
		return serveCmd(rest)
	case "init":
		return initCmd(rest, out)
	case "bots":
		return botsCmd(rest, out)
	case "status":
		return statusCmd(rest, out)
	case "cost":
		return costCmd(rest, out)
	case "send":
		return sendCmd(rest, out)
	case "approvals":
		return approvalsCmd(rest, out)
	case "routines":
		return routinesCmd(rest, out)
	case "repos":
		return reposCmd(rest, out)
	case "tui":
		return tuiCmd(rest)
	case "attach":
		return attachCmd(rest)
	case "devices":
		return devicesCmd(rest, out, os.Stdin)
	case "sessions":
		return sessionsCmd(rest, out)
	case "vm":
		return vmCmd(rest, out)
	case "vm-agent":
		return vmAgentCmd()
	case "help", "-h", "--help":
		fmt.Fprint(out, usageText)
		return nil
	}
	fmt.Fprint(os.Stderr, usageText)
	return fmt.Errorf("unknown command %q", cmd)
}
