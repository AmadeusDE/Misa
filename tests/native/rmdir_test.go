package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRmdir_RemoveEmpty(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty_dir")
	os.Mkdir(target, 0755)

	r := runIn(t, dir, "rmdir", "empty_dir")
	wantSuccess(t, r)

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestRmdir_Parents(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a/b/c")
	os.MkdirAll(target, 0755)

	r := runIn(t, dir, "rmdir", "-p", "a/b/c")
	wantSuccess(t, r)

	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Error("expected parent directories to be removed")
	}
}

func TestRmdir_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "non_empty")
	mkFile(t, target, "file.txt", "content")

	r := runIn(t, dir, "rmdir", "non_empty")
	wantCode(t, r, 1)
	wantErrContains(t, r, "not empty")
}

func TestRmdir_IgnoreFailOnNonEmpty(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "non_empty")
	mkFile(t, target, "file.txt", "content")

	r := runIn(t, dir, "rmdir", "--ignore-fail-on-non-empty", "non_empty")
	wantSuccess(t, r)
}

func TestRmdir_NonExistent(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "rmdir", "does-not-exist")
	wantCode(t, r, 1)
	wantErrContains(t, r, "No such file or directory")
}

func TestRmdir_SymlinkToDir(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	os.Mkdir(targetDir, 0755)
	symlink(t, "target", filepath.Join(dir, "sl"))

	r := runIn(t, dir, "rmdir", "sl")
	wantCode(t, r, 1)
	wantErrContains(t, r, "rmdir: failed to remove 'sl': Not a directory")
}

func TestRmdir_SymlinkToDirWithSlash(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	os.Mkdir(targetDir, 0755)
	symlink(t, "target", filepath.Join(dir, "sl"))

	r := runIn(t, dir, "rmdir", "sl/")
	wantCode(t, r, 1)
	wantErrContains(t, r, "rmdir: failed to remove 'sl/': Symbolic link not followed")
}
