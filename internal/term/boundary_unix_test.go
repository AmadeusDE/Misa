//go:build unix

package term

import (
	"strings"
	"testing"
	"time"
)

func TestResizeReachesTheProgramsTTY(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "stty size; read x; stty size"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	defer tm.Close()
	waitFor(t, "first size", func() bool { return strings.Contains(screenText(tm), "5 40") })
	tm.Resize(7, 33)
	tm.SendBytes([]byte("\r"))
	waitFor(t, "resized size", func() bool { return strings.Contains(screenText(tm), "7 33") })
}

func TestCloseIsIdempotent(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "sleep 30"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	tm.Close()
	tm.Close()
	waitExited(t, tm)
	if !tm.Done() {
		t.Error("not done after Close")
	}
	tm.SendBytes([]byte("late")) // must not panic or block
}

func TestCloseKillsGrandchildInTheProcessGroup(t *testing.T) {
	tm := New(5, 40)
	if err := tm.StartArgv("", testBin(t, "sh"), "-c", "(sleep 300; true) & echo ok; wait"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	waitFor(t, "output", func() bool { return strings.Contains(screenText(tm), "ok") })
	done := make(chan struct{})
	go func() { tm.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung")
	}
	waitExited(t, tm)
}

func TestInputOrderIsPreserved(t *testing.T) {
	tm := New(5, 80)
	if err := tm.StartArgv("", testBin(t, "cat")); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	defer tm.Close()
	tm.SendBytes([]byte("one "))
	tm.SendBytes([]byte("two "))
	tm.SendBytes([]byte("three\r"))
	waitFor(t, "echo", func() bool { return strings.Contains(screenText(tm), "one two three") })
}
