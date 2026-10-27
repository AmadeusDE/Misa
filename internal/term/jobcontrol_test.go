//go:build unix

package term

import (
	"strings"
	"testing"
	"time"
)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Programs alone in their session suspend themselves with SIGSTOP (as Claude
// Code does on ^Z): the kernel drops a terminal's SIGTSTP for them.
func TestStoppedProgramIsSeenAndContinueResumesIt(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv(t.TempDir(), "sh", "-c", "kill -STOP $$; read line; echo got $line"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	defer tm.Close()
	waitFor(t, "stop", tm.Stopped)

	if err := tm.Continue(); err != nil {
		t.Fatal(err)
	}
	if tm.Stopped() {
		t.Error("still stopped after Continue")
	}
	tm.SendBytes([]byte("hello\r"))
	select {
	case <-tm.Exited():
	case <-time.After(5 * time.Second):
		t.Fatal("resumed program never finished")
	}
	if tm.ExitCode() != 0 || !strings.Contains(strings.Join(tm.PlainLines(), "\n"), "got hello") {
		t.Errorf("exit %d, screen %q", tm.ExitCode(), tm.PlainLines())
	}
}

func TestStoppedProgramIsKilledByClose(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv(t.TempDir(), "sh", "-c", "kill -STOP $$; sleep 30"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	waitFor(t, "stop", tm.Stopped)
	done := make(chan struct{})
	go func() { tm.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung on a stopped program")
	}
}
