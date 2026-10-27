package webui

import (
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/term"
)

func TestTitleUpdatesDoNotDeadlockInput(t *testing.T) {
	tm := term.New(10, 40)
	if err := tm.StartArgv(t.TempDir(), "sh", "-c", `while :; do printf '\033]2;busy\007'; done`); err != nil {
		t.Fatal(err)
	}
	ts := &termSession{t: tm, nextSeq: 1, wake: make(chan struct{}, 1)}
	tm.SetOSCHook(ts.oscTitle)
	tm.StartReader()
	t.Cleanup(tm.Close)

	done := make(chan struct{})
	go func() {
		defer close(done)
		stop := time.Now().Add(300 * time.Millisecond)
		for seq := uint64(1); time.Now().Before(stop); seq++ {
			rows := 10 + int(seq%5)
			ts.apply(inputBatch{Seq: seq, Events: []inputEvent{{Type: "resize", Rows: rows, Cols: 40}}})
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("input and title updates deadlocked")
	}
	if got := ts.currentTitle(); got != "busy" {
		t.Errorf("title = %q, want busy", got)
	}
}
