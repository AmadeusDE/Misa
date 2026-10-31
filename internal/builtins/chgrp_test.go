package builtins

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// The tests run unprivileged: uid 1000, primary gid 1000, supplementary group
// 998 in the CI image. Nothing here needs (or may need) root, and gid 0 is used
// as the group we are guaranteed not to be a member of.

// chgrpGroups returns the process's supplementary groups (os.Getgroups'
// error is not interesting for these tests).
func chgrpGroups(t *testing.T) []int {
	t.Helper()
	gids, err := os.Getgroups()
	if err != nil {
		t.Fatalf("getgroups: %v", err)
	}
	return gids
}

// chgrpTestGroups returns the process's primary gid and a second gid it belongs
// to (changing a file's group to either is permitted without privileges).
func chgrpTestGroups(t *testing.T) (primary, other int) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: every group change is permitted")
	}
	primary = os.Getgid()
	for _, g := range chgrpGroups(t) {
		if g != primary {
			return primary, g
		}
	}
	t.Skip("process belongs to a single group")
	return 0, 0
}

// chgrpRequireNoRootGroup skips when changing a file to gid 0 would be allowed.
func chgrpRequireNoRootGroup(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: gid 0 is settable")
	}
	for _, g := range chgrpGroups(t) {
		if g == 0 {
			t.Skip("process belongs to group 0")
		}
	}
}

func statGid(t *testing.T, path string) int {
	t.Helper()
	info, err := os.Stat(path) // follows symlinks
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat %s: no syscall.Stat_t", path)
	}
	return int(st.Gid)
}

func lstatGid(t *testing.T, path string) int {
	t.Helper()
	info, err := os.Lstat(path) // the symlink itself
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("lstat %s: no syscall.Stat_t", path)
	}
	return int(st.Gid)
}

func assertNoStderr(t *testing.T, r runResult) {
	t.Helper()
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func wantGid(t *testing.T, path string, want int) {
	t.Helper()
	if got := statGid(t, path); got != want {
		t.Errorf("gid of %s = %d, want %d", filepath.Base(path), got, want)
	}
}

// chgrpSymlinkTree builds
//
//	top/f1            regular file
//	top/ln -> ../sib  symlink to a directory
//	sib/f2            regular file
//	toplink -> top    symlink to the tree, named on the command line
//
// and makes sure every gid in it differs from target.
func chgrpSymlinkTree(t *testing.T, target int) (top, toplink string) {
	t.Helper()
	dir := t.TempDir()
	top = filepath.Join(dir, "top")
	if err := os.MkdirAll(top, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "top/f1", "1")
	writeFile(t, dir, "sib/f2", "2")
	if err := os.Symlink("../sib", filepath.Join(top, "ln")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	toplink = filepath.Join(dir, "toplink")
	if err := os.Symlink("top", toplink); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	for _, p := range []string{top, filepath.Join(top, "f1"), filepath.Join(dir, "sib"), filepath.Join(dir, "sib/f2")} {
		if statGid(t, p) == target {
			t.Skipf("%s already belongs to gid %d", p, target)
		}
	}
	return top, toplink
}

func TestChgrpNumericGroup(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "data")

	r := runBuiltinIn(t, dir, "chgrp", strconv.Itoa(other), "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	assertNoStderr(t, r)
	wantGid(t, p, other)
}

func TestChgrpNamedGroup(t *testing.T) {
	primary, other := chgrpTestGroups(t)
	g, err := user.LookupGroupId(strconv.Itoa(primary))
	if err != nil {
		t.Skipf("no group name for gid %d: %v", primary, err)
	}
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "data")
	// Start from a different gid so the change is observable even when the
	// file's original group already is the primary one.
	if r := runBuiltinIn(t, dir, "chgrp", strconv.Itoa(other), "f.txt"); r.code != 0 {
		t.Fatalf("setup chgrp failed: %+v", r)
	}

	r := runBuiltinIn(t, dir, "chgrp", g.Name, "f.txt")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	wantGid(t, p, primary)
}

func TestChgrpRecursive(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	for _, name := range []string{"f.txt", "sub/g.txt", "sub/deeper/h.txt"} {
		writeFile(t, dir, name, "x")
	}

	r := runBuiltinIn(t, dir, "chgrp", "-R", strconv.Itoa(other), ".")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	for _, name := range []string{"f.txt", "sub", "sub/g.txt", "sub/deeper", "sub/deeper/h.txt"} {
		if got := statGid(t, filepath.Join(dir, name)); got != other {
			t.Errorf("gid of %s = %d, want %d", name, got, other)
		}
	}
}

func TestChgrpRecursiveRegularFile(t *testing.T) {
	// -R on a plain file has nothing to recurse into and must still succeed.
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "data")

	r := runBuiltinIn(t, dir, "chgrp", "-R", strconv.Itoa(other), "f.txt")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	wantGid(t, p, other)
}

