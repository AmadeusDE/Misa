package worktree

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSnapshotCarriesDirtyStateWithoutChangingParent(t *testing.T) {
	ctx := context.Background()
	repo := initTestRepo(t)
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, path)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Preserve both the index version and a distinct unstaged version.
	write("README.md", "staged version\n")
	runCmd(t, repo, "git", "add", "README.md")
	write("README.md", "unstaged version\n")
	write("new dir/file with spaces.txt", "untracked contents\n")
	write("trailing space ", "untracked whitespace path\n")
	if err := os.Symlink("README.md", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(runCmd(t, repo, "git", "rev-parse", "HEAD"))
	indexPath := filepath.Join(repo, ".git", "index")
	// Read status without refreshing index stats; even those bytes must remain unchanged.
	beforeStatus := runCmd(t, repo, "git", "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	beforeIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeStash := runCmd(t, repo, "git", "stash", "list")
	wt, err := Allocate(ctx, repo, AllocateOptions{Run: "snapshot-run", Agent: "root/worker-1", AuthorizeSnapshot: true, WorktreeDir: filepath.Join(t.TempDir(), "child")})
	if err != nil {
		t.Fatal(err)
	}
	afterIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeIndex, afterIndex) {
		t.Fatal("snapshot modified parent index")
	}
	if beforeStash != runCmd(t, repo, "git", "stash", "list") {
		t.Fatal("snapshot modified parent stash list")
	}
	if beforeStatus != runCmd(t, repo, "git", "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=all") {
		t.Fatal("snapshot changed parent status")
	}
	if wt.Meta.BaseCommit != head || wt.Meta.SnapshotCommit == "" || wt.Meta.SnapshotCommit == head {
		t.Fatalf("snapshot metadata: %+v", wt.Meta)
	}
	if actual := strings.TrimSpace(runCmd(t, wt.Dir, "git", "rev-parse", "HEAD")); actual != head {
		t.Fatalf("snapshot became branch HEAD: %s", actual)
	}
	if got := runCmd(t, wt.Dir, "git", "show", ":README.md"); got != "staged version\n" {
		t.Fatalf("index version not carried: %q", got)
	}
	for _, path := range []string{"README.md", "new dir/file with spaces.txt", "trailing space "} {
		parent, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil {
			t.Fatal(err)
		}
		child, err := os.ReadFile(filepath.Join(wt.Dir, path))
		if err != nil || !bytes.Equal(parent, child) {
			t.Fatalf("%q differs: %q %v", path, child, err)
		}
		if !slices.Contains(wt.Meta.DirtyFiles, path) {
			t.Errorf("%q absent from carried file list: %q", path, wt.Meta.DirtyFiles)
		}
	}
	if target, err := os.Readlink(filepath.Join(wt.Dir, "link")); err != nil || target != "README.md" {
		t.Fatalf("symlink=%q err=%v", target, err)
	}
	loaded, err := Load(wt.Dir)
	if err != nil || loaded.Meta.SnapshotCommit != wt.Meta.SnapshotCommit || !slices.Equal(loaded.Meta.DirtyFiles, wt.Meta.DirtyFiles) {
		t.Fatalf("persisted baseline: %v %v", loaded, err)
	}
}
