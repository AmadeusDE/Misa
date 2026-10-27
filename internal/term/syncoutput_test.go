package term

import (
	"strings"
	"testing"
	"time"
)

func TestOutputBeforeSyncBeginCountsAsApplied(t *testing.T) {
	tm := New(2, 20)
	if !tm.feedSync([]byte("hello\x1b[?2026h")) {
		t.Error("output before a sync begin was applied but not reported")
	}
	if tm.feedSync([]byte("frame")) {
		t.Error("output inside a sync block reported before the block ends")
	}
	if !tm.feedSync([]byte("\x1b[?2026l")) {
		t.Error("a finished frame was not reported")
	}
}

func TestUnfinishedSyncFrameIsShownAfterATimeout(t *testing.T) {
	defer func(d time.Duration) { syncTimeout = d }(syncTimeout)
	syncTimeout = 20 * time.Millisecond
	tm := New(2, 20)
	if tm.processChunk([]byte("\x1b[?2026hprompt> ")) {
		t.Fatal("an open frame was reported at once")
	}
	select {
	case <-tm.Dirty():
	case <-time.After(2 * time.Second):
		t.Fatal("an open frame was never shown")
	}
	if got := tm.PlainLines()[0]; !strings.HasPrefix(got, "prompt>") {
		t.Fatalf("screen = %q", got)
	}
	if tm.processChunk([]byte("more")); !strings.HasPrefix(tm.PlainLines()[0], "prompt> more") {
		t.Fatalf("output after the timeout was held back: %q", tm.PlainLines()[0])
	}
}

func TestFinishedSyncFrameCancelsItsTimeout(t *testing.T) {
	defer func(d time.Duration) { syncTimeout = d }(syncTimeout)
	syncTimeout = 20 * time.Millisecond
	tm := New(2, 20)
	tm.processChunk([]byte("\x1b[?2026hone\x1b[?2026l\x1b[?2026htwo"))
	time.Sleep(5 * time.Millisecond)
	tm.processChunk([]byte("\x1b[?2026l"))
	time.Sleep(60 * time.Millisecond)
	if got := tm.PlainLines()[0]; !strings.HasPrefix(got, "onetwo") {
		t.Fatalf("screen = %q", got)
	}
}
