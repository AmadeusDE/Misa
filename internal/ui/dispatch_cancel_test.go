package ui

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestRunBuiltinContextCancellation(t *testing.T) {
	command, _ := builtins.Lookup("sleep")
	plan := commandPlan{builtin: command, fields: []string{"sleep", "60"}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- runBuiltinContext(ctx, plan, t.TempDir(), nil, io.Discard, 80, 24) }()
	cancel()
	select {
	case code := <-done:
		if code != 130 {
			t.Fatalf("status = %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("invocation context not propagated")
	}
}
