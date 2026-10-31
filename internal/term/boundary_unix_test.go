//go:build unix

package term

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// sizeHelperEnv makes the test binary act as a tiny program that prints its
// tty's size, waits for a line, and prints it again. stty is not on every
// target (Android 8's toybox has none; it arrived in Android 9), so the test
// re-executes itself instead of relying on the device's shell utilities.
const sizeHelperEnv = "MISA_TERM_SIZE_HELPER"

func TestSizeHelperProcess(t *testing.T) {
	if os.Getenv(sizeHelperEnv) != "1" {
		t.Skip("helper process for TestResizeReachesTheProgramsTTY")
	}
	printSize := func() {
		ws, err := unix.IoctlGetWinsize(0, unix.TIOCGWINSZ)
		if err != nil {
			fmt.Printf("winsize: %v\r\n", err)
			os.Exit(2)
		}
		fmt.Printf("%d %d\r\n", ws.Row, ws.Col)
	}
	printSize()
	bufio.NewReader(os.Stdin).ReadString('\n')
	printSize()
	os.Exit(0)
}

func TestResizeReachesTheProgramsTTY(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Skip("test binary path unavailable: ", err)
	}
	t.Setenv(sizeHelperEnv, "1")
	tm := New(5, 40)
	if err := tm.StartArgv("", self, "-test.run=^TestSizeHelperProcess$", "-test.count=1"); err != nil {
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
