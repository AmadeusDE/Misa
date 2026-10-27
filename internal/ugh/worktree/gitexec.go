package worktree

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// GitFunc runs git with args in dir, env adding variables to the environment. A failed
// command still returns what it wrote.
type GitFunc func(ctx context.Context, dir string, env []string, args ...string) (stdout, stderr []byte, err error)

type gitKey struct{}

// WithGit makes git commands under ctx run through f, for example on a VM that shares the
// repository at the same path.
func WithGit(ctx context.Context, f GitFunc) context.Context {
	return context.WithValue(ctx, gitKey{}, f)
}

// Git runs one git command through the runner in ctx, or here when there is none.
func Git(ctx context.Context, dir string, env []string, args ...string) ([]byte, []byte, error) {
	if f, ok := ctx.Value(gitKey{}).(GitFunc); ok && f != nil {
		return f(ctx, dir, env, args...)
	}
	return LocalGit(ctx, dir, env, args...)
}

// LocalGit is the GitFunc that runs git in this process's environment.
func LocalGit(ctx context.Context, dir string, env []string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
