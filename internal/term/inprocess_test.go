package term

import (
	"context"
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
	release := make(chan struct{})
	defer close(release)
	for name, fn := range map[string]func(*os.File) int{
		"sleeping": func(*os.File) int { <-release; return 0 },
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

func TestInterruptCancelsFunctionContext(t *testing.T) {
	for _, interrupt := range []bool{true, false} {
		tm := New(5, 40)
		entered := make(chan struct{})
		observed := make(chan struct{})
		err := tm.StartFuncContext(context.Background(), func(ctx context.Context, tty *os.File) int {
			close(entered)
			<-ctx.Done()
			close(observed)
			return 0
		})
		if err != nil {
			t.Fatal(err)
		}
		tm.StartReader()
		<-entered
		if interrupt {
			tm.Interrupt()
		} else {
			tm.Close()
		}
		select {
		case <-observed:
		case <-time.After(time.Second):
			t.Fatal("function did not observe cancellation")
		}
		waitExited(t, tm)
		if code := tm.WaitExit(); interrupt && code != 130 {
			t.Fatalf("exit = %d", code)
		}
	}
}