func TestChgrpRecursiveSymlinkOperandDefault(t *testing.T) {
	// -R -P (the default): -h is implied by -R, as in toybox and GNU, so the
	// command-line symlink itself changes and nothing behind it does.
	_, other := chgrpTestGroups(t)
	top, toplink := chgrpSymlinkTree(t, other)
	sib := filepath.Join(filepath.Dir(top), "sib")

	r := runBuiltinIn(t, filepath.Dir(top), "chgrp", "-R", strconv.Itoa(other), "toplink")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	if got := lstatGid(t, toplink); got != other {
		t.Errorf("gid of the symlink itself = %d, want %d", got, other)
	}
	wantGid(t, top, os.Getgid())
	wantGid(t, filepath.Join(top, "f1"), os.Getgid())
	wantGid(t, filepath.Join(sib, "f2"), os.Getgid())
}

func TestChgrpRecursiveSymlinkOperandFollowCmdline(t *testing.T) {
	// -R -H: the command-line symlink is followed and recursed into; symlinks
	// inside the tree are not.
	_, other := chgrpTestGroups(t)
	top, toplink := chgrpSymlinkTree(t, other)
	dir := filepath.Dir(top)
	sib := filepath.Join(dir, "sib")
	before := statGid(t, filepath.Join(sib, "f2"))
	linkGid := lstatGid(t, toplink)

	r := runBuiltinIn(t, dir, "chgrp", "-RH", strconv.Itoa(other), "toplink")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	wantGid(t, top, other)
	wantGid(t, filepath.Join(top, "f1"), other)
	wantGid(t, sib, other) // top/ln was applied to its target
	wantGid(t, filepath.Join(sib, "f2"), before)
	if got := lstatGid(t, toplink); got != linkGid {
		t.Errorf("gid of the symlink itself = %d, want %d", got, linkGid)
	}
}

func TestChgrpRecursiveSymlinkFollowAll(t *testing.T) {
	// -R -L: every symlink is followed and recursed into.
	_, other := chgrpTestGroups(t)
	top, toplink := chgrpSymlinkTree(t, other)
	dir := filepath.Dir(top)
	sib := filepath.Join(dir, "sib")
	linkGid := lstatGid(t, toplink)

	r := runBuiltinIn(t, dir, "chgrp", "-RL", strconv.Itoa(other), "toplink")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	wantGid(t, top, other)
	wantGid(t, filepath.Join(top, "f1"), other)
	wantGid(t, sib, other)
	wantGid(t, filepath.Join(sib, "f2"), other)
	if got := lstatGid(t, toplink); got != linkGid {
		t.Errorf("gid of the symlink itself = %d, want %d", got, linkGid)
	}
}

func TestChgrpRecursiveSymlinkLoop(t *testing.T) {
	// A symlink loop must terminate: loop/loop -> loop is entered once.
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	loop := filepath.Join(dir, "loop")
	if err := os.MkdirAll(loop, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".", filepath.Join(loop, "again")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if statGid(t, loop) == other {
		t.Skipf("loop already belongs to gid %d", other)
	}

	r := runBuiltinIn(t, dir, "chgrp", "-RL", strconv.Itoa(other), "loop")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	wantGid(t, loop, other)
}

func TestChgrpSymlinkDereferenced(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	target := writeFile(t, dir, "target", "x")
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	linkGid := lstatGid(t, link)

	r := runBuiltinIn(t, dir, "chgrp", strconv.Itoa(other), "link")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	wantGid(t, target, other)
	if got := lstatGid(t, link); got != linkGid {
		t.Errorf("gid of the symlink itself = %d, want %d", got, linkGid)
	}
}

func TestChgrpSymlinkNoDereference(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	target := writeFile(t, dir, "target", "x")
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	targetGid := statGid(t, target)

	r := runBuiltinIn(t, dir, "chgrp", "-h", strconv.Itoa(other), "link")
	assertExitCode(t, r, 0)
	assertNoStderr(t, r)
	if got := lstatGid(t, link); got != other {
		t.Errorf("gid of the symlink itself = %d, want %d", got, other)
	}
	wantGid(t, target, targetGid)
}

func TestChgrpReference(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	ref := writeFile(t, dir, "ref", "x")
	target := writeFile(t, dir, "target", "x")
	refGid := statGid(t, ref)
	targetGid := statGid(t, target)
	if refGid == targetGid {
		// Make the reference differ using the other group we belong to.
		if r := runBuiltinIn(t, dir, "chgrp", strconv.Itoa(other), "ref"); r.code != 0 {
			t.Fatalf("setup chgrp failed: %+v", r)
		}
	}

	for _, args := range [][]string{
		{"--reference=ref", "target"},
		{"--reference", "ref", "target"},
	} {
		wantRefGid := statGid(t, ref)
		r := runBuiltinIn(t, dir, "chgrp", args...)
		assertExitCode(t, r, 0)
		assertNoStderr(t, r)
		wantGid(t, target, wantRefGid)
	}
}

func TestChgrpReferenceMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "x")
	before := statGid(t, p)

	r := runBuiltinIn(t, dir, "chgrp", "--reference=nope", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "failed to get attributes of 'nope'")
	wantGid(t, p, before)
}

