package worktree

import (
	"context"
	"strings"
	"testing"
)

func TestGitFollowsTheRunnerInContext(t *testing.T) {
	var got []string
	ctx := WithGit(context.Background(), func(_ context.Context, dir string, env []string, args ...string) ([]byte, []byte, error) {
		got = append(got, dir+":"+strings.Join(env, ",")+":"+strings.Join(args, " "))
		return []byte("/elsewhere/.git\n"), nil, nil
	})
	common, err := FindGitCommonDir(ctx, "/repo")
	if err != nil || common != "/elsewhere/.git" {
		t.Fatalf("common = %q, %v", common, err)
	}
	if len(got) != 1 || got[0] != "/repo::rev-parse --git-common-dir" {
		t.Fatalf("runner calls = %v", got)
	}
}
