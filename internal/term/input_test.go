package term

import (
	"bytes"
	"testing"
	"time"
)

func TestSendBytesNeverBlocksOnBusyProgram(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv(t.TempDir(), "sh", "-c", "stty raw -echo; sleep 3"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	defer tm.Close()
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	tm.SendBytes(bytes.Repeat([]byte("x"), 256*1024))
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("SendBytes blocked for %v on a program not reading stdin", d)
	}
}

func TestSendBytesKeepsOrder(t *testing.T) {
	tm := New(5, 40)
	br := pipeReplies(t, tm)
	for _, s := range []string{"a", "b", "c"} {
		tm.SendBytes([]byte(s))
	}
	readReply(t, br, "abc")
}
