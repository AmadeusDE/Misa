package tools

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A command that leaves a background job behind reports its process group,
// so a managed worktree can refuse cleanup while the job runs; a command
// that finishes cleanly reports nothing.
func TestBashReportsSurvivingGroups(t *testing.T) {
	var got []int
	ctx := WithGroupTracker(context.Background(), func(pgid int) { got = append(got, pgid) })
	run := func(cmd string) {
		raw, _ := json.Marshal(map[string]string{"command": cmd})
		if _, err := (bashTool{}).Run(ctx, t.TempDir(), raw); err != nil {
			t.Fatal(err)
		}
	}
	run("true")
	if len(got) != 0 {
		t.Fatalf("finished command reported groups %v", got)
	}
	run("sleep 30 >/dev/null 2>&1 &")
	if len(got) != 1 || !groupAlive(got[0]) {
		t.Fatalf("background job not reported: %v", got)
	}
	_ = syscall.Kill(-got[0], syscall.SIGKILL)
}

// Cancelling a running command kills its whole process group, children
// included, not just the shell.
func TestBashCancelKillsTheGroup(t *testing.T) {
	var pgid int
	ctx, cancel := context.WithCancel(WithGroupTracker(context.Background(), func(g int) { pgid = g }))
	pidFile := t.TempDir() + "/pid"
	done := make(chan struct{})
	go func() {
		raw, _ := json.Marshal(map[string]string{"command": "sleep 30 & echo $! > " + pidFile + "; wait"})
		_, _ = (bashTool{}).Run(ctx, t.TempDir(), raw)
		close(done)
	}()
	var child int
	for deadline := time.Now().Add(5 * time.Second); child == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("command did not start")
		}
		if b, err := os.ReadFile(pidFile); err == nil {
			child, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled command did not return")
	}
	for deadline := time.Now().Add(5 * time.Second); syscall.Kill(child, 0) == nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(child, syscall.SIGKILL)
			t.Fatal("the shell's child outlived the cancel")
		}
	}
	if pgid != 0 {
		t.Fatalf("a killed group was reported as surviving: %d", pgid)
	}
}
