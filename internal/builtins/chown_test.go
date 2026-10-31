package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ownUidGid is the account running the tests: chown to its own ids needs no
// privileges, which keeps these tests unprivileged-safe.
func ownUidGid() string {
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

func TestChownMissingOperand(t *testing.T) {
	r := runBuiltin(t, "chown")
	assertExitCode(t, r, 1)
	if r.stderr != "chown: missing operand\n" {
		t.Errorf("stderr = %q, want %q", r.stderr, "chown: missing operand\n")
	}

	r = runBuiltin(t, "chown", ownUidGid())
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing operand")
}

func TestChownVerboseNonRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "x")

	r := runBuiltinIn(t, dir, "chown", "-v", ownUidGid(), "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed ownership of 'f.txt'\n")
	assertNoStderr(t, r)
}

func TestChownErrorIsRaw(t *testing.T) {
	// chown reports the kernel's own error text, with no "changing ownership
	// of" frame (that is chgrp's format).
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "chown", ownUidGid(), "nope.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "chown: chown ")

	r = runBuiltinIn(t, dir, "chown", "-f", ownUidGid(), "nope.txt")
	assertExitCode(t, r, 1)
	assertNoStderr(t, r)
}

func TestChownDanglingSymlinkOperand(t *testing.T) {
	// Without -R and without -h, a dangling symlink on the command line is
	// rejected before the operation is attempted.
	dir := t.TempDir()
	if err := os.Symlink("missing", filepath.Join(dir, "dangling")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	r := runBuiltinIn(t, dir, "chown", ownUidGid(), "dangling")
	assertExitCode(t, r, 1)
	if want := "chown: cannot dereference 'dangling': No such file or directory\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}

	// With -R the pre-check does not apply and -h is implied (toybox/GNU), so
	// the dangling link itself is the target: nothing to dereference, and the
	// lchown of the link succeeds.
	r = runBuiltinIn(t, dir, "chown", "-R", ownUidGid(), "dangling")
	assertExitCode(t, r, 0)
	if r.stderr != "" {
		t.Errorf("chown -R on a dangling symlink: stderr = %q, want empty", r.stderr)
	}
}

// chownFollowTree builds top/f1 and toplink -> top.
func chownFollowTree(t *testing.T) (top, toplink string) {
	t.Helper()
	dir := t.TempDir()
	top = filepath.Join(dir, "top")
	if err := os.MkdirAll(top, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "top/f1", "x")
	toplink = filepath.Join(dir, "toplink")
	if err := os.Symlink("top", toplink); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return top, toplink
}

func TestChownRecursiveFollowCmdline(t *testing.T) {
	// -H: a symlink named on the command line is followed and recursed into.
	top, _ := chownFollowTree(t)
	dir := filepath.Dir(top)

	r := runBuiltinIn(t, dir, "chown", "-RHv", ownUidGid(), "toplink")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed ownership of 'toplink'\nchanged ownership of 'toplink/f1'\n")
	assertNoStderr(t, r)
}

func TestChownRecursiveSymlinkOperandDefault(t *testing.T) {
	// Without -H/-L the command-line symlink is dereferenced (its target is
	// changed) but not entered.
	top, _ := chownFollowTree(t)
	dir := filepath.Dir(top)

	r := runBuiltinIn(t, dir, "chown", "-Rv", ownUidGid(), "toplink")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed ownership of 'toplink'\n")
	assertNoStderr(t, r)
}

func TestChownFollowFlagsAreNoOpsWithoutRecursive(t *testing.T) {
	// -H, -L and -P only matter with -R.
	top, _ := chownFollowTree(t)
	dir := filepath.Dir(top)
	for _, flag := range []string{"-H", "-L", "-P"} {
		r := runBuiltinIn(t, dir, "chown", flag, "-v", ownUidGid(), "toplink")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "changed ownership of 'toplink'\n")
	}
}

func TestChownSharedWalkerMatchesRecursiveL(t *testing.T) {
	// Both historical -R paths (WalkDir and the -L recursion) now share one
	// walker: the -L form is kept, so children are named <operand>/<child>.
	dir := t.TempDir()
	writeFile(t, dir, "d/a", "x")
	writeFile(t, dir, "d/b", "x")

	for _, flag := range []string{"-R", "-RL"} {
		r := runBuiltinIn(t, dir, "chown", flag, "-v", ownUidGid(), "d")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "changed ownership of 'd'\nchanged ownership of 'd/a'\nchanged ownership of 'd/b'\n")
	}
}
