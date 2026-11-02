//go:build unix

package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/unix"
)

func TestBrowserSpecialImageDoesNotOpenEditor(t *testing.T) {
	m, _, cmd, _ := browserFixture(t)
	path := filepath.Join(m.dir, "pipe.png")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	// Keep a writer available so a regression into the editor's blocking read
	// can be released rather than leaving a stuck test goroutine behind.
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	writer := os.NewFile(uintptr(fd), path)
	defer writer.Close()
	m.reloadFileList()
	selectBrowserName(t, m, "pipe.png")
	done := make(chan struct{})
	go func() {
		m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		writer.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("activation stayed blocked after releasing FIFO")
		}
		t.Fatal("activation tried to read a FIFO")
	}
	if m.screen != screenFileBrowser || *cmd != nil || !strings.Contains(m.status, "regular file") {
		t.Fatalf("special file activation: screen=%v command=%v status=%q", m.screen, *cmd, m.status)
	}
}
