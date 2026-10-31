//go:build unix

package sshclient

import (
	"os"
	"os/signal"
	"syscall"
)

// resizeEvents delivers a value on SIGWINCH until unsubscribe is called.
func resizeEvents() (events <-chan struct{}, unsubscribe func()) {
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	out := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-winch:
				select {
				case out <- struct{}{}:
				default:
				}
			}
		}
	}()
	return out, func() {
		signal.Stop(winch)
		close(done)
	}
}