func TestChgrpVerbose(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "x")
	writeFile(t, dir, "sub/g.txt", "x")

	r := runBuiltinIn(t, dir, "chgrp", "-v", strconv.Itoa(other), "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed group of 'f.txt'\n")

	r = runBuiltinIn(t, dir, "chgrp", "-Rv", strconv.Itoa(other), "sub")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed group of 'sub'\nchanged group of 'sub/g.txt'\n")
}

func TestChgrpChanges(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "x")

	r := runBuiltinIn(t, dir, "chgrp", "-c", strconv.Itoa(other), "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed group of 'f.txt'\n")

	// Same group again: nothing to report.
	r = runBuiltinIn(t, dir, "chgrp", "-c", strconv.Itoa(other), "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	assertNoStderr(t, r)

	// -v still reports the no-op.
	r = runBuiltinIn(t, dir, "chgrp", "-v", strconv.Itoa(other), "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "changed group of 'f.txt'\n")
}

func TestChgrpSilent(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "chgrp", "-f", "0", "nope.txt")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertNoStderr(t, r)
}

func TestChgrpInvalidGroup(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f", "x")
	before := statGid(t, p)

	r := runBuiltinIn(t, dir, "chgrp", "nosuchgroup_xyz_123", "f")
	assertExitCode(t, r, 1)
	if r.stderr != "chgrp: invalid group: 'nosuchgroup_xyz_123'\n" {
		t.Errorf("stderr = %q, want %q", r.stderr, "chgrp: invalid group: 'nosuchgroup_xyz_123'\n")
	}
	assertStdout(t, r, "")
	wantGid(t, p, before)
}

func TestChgrpMissingOperand(t *testing.T) {
	r := runBuiltin(t, "chgrp")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing operand")

	r = runBuiltin(t, "chgrp", "1000")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing operand")

	r = runBuiltin(t, "chgrp", "--reference=/etc/hostname")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing operand")
}

func TestChgrpContinuesAfterError(t *testing.T) {
	_, other := chgrpTestGroups(t)
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "x")

	r := runBuiltinIn(t, dir, "chgrp", strconv.Itoa(other), "nope.txt", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "changing group of 'nope.txt'")
	wantGid(t, p, other)
}

func TestChgrpNotPermitted(t *testing.T) {
	chgrpRequireNoRootGroup(t)
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "x")
	before := statGid(t, p)

	r := runBuiltinIn(t, dir, "chgrp", "0", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "chgrp: changing group of 'f.txt': ")
	wantGid(t, p, before)

	// -f silences the message but keeps the exit status.
	r = runBuiltinIn(t, dir, "chgrp", "-f", "0", "f.txt")
	assertExitCode(t, r, 1)
	assertNoStderr(t, r)
	wantGid(t, p, before)
}

func TestChgrpUnsupportedOptions(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f.txt", "x")
	before := statGid(t, p)

	// --from is deliberately unsupported: it must fail, not silently ignore.
	r := runBuiltinIn(t, dir, "chgrp", "--from=1000", "1000", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option --from")
	if !strings.HasPrefix(r.stderr, "chgrp: ") {
		t.Errorf("stderr = %q, want a chgrp: prefix", r.stderr)
	}

	r = runBuiltinIn(t, dir, "chgrp", "-Z", "1000", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option -Z")

	wantGid(t, p, before)
}

func TestChgrpEmptyGroupIsNoOp(t *testing.T) {
	// An empty GROUP leaves the group alone and still succeeds (uutils
	// test_no_change).
	dir := t.TempDir()
	p := writeFile(t, dir, "f", "x")
	before := statGid(t, p)

	r := runBuiltinIn(t, dir, "chgrp", "", "f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	assertNoStderr(t, r)
	wantGid(t, p, before)
}

func TestChgrpHelpAndRegistry(t *testing.T) {
	if _, ok := Lookup("chgrp"); !ok {
		t.Fatal("chgrp is not registered")
	}
	r := runBuiltin(t, "chgrp", "--help")
	assertExitCode(t, r, 0)
	// runBuiltinIn leaves CmdName empty, so only the flag list is checked here;
	// the shell fills the command name in and prints "Usage: chgrp ...".
	assertStdoutContains(t, r, "Usage:")
	assertStdoutContains(t, r, "--reference=VALUE")
	assertStdoutContains(t, r, "--no-dereference")
	assertStdoutContains(t, r, "--recursive")
	assertNoStderr(t, r)

	r = runBuiltin(t, "chgrp", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "0.1.0")
}
