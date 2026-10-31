//go:build unix

package sshclient

import (
	"syscall"
	"testing"
	"time"
)

func TestResizeEventsFireOnSIGWINCH(t *testing.T) {
	events, unsubscribe := resizeEvents()
	defer unsubscribe()
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("no event after SIGWINCH")
	}
}
