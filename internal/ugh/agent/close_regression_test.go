package agent

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

// Embedded in Misa, closing the pane cancels the root's run context and the
// process keeps running, so nothing cleans up by exiting. A background
// subagent started by that run, and the bash command it is in, must both
// stop: the child's context derives from the run's, and bash kills its
// whole process group on cancel.
func TestCancelledRunStopsBackgroundChildAndItsBash(t *testing.T) {
	pidFile := t.TempDir() + "/pid"
	bash, _ := tools.NewRegistry(tools.All()...).Get("bash")
	finished := make(chan struct{})
	a := New(Config{Provider: &funcProvider{}, Model: "m", Cwd: t.TempDir(),
		StartChild: func(ctx context.Context, _ *Agent, _ Task) ChildResult {
			defer close(finished)
			raw, _ := json.Marshal(map[string]any{"command": "sleep 60 & echo $! > " + pidFile + "; wait", "timeout_seconds": 600})
			_, _ = bash.Run(ctx, t.TempDir(), raw)
			return ChildResult{Status: "cancelled"}
		}})
	runCtx, cancel := context.WithCancel(context.Background())
	a.startKids(runCtx, []Task{{Task: "long job"}})
	var child int
	for deadline := time.Now().Add(5 * time.Second); child == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("child's bash never started")
		}
		b, _ := os.ReadFile(pidFile)
		child, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("background child kept running after its run was cancelled")
	}
	for deadline := time.Now().Add(5 * time.Second); syscall.Kill(child, 0) == nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(child, syscall.SIGKILL)
			t.Fatal("the child's bash process outlived the cancel")
		}
	}
}
