package sshclient

import (
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/crypto/ssh"
)

// sizePoll catches resizes no resize event reports, as when sshc runs
// in-process on a Misa pane's terminal rather than as that terminal's own
// process, and everywhere resize events do not exist.
const sizePoll = 500 * time.Millisecond

// followSize passes local terminal resizes on to the remote PTY until the
// returned stop is called.
func followSize(tty *os.File, session *ssh.Session, w, h int) (stop func()) {
	events, unsubscribe := resizeEvents()
	done := make(chan struct{})
	go watchSize(
		func() (int, int, error) { return term.GetSize(tty.Fd()) },
		func(w, h int) { session.WindowChange(h, w) },
		events, sizePoll, w, h, done,
	)
	return func() {
		unsubscribe()
		close(done)
	}
}

// watchSize reads the size on every event and every poll tick, and calls
// apply when it differs from the last one applied. A nil events channel
// leaves polling as the only trigger.
func watchSize(read func() (int, int, error), apply func(w, h int), events <-chan struct{}, poll time.Duration, w, h int, done <-chan struct{}) {
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-events:
		case <-ticker.C:
		}
		nw, nh, err := read()
		if err != nil || (nw == w && nh == h) {
			continue
		}
		w, h = nw, nh
		apply(w, h)
	}
}
