package worktree

import (
	"context"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/ugh/confine"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// runGitCmd executes a git command in the specified directory.
func runGitCmd(ctx context.Context, dir string, args ...string) (string, error) {
	stdout, stderr, err := Git(ctx, dir, nil, args...)
	outStr := string(stdout)
	if !strings.ContainsRune(outStr, 0) {
		outStr = strings.TrimSpace(outStr)
	}
	errStr := strings.TrimSpace(string(stderr))
	if err != nil {
		if errStr != "" {
			return outStr, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, errStr)
		}
		return outStr, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return outStr, nil
}

// isDirty checks if there are uncommitted changes (tracked or untracked) in dir.
func isDirty(ctx context.Context, dir string) (bool, error) {
	files, err := getUncommittedFiles(ctx, dir)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// getUncommittedFiles returns the list of changed files in dir according to git status,
// filtering out internal harness metadata files (.ugh-worktree.json, ugh-worktree.lock).
func getUncommittedFiles(ctx context.Context, dir string) ([]string, error) {
	out, err := runGitCmd(ctx, dir, "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	entries := strings.Split(out, "\x00")
	var files []string
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		name := entry[3:]
		if strings.ContainsAny(entry[:2], "RC") {
			i++
		} // rename source follows destination in -z output
		if !isMetadataPath(name) {
			files = append(files, name)
		}
	}
	return files, nil
}

// getCommit returns the full commit hash for the given reference.
func getCommit(ctx context.Context, dir, ref string) (string, error) {
	out, err := runGitCmd(ctx, dir, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// getCurrentBranch returns the current branch name of dir, or "HEAD" if detached.
func getCurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := runGitCmd(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err == nil {
		return strings.TrimSpace(out), nil
	}
	// Detached HEAD fallback
	return "HEAD", nil
}

// getCommitsBetween returns commit hashes in chronological order between base and target.
func getCommitsBetween(ctx context.Context, dir, base, target string) ([]string, error) {
	out, err := runGitCmd(ctx, dir, "rev-list", "--reverse", base+".."+target)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var commits []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

// getConflictedFiles returns the list of files currently experiencing merge/cherry-pick conflicts.
func getConflictedFiles(ctx context.Context, dir string) []string {
	out, err := runGitCmd(ctx, dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var files []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files
}

// createStashSnapshot generates a commit containing uncommitted changes without modifying
// HEAD, the working directory, or the reflog, using `git stash create`.
func createStashSnapshot(ctx context.Context, dir string) (string, error) {
	// Even index refreshes must not touch the parent's index. Stash creation
	// reads an isolated copy; it never writes refs/stash or the parent's files.
	index, err := runGitCmd(ctx, dir, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(index) {
		index = filepath.Join(dir, index)
	}
	data, err := os.ReadFile(index)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(index), "ugh-snapshot-index-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	raw, _, err := Git(ctx, dir, []string{"GIT_INDEX_FILE=" + tmp.Name()}, "stash", "create")
	if err != nil {
		return "", fmt.Errorf("git stash create snapshot: %w", err)
	}
	out := string(raw)
	commit := strings.TrimSpace(out)
	if commit == "" {
		// If git stash create outputs nothing, fallback to HEAD
		return getCommit(ctx, dir, "HEAD")
	}
	return commit, nil
}

// branchExists checks if a branch exists locally in the repository.
func branchExists(ctx context.Context, dir, branch string) bool {
	_, err := runGitCmd(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func isMetadataPath(path string) bool {
	switch filepath.Base(path) {
	case ".ugh-session.jsonl", ".ugh-worktree.json", "ugh-worktree.lock":
		return true
	}
	return false
}

// Stash snapshots omit untracked files. Copy those explicitly, without following
// symlinks or changing the parent's index, working files or stash list.
func copyUntracked(ctx context.Context, source, destination string) error {
	out, err := runGitCmd(ctx, source, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	for _, path := range strings.Split(out, "\x00") {
		if path == "" || isMetadataPath(path) {
			continue
		}
		if err := copySnapshotFile(filepath.Join(source, path), filepath.Join(destination, path)); err != nil {
			return fmt.Errorf("snapshot untracked %q: %w", path, err)
		}
	}
	return nil
}

func copySnapshotFile(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if err := confine.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(target, destination)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file or symlink")
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := confine.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
