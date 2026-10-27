package sshclient

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/crypto/ssh"
)

// sizePoll catches resizes no SIGWINCH reports, as when sshc runs in-process
// on a Misa pane's terminal rather than as that terminal's own process.
const sizePoll = 500 * time.Millisecond

// followSize passes local terminal resizes on to the remote PTY until the
// returned stop is called.
func followSize(tty *os.File, session *ssh.Session, w, h int) (stop func()) {
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	ticker := time.NewTicker(sizePoll)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-winch:
			case <-ticker.C:
			}
			nw, nh, err := term.GetSize(tty.Fd())
			if err != nil || (nw == w && nh == h) {
				continue
			}
			w, h = nw, nh
			session.WindowChange(h, w)
		}
	}()
	return func() {
		signal.Stop(winch)
		ticker.Stop()
		close(done)
	}
}
