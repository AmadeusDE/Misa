// Package sshclient is sshc, Misa's built-in SSH client: an interactive
// shell or a remote command over golang.org/x/crypto/ssh, configured from
// ~/.ssh/config and checked against ~/.ssh/known_hosts like OpenSSH.
package sshclient

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"

	"github.com/charmbracelet/x/term"
	"golang.org/x/crypto/ssh"

	"github.com/AmadeusDE/Misa/internal/hostpath"
)

const exitSSHError = 255

// env is what sshc takes from the system, so tests can supply their own.
type env struct {
	Home             string
	LocalUser        string
	SystemConfig     string
	SystemKnownHosts []string
	AgentSocket      string
	Term             string
	Stdin            io.Reader
	Stdout, Stderr   io.Writer
	Prompt           prompter // nil: ask on Stdin when it is a terminal
}

// Run connects as the command line says and returns OpenSSH's exit status:
// the remote command's, or 255 when sshc itself fails.
func Run(o Options, termName string, stdin io.Reader, stdout, stderr io.Writer) int {
	e := env{
		SystemConfig:     "/etc/ssh/ssh_config",
		SystemKnownHosts: []string{"/etc/ssh/ssh_known_hosts"},
		AgentSocket:      os.Getenv("SSH_AUTH_SOCK"),
		Term:             termName,
		Stdin:            stdin,
		Stdout:           stdout,
		Stderr:           stderr,
	}
	e.Home, _ = os.UserHomeDir()
	if u, err := user.Current(); err == nil {
		e.LocalUser = hostpath.AccountName(u.Username)
	} else {
		e.LocalUser = hostpath.Username()
	}
	return run(o, e)
}

func run(o Options, e env) int {
	fail := func(format string, args ...any) int {
		fmt.Fprintf(e.Stderr, "sshc: "+format+"\n", args...)
		return exitSSHError
	}
	t, err := resolve(o, e)
	if err != nil {
		return fail("%v", err)
	}
	tty := terminalOf(e.Stdin)
	p := e.Prompt
	switch {
	case p != nil:
	case tty != nil:
		p = ttyPrompter{tty: tty, out: e.Stderr}
	default:
		p = noPrompter{}
	}

	keys, err := newHostKeys(append(t.KnownHosts, e.SystemKnownHosts...), t.Strict, p, e.Stderr)
	if err != nil {
		return fail("known hosts: %v", err)
	}
	methods, closeAgent := authMethods(t, e.AgentSocket, p, e.Stderr)
	defer closeAgent()

	client, err := ssh.Dial("tcp", t.Addr, &ssh.ClientConfig{
		User:              t.User,
		Auth:              methods,
		HostKeyCallback:   keys.Callback,
		HostKeyAlgorithms: keys.Algorithms(t.Addr),
	})
	if err != nil {
		return fail("connect to host %s: %v", t.Alias, err)
	}
	defer client.Close()

	wantTTY := o.ForceTTY || (len(o.Command) == 0 && !o.NoTTY)
	if wantTTY && tty == nil {
		fmt.Fprintln(e.Stderr, "Pseudo-terminal will not be allocated because stdin is not a terminal.")
		wantTTY = false
	}
	if !wantTTY {
		tty = nil
	}
	code, err := runSession(client, o.Command, tty, e)
	switch {
	case err != nil:
		fail("%v", err)
	case tty != nil:
		fmt.Fprintf(e.Stderr, "Connection to %s closed.\n", t.Alias)
	}
	return code
}

func terminalOf(r io.Reader) *os.File {
	if f, ok := r.(*os.File); ok && term.IsTerminal(f.Fd()) {
		return f
	}
	return nil
}

// runSession runs the remote shell (no command) or command, on a remote PTY
// when tty is the local terminal. The local terminal is raw until it ends.
func runSession(client *ssh.Client, command []string, tty *os.File, e env) (int, error) {
	session, err := client.NewSession()
	if err != nil {
		return exitSSHError, err
	}
	defer session.Close()
	session.Stdout, session.Stderr = e.Stdout, e.Stderr
	session.Stdin = e.Stdin

	disconnected := false
	if tty != nil {
		w, h, err := term.GetSize(tty.Fd())
		if err != nil {
			w, h = 80, 24
		}
		termName := e.Term
		if termName == "" {
			termName = "xterm-256color"
		}
		modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}
		if err := session.RequestPty(termName, h, w, modes); err != nil {
			return exitSSHError, fmt.Errorf("PTY allocation request failed: %w", err)
		}
		state, err := term.MakeRaw(tty.Fd())
		if err != nil {
			return exitSSHError, err
		}
		defer term.Restore(tty.Fd(), state)
		defer followSize(tty, session, w, h)()
		session.Stdin = &escapeReader{r: e.Stdin, atLineStart: true, disconnect: func() {
			disconnected = true
			client.Close()
		}}
	}

	if len(command) == 0 {
		err = session.Shell()
	} else {
		err = session.Start(strings.Join(command, " "))
	}
	if err != nil {
		return exitSSHError, err
	}
	err = session.Wait()

	var exit *ssh.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exit) && exit.Signal() == "":
		return exit.ExitStatus(), nil
	case errors.As(err, &exit):
		return exitSSHError, fmt.Errorf("remote command killed by signal %s", exit.Signal())
	case disconnected:
		return exitSSHError, nil
	}
	return exitSSHError, err
}
