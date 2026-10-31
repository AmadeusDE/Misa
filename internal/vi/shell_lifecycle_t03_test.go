package vi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// A controllable child, not a host sleep executable. Its process is killed by
// cancellation; the marker environment variable prevents accidental execution.
func TestT03BlockingChild(t *testing.T) {
	if os.Getenv("MISA_T03_CHILD") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("MISA_T03_READY"), []byte("ready"), 0600); err != nil {
		os.Exit(2)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestT03CancelAndResizeWhileRunning(t *testing.T) {
	t03Shell(t)
	t.Setenv("MISA_T03_CHILD", "1")
	ready := filepath.Join(t.TempDir(), "ready")
	t.Setenv("MISA_T03_READY", ready)
	e, _ := f07Editor(t, "one\ntwo\n")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(binary, "'", "'\\''") + "'"
	var cmd tea.Cmd
	for _, key := range replayKeys(t, ":%!"+quoted+" -test.run=^TestT03BlockingChild$<CR>") {
		cmd = e.Update(key)
	}
	if cmd == nil {
		t.Fatal("shell command did not return an asynchronous command")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	// Do not cancel until the child is actually running. Always cancel on a
	// readiness failure too, so its process cannot leak into later tests.
	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
readyLoop:
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case msg := <-done:
			e.Update(msg)
			t.Fatal("child exited before readiness")
		case <-deadline:
			e.Update(keyFromName("ctrl+c"))
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
			t.Fatal("child did not become ready")
		case <-tick.C:
			continue readyLoop
		}
	}
	e.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if v := e.View(); v == "" {
		t.Error("empty view while shell runs")
	}
	e.Update(keyFromName("ctrl+c"))
	select {
	case msg := <-done:
		e.Update(msg)
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl-C did not cancel shell")
	}
	if got := strings.Join(e.lines, "\n"); got != "one\ntwo" {
		t.Errorf("cancelled filter changed buffer: %q", got)
	}
}
