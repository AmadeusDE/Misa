package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Terminal-owning commands retain their own signal handling. Job suspension is
// separate: this bridge never subscribes to SIGTSTP/SIGTTIN/SIGTTOU.
func builtinSignalBridgeSafe(name string) bool {
	switch name {
	case "vi", "top", "htop", "btop", "bytop", "bashtop", "tm", "mines", "chess", "battleship", "sshc", "devices", "timeout":
		return false
	}
	return true
}

func standaloneBuiltinContext(name string) (context.Context, func()) {
	if !builtinSignalBridgeSafe(name) {
		return context.Background(), func() {}
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	ctx, stop := bridgeBuiltinSignals(signals, 500*time.Millisecond, os.Exit)
	return ctx, func() { signal.Stop(signals); stop() }
}

func bridgeBuiltinSignals(signals <-chan os.Signal, grace time.Duration, exit func(int)) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		var first os.Signal
		select {
		case first = <-signals:
			cancel()
		case <-stopped:
			return
		}
		timer := time.NewTimer(grace)
		defer timer.Stop()
		select {
		case sig := <-signals:
			exit(128 + int(sig.(syscall.Signal)))
		case <-timer.C:
			exit(128 + int(first.(syscall.Signal)))
		case <-stopped:
		}
	}()
	return ctx, func() {
		select {
		case <-stopped:
		default:
			close(stopped)
		}
		cancel()
		<-done
	}
}
