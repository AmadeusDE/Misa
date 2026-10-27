package worktree

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"
)

// backgroundGroup starts a process group whose leader exits at once while a
// backgrounded sleep lives on in it, as `cmd &` from the bash tool does.
func backgroundGroup(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sh", "-c", "sleep 30 & exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	pgid := cmd.Process.Pid
	t.Cleanup(func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
	if !isGroupAlive(pgid) {
		t.Fatal("background job did not survive its leader")
	}
	return pgid
}

func waitGroupGone(t *testing.T, pgid int) {
	t.Helper()
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	for deadline := time.Now().Add(5 * time.Second); isGroupAlive(pgid); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("group did not exit")
		}
	}
}

// Off Linux there is no /proc, so the cwd scan that guarded worktrees in use
// was silently skipped and nothing ugh started was tracked. A tracked
// background group must block integration and cleanup on its own, with
// Force changing nothing, and both must proceed once it is gone.
func TestTrackedGroupGuardsWorktreeWithoutProc(t *testing.T) {
	old := procRoot
	procRoot = filepath.Join(t.TempDir(), "no-proc")
	t.Cleanup(func() { procRoot = old })
	ctx := context.Background()
	repo, wt := preparedIntegration(t)
	pgid := backgroundGroup(t)
	if err := wt.TrackGroup(ctx, pgid); err != nil {
		t.Fatal(err)
	}

	opts := IntegrateOptions{WorktreeDir: wt.Dir, TargetBranch: "master"}
	if _, err := Integrate(ctx, repo, opts); !errors.Is(err, ErrActiveWriter) {
		t.Fatalf("integration with a live background group: %v", err)
	}
	if err := Cleanup(ctx, repo, wt.Dir, CleanupOptions{Force: true}); !errors.Is(err, ErrActiveProcesses) {
		t.Fatalf("forced cleanup with a live background group: %v", err)
	}

	waitGroupGone(t, pgid)
	if _, err := Integrate(ctx, repo, opts); err != nil {
		t.Fatalf("integration after the group exited: %v", err)
	}
	if err := Cleanup(ctx, repo, wt.Dir, CleanupOptions{}); err != nil {
		t.Fatalf("cleanup after the group exited: %v", err)
	}
}

// Tracking prunes groups that have exited, so the list stays bounded.
func TestTrackGroupPrunesExitedGroups(t *testing.T) {
	_, wt := preparedIntegration(t)
	ctx := context.Background()
	gone := backgroundGroup(t)
	if err := wt.TrackGroup(ctx, gone); err != nil {
		t.Fatal(err)
	}
	waitGroupGone(t, gone)
	live := backgroundGroup(t)
	if err := wt.TrackGroup(ctx, live); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(wt.Meta.ActiveGroups, []int{live}) {
		t.Fatalf("groups = %v, want only %d", wt.Meta.ActiveGroups, live)
	}
	loaded, err := Load(wt.Dir)
	if err != nil || !slices.Equal(loaded.Meta.ActiveGroups, []int{live}) {
		t.Fatalf("persisted groups = %v (%v)", loaded.Meta.ActiveGroups, err)
	}
}
