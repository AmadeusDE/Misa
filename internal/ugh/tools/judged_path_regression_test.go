package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTool(t *testing.T, tool interface {
	Run(context.Context, string, json.RawMessage) (string, error)
}, cwd string, args map[string]string) (string, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return tool.Run(context.Background(), cwd, raw)
}

// The permission rules judge a dangling symlink as the link's own path in
// the workspace; writing must not follow it to a file somewhere else.
func TestWriteRefusesADanglingSymlinkOutOfItsDirectory(t *testing.T) {
	ws, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "planted")
	if err := os.Symlink(target, filepath.Join(ws, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := runTool(t, writeTool{}, ws, map[string]string{"path": "notes.txt", "content": "x"}); err == nil {
		t.Error("write through a dangling symlink succeeded")
	}
	if _, err := os.Lstat(target); err == nil {
		t.Error("write created the symlink's target outside the workspace")
	}
}

// The rules expand ~ to $HOME; the tools must open the same file.
func TestToolsExpandHomeLikeTheRules(t *testing.T) {
	home, ws := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	os.WriteFile(filepath.Join(home, "f.txt"), []byte("from home\n"), 0o644)
	out, err := runTool(t, readTool{}, ws, map[string]string{"path": "~/f.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "from home"; !strings.Contains(out, want) {
		t.Errorf("read ~/f.txt = %q, want the file in $HOME", out)
	}
}

// Paths resolve through their symlinks, as the rules judge them.
func TestResolveFollowsSymlinksLikeTheRules(t *testing.T) {
	ws, outside := t.TempDir(), t.TempDir()
	os.Symlink(outside, filepath.Join(ws, "link"))
	realOutside, _ := filepath.EvalSymlinks(outside)
	if got, want := resolve(ws, "link/new/file"), filepath.Join(realOutside, "new", "file"); got != want {
		t.Errorf("resolve = %s, want %s", got, want)
	}
}
