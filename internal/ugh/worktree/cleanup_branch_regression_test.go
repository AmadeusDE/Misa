package worktree

import (
	"context"
	"path/filepath"
	"testing"
)

func allocatedForCleanup(t *testing.T) (string, *Worktree) {
	t.Helper()
	repo := initTestRepo(t)
	runCmd(t, repo, "git", "branch", "keep")
	wt, err := Allocate(context.Background(), repo, AllocateOptions{Run: "r", Agent: "root/1", Branch: "work", WorktreeDir: filepath.Join(t.TempDir(), "wt")})
	if err != nil {
		t.Fatal(err)
	}
	if err := wt.MarkCompleted("done"); err != nil {
		t.Fatal(err)
	}
	return repo, wt
}

// The worker can rewrite the metadata in its worktree; cleanup must not
// delete whatever branch that metadata names.
func TestCleanupIgnoresABranchNamedByRewrittenMetadata(t *testing.T) {
	repo, wt := allocatedForCleanup(t)
	wt.Meta.Branch = "keep"
	if err := wt.SaveMeta(); err != nil {
		t.Fatal(err)
	}
	Cleanup(context.Background(), repo, wt.Dir, CleanupOptions{DeleteBranch: true, Force: true})
	if !branchExists(context.Background(), repo, "keep") {
		t.Fatal("cleanup deleted the branch the rewritten metadata named")
	}
}

func TestCleanupKeepsABranchTheWorkerSwitchedTo(t *testing.T) {
	repo, wt := allocatedForCleanup(t)
	runCmd(t, wt.Dir, "git", "switch", "-q", "keep")
	Cleanup(context.Background(), repo, wt.Dir, CleanupOptions{DeleteBranch: true, Force: true, Branch: "work"})
	if !branchExists(context.Background(), repo, "keep") {
		t.Fatal("cleanup deleted a branch the worker switched to")
	}
}

func TestCleanupDeletesTheAllocatedBranch(t *testing.T) {
	repo, wt := allocatedForCleanup(t)
	if err := Cleanup(context.Background(), repo, wt.Dir, CleanupOptions{DeleteBranch: true, Force: true, Branch: "work"}); err != nil {
		t.Fatal(err)
	}
	if branchExists(context.Background(), repo, "work") {
		t.Fatal("allocated branch survived")
	}
}
