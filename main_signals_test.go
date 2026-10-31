package main

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestStandaloneSignalBridge(t *testing.T) {
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			signals := make(chan os.Signal, 2)
			exits := make(chan int, 1)
			ctx, stop := bridgeBuiltinSignals(signals, 20*time.Millisecond, func(code int) { exits <- code })
			defer stop()
			signals <- sig
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("not canceled")
			}
			select {
			case code := <-exits:
				want := 128 + int(sig.(syscall.Signal))
				if code != want {
					t.Fatalf("exit %d want %d", code, want)
				}
			case <-time.After(time.Second):
				t.Fatal("no forced exit")
			}
		})
	}
}
func TestStandaloneSecondSignalAndCleanup(t *testing.T) {
	signals := make(chan os.Signal, 2)
	exits := make(chan int, 1)
	ctx, stop := bridgeBuiltinSignals(signals, time.Hour, func(code int) { exits <- code })
	signals <- os.Interrupt
	<-ctx.Done()
	signals <- syscall.SIGTERM
	select {
	case code := <-exits:
		if code != 143 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("second signal swallowed")
	}
	stop()
	signals = make(chan os.Signal, 2)
	_, stop = bridgeBuiltinSignals(signals, 10*time.Millisecond, func(code int) { exits <- code })
	stop()
	signals <- os.Interrupt
	select {
	case code := <-exits:
		t.Fatalf("exit after cleanup: %d", code)
	case <-time.After(25 * time.Millisecond):
	}
}
func TestStandaloneSignalExemptions(t *testing.T) {
	for _, name := range []string{"vi", "top", "htop", "btop", "bytop", "bashtop", "tm", "mines", "chess", "battleship", "sshc", "devices", "timeout"} {
		if builtinSignalBridgeSafe(name) {
			t.Errorf("%s owns terminal/signals", name)
		}
	}
	for _, name := range []string{"sleep", "cat", "yes", "tail", "cp"} {
		if !builtinSignalBridgeSafe(name) {
			t.Error(name)
		}
	}
}
