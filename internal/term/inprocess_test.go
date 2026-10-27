package term

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

func TestStartFuncRunsOnTTY(t *testing.T) {
	tm := New(5, 40)
	err := tm.StartFunc(func(tty *os.File) int {
		fmt.Fprint(tty, "hello from builtin")
		return 7
	})
	if err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	waitExited(t, tm)
	if code := tm.WaitExit(); code != 7 {
		t.Errorf("exit = %d, want 7", code)
	}
	if got := tm.PlainScreen()[0]; got != "hello from builtin" {
		t.Errorf("screen = %q", got)
	}
}

func TestInterruptAbandonsBlockedBuiltin(t *testing.T) {
	for name, fn := range map[string]func(*os.File) int{
		"sleeping": func(*os.File) int { time.Sleep(time.Minute); return 0 },
		"reading":  func(tty *os.File) int { io.ReadAll(tty); return 0 },
	} {
		tm := New(5, 40)
		if err := tm.StartFunc(fn); err != nil {
			t.Fatal(err)
		}
		tm.StartReader()
		if !tm.Interrupt() {
			t.Fatalf("%s: Interrupt refused an in-process session", name)
		}
		waitExited(t, tm)
		if code := tm.WaitExit(); code != 130 {
			t.Errorf("%s: exit = %d, want 130", name, code)
		}
	}
}
