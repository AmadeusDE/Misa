package native

// End-to-end tests for the chgrp builtin, through the builtins registry.
//
// TestChgrp_SymlinkPolicy is a port of toybox's tests/chgrp.test
// (https://github.com/landley/toybox/blob/6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb/tests/chgrp.test,
// 0BSD licence, © Rob Landley): same hierarchy, same case list, same slot
// order. toybox runs it as root and alternates between "root" and the last
// group in /etc/group; these tests must pass unprivileged, so they alternate
// between "home" (the process's own gid, which every file it creates starts
// in) and "to" (another gid the process belongs to, from syscall.Getgroups).
// A slot toybox expects to become "root" must become "to" here, and every
// other slot must keep the gid it had before the run (toybox's "$GRP").
//
// The placement asserted below is what toybox's cases expect: without -H/-L,
// -R implies -h, so recursion changes symlinks themselves instead of their
// targets (a link is dereferenced only with -H or -L).
//
// The remaining cases mirror toybox's and uutils coreutils' tests/by-util/
// test_chgrp.rs (https://github.com/uutils/coreutils/blob/fee61ff5d4f87e0f912a52e639942b5f7ad58663/tests/by-util/test_chgrp.rs,
// MIT): the invalid-group wording, --reference, -f suppressing the message but
// not the exit status, -R on a regular file, and the exit status when the last
// file succeeds but an earlier one failed.

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// chgrpHomeGid is the group every file these tests create starts in: the
// process's own gid, which it may always set.
func chgrpHomeGid() int { return os.Getgid() }

// chgrpSecondGid returns a group the process belongs to that is not its own
// gid, or -1 when it belongs to a single group.
func chgrpSecondGid() int {
	gids, err := syscall.Getgroups()
	if err != nil {
		return -1
	}
	for _, g := range gids {
		if g != os.Getgid() {
			return g
		}
	}
	return -1
}

// chgrpToGid is the "changed to" group of every test below.
func chgrpToGid(t *testing.T) int {
	t.Helper()
	g := chgrpSecondGid()
	if g < 0 {
		t.Skip("the test process belongs to a single group: no second gid to chgrp to")
	}
	return g
}

// chgrpRequireGid0Unreachable skips the cases that expect EPERM: only root
// (or a member of group 0) may change a file's group to gid 0.
func chgrpRequireGid0Unreachable(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: changing a file's group to gid 0 is permitted")
	}
	gids, err := syscall.Getgroups()
	if err != nil {
		t.Fatalf("getgroups: %v", err)
	}
	for _, g := range append([]int{os.Getgid()}, gids...) {
		if g == 0 {
			t.Skip("the test process belongs to group 0: gid 0 is settable")
		}
	}
}

// gidOf reports the gid stat(2) sees, i.e. of the symlink target.
func gidOf(t *testing.T, path string) int {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return statGidOf(t, path, info)
}

// linkGidOf reports the gid lstat(2) sees, i.e. of the symlink itself (that is
// the number `ls -l` prints, which is what the toybox table compares).
func linkGidOf(t *testing.T, path string) int {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	return statGidOf(t, path, info)
}

func statGidOf(t *testing.T, path string, info os.FileInfo) int {
	t.Helper()
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat %s: not a syscall.Stat_t", path)
	}
	return int(st.Gid)
}

// setGid is test setup, not the code under test: it puts path (a symlink
// included) in gid with lchown.
func setGid(t *testing.T, path string, gid int) {
	t.Helper()
	if err := os.Lchown(path, -1, gid); err != nil {
		t.Fatalf("lchown %s to %d: %v", path, gid, err)
	}
}

func wantGidOf(t *testing.T, path string, want int) {
	t.Helper()
	if got := gidOf(t, path); got != want {
		t.Errorf("gid of %s = %d, want %d", filepath.Base(path), got, want)
	}
}

func wantLinkGid(t *testing.T, path string, want int) {
	t.Helper()
	if got := linkGidOf(t, path); got != want {
		t.Errorf("gid of the symlink %s itself = %d, want %d", filepath.Base(path), got, want)
	}
}

// TestChgrp_NamedGroup changes a file's group by group name, to the group the
// test process itself runs in.
func TestChgrp_NamedGroup(t *testing.T) {
	home := chgrpHomeGid()
	g, err := user.LookupGroupId(strconv.Itoa(home))
	if err != nil {
		t.Skipf("no group name for gid %d: %v", home, err)
	}
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "content")
	// Start elsewhere when possible, so the named form is a real change.
	if to := chgrpSecondGid(); to >= 0 {
		setGid(t, p, to)
	}

	r := runIn(t, dir, "chgrp", g.Name, "f.txt")
	wantCode(t, r, 0)
	wantOut(t, r, "")
	wantErr(t, r, "")
	wantGidOf(t, p, home)
}

// TestChgrp_NumericGroup is the same change with the gid written as digits.
func TestChgrp_NumericGroup(t *testing.T) {
	home := chgrpHomeGid()
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "content")
	if to := chgrpSecondGid(); to >= 0 {
		setGid(t, p, to)
	}

	r := runIn(t, dir, "chgrp", strconv.Itoa(home), "f.txt")
	wantCode(t, r, 0)
	wantOut(t, r, "")
	wantErr(t, r, "")
	wantGidOf(t, p, home)
}

// TestChgrp_Recursive walks a nested tree.
func TestChgrp_Recursive(t *testing.T) {
	home, to := chgrpHomeGid(), chgrpToGid(t)
	dir := t.TempDir()
	for _, name := range []string{"top/f.txt", "top/sub/g.txt", "top/sub/deeper/h.txt"} {
		mkFile(t, dir, name, "x")
	}

	r := runIn(t, dir, "chgrp", "-R", strconv.Itoa(to), "top")
	wantCode(t, r, 0)
	wantOut(t, r, "")
	wantErr(t, r, "")
	for _, name := range []string{"top", "top/f.txt", "top/sub", "top/sub/g.txt", "top/sub/deeper", "top/sub/deeper/h.txt"} {
		wantGidOf(t, filepath.Join(dir, name), to)
	}
	wantGidOf(t, dir, home) // the tree's parent is not the operand
}

// TestChgrp_RecursiveRegularFile: -R on a plain file has nothing to recurse
// into and must still succeed.
func TestChgrp_RecursiveRegularFile(t *testing.T) {
	to := chgrpToGid(t)
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")

	r := runIn(t, dir, "chgrp", "-R", strconv.Itoa(to), "f.txt")
	wantCode(t, r, 0)
	wantOut(t, r, "")
	wantErr(t, r, "")
	wantGidOf(t, p, to)
}

// chgrpTreeSlots are the seven paths toybox's `ls -lR` prints, in its order.
var chgrpTreeSlots = []string{
	"dir", "dir2", "dir/dir", "dir/file", "dir/dir/dir", "dir2/dir", "dir2/file",
}

// chgrpTree builds toybox's hierarchy under dir:
//
//	dir/dir/dir     directories
//	dir/file        regular file
//	dir2/dir  -> ../dir/dir
//	dir2/file -> ../dir/file
func chgrpTree(t *testing.T, dir string) {
	t.Helper()
	for _, d := range []string{"dir/dir/dir", "dir2"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	mkFile(t, dir, "dir/file", "")
	if err := os.Symlink("../dir/dir", filepath.Join(dir, "dir2/dir")); err != nil {
		t.Fatalf("symlink dir2/dir: %v", err)
	}
	if err := os.Symlink("../dir/file", filepath.Join(dir, "dir2/file")); err != nil {
		t.Fatalf("symlink dir2/file: %v", err)
	}
}

// chgrpSymlinkCases is toybox's case list, one case per row: args is the
// command line after the program name ("%g" is replaced by the "to" gid) and
// changed names the slots toybox expects to become "root" (here: "to"); every
// other slot must keep its previous gid.
var chgrpSymlinkCases = []struct {
	name    string
	args    []string
	changed []string
}{
	// Basic smoketest.
	{"dir", []string{"%g", "dir"}, []string{"dir"}},
	{"file", []string{"%g", "dir/file"}, []string{"dir/file"}},
	{"dir and file", []string{"%g", "dir", "dir/file"}, []string{"dir", "dir/file"}},

	// Symlinks given on the command line affect the target, not the link.
	{"symlink->file", []string{"%g", "dir2/file"}, []string{"dir/file"}},
	{"symlink->dir", []string{"%g", "dir2/dir"}, []string{"dir/dir"}},
	{"-h symlink->file", []string{"-h", "%g", "dir2/file"}, []string{"dir2/file"}},
	{"-h symlink->dir", []string{"-h", "%g", "dir2/dir"}, []string{"dir2/dir"}},

	// -R: -h is implied, so links met while recursing change themselves.
	{"-R dir", []string{"-R", "%g", "dir"}, []string{"dir", "dir/dir", "dir/file", "dir/dir/dir"}},
	{"-R dir2", []string{"-R", "%g", "dir2"}, []string{"dir2", "dir2/dir", "dir2/file"}},
	{"-R symlink->dir", []string{"-R", "%g", "dir2/dir"}, []string{"dir2/dir"}},
	{"-R symlink->file", []string{"-R", "%g", "dir2/file"}, []string{"dir2/file"}},

	// -RP is the same as -R by itself.
	{"-RP dir2", []string{"-RP", "%g", "dir2"}, []string{"dir2", "dir2/dir", "dir2/file"}},
	{"-RP symlink->dir", []string{"-RP", "%g", "dir2/dir"}, []string{"dir2/dir"}},
	{"-RP symlink->file", []string{"-RP", "%g", "dir2/file"}, []string{"dir2/file"}},

	// -RH: change the target, but only recurse through a symlink named on the
	// command line.
	{"-RH dir2", []string{"-RH", "%g", "dir2"}, []string{"dir2", "dir/dir", "dir/file"}},
	{"-RH symlink->dir", []string{"-RH", "%g", "dir2/dir"}, []string{"dir/dir", "dir/dir/dir"}},
	{"-RH symlink->file", []string{"-RH", "%g", "dir2/file"}, []string{"dir/file"}},

	// -RL: change the target and always recurse through it.
	{"-RL dir2", []string{"-RL", "%g", "dir2"}, []string{"dir2", "dir/dir", "dir/file", "dir/dir/dir"}},
	{"-RL symlink->dir", []string{"-RL", "%g", "dir2/dir"}, []string{"dir/dir", "dir/dir/dir"}},
	{"-RL symlink->file", []string{"-RL", "%g", "dir2/file"}, []string{"dir/file"}},

	// -H, -L and -P are no-ops without -R.
	{"-H without -R", []string{"-H", "%g", "dir2/dir"}, []string{"dir/dir"}},
	{"-L without -R", []string{"-L", "%g", "dir2/dir"}, []string{"dir/dir"}},
	{"-P without -R", []string{"-P", "%g", "dir2/dir"}, []string{"dir/dir"}},
}

// TestChgrp_SymlinkPolicy runs the toybox table over one shared hierarchy,
// resetting every slot to "home" before each case.
func TestChgrp_SymlinkPolicy(t *testing.T) {
	home, to := chgrpHomeGid(), chgrpToGid(t)
	dir := t.TempDir()
	chgrpTree(t, dir)

	for _, c := range chgrpSymlinkCases {
		t.Run(c.name, func(t *testing.T) {
			for _, slot := range chgrpTreeSlots {
				setGid(t, filepath.Join(dir, slot), home)
			}
			want := make(map[string]int, len(chgrpTreeSlots))
			for _, slot := range chgrpTreeSlots {
				want[slot] = home
			}
			for _, slot := range c.changed {
				want[slot] = to
			}

			args := make([]string, len(c.args))
			for i, a := range c.args {
				args[i] = strings.ReplaceAll(a, "%g", strconv.Itoa(to))
			}
			r := runIn(t, dir, "chgrp", args...)
			wantCode(t, r, 0)
			wantOut(t, r, "")
			wantErr(t, r, "")

			for _, slot := range chgrpTreeSlots {
				if got := linkGidOf(t, filepath.Join(dir, slot)); got != want[slot] {
					t.Errorf("%s: gid = %d, want %d (toybox: %s)",
						slot, got, want[slot], toyboxVerdict(want[slot] == to))
				}
			}
		})
	}
}

// toyboxVerdict spells a slot's expectation the way toybox's table does.
func toyboxVerdict(changed bool) string {
	if changed {
		return "changed"
	}
	return "unchanged"
}

// TestChgrp_Verbose checks -v's exact wording, one line per file.
func TestChgrp_Verbose(t *testing.T) {
	to := chgrpToGid(t)
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x")
	mkFile(t, dir, "sub/g.txt", "x")

	r := runIn(t, dir, "chgrp", "-v", strconv.Itoa(to), "f.txt")
	wantCode(t, r, 0)
	wantOut(t, r, "changed group of 'f.txt'\n")
	wantErr(t, r, "")

	r = runIn(t, dir, "chgrp", "-Rv", strconv.Itoa(to), "sub")
	wantCode(t, r, 0)
	wantOut(t, r, "changed group of 'sub'\nchanged group of 'sub/g.txt'\n")
	wantErr(t, r, "")
}

// TestChgrp_Changes checks -c: only files whose gid actually changes are
// reported.
func TestChgrp_Changes(t *testing.T) {
	to := chgrpToGid(t)
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x")

	r := runIn(t, dir, "chgrp", "-c", strconv.Itoa(to), "f.txt")
	wantCode(t, r, 0)
	wantOut(t, r, "changed group of 'f.txt'\n")
	wantErr(t, r, "")

	// Already in that group: nothing to report, still a success.
	r = runIn(t, dir, "chgrp", "-c", strconv.Itoa(to), "f.txt")
	wantCode(t, r, 0)
	wantOut(t, r, "")
	wantErr(t, r, "")
}

// TestChgrp_Silent checks that -f hides the message of an unopenable file but
// not its exit status.
func TestChgrp_Silent(t *testing.T) {
	to := chgrpToGid(t)
	dir := t.TempDir()

	r := runIn(t, dir, "chgrp", "-f", strconv.Itoa(to), "nope.txt")
	wantCode(t, r, 1)
	wantOut(t, r, "")
	wantErr(t, r, "")

	// Without -f the very same failure is reported.
	r = runIn(t, dir, "chgrp", strconv.Itoa(to), "nope.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "chgrp: changing group of 'nope.txt': ")
}

// TestChgrp_Reference copies the group of --reference's file, following a
// symlink given as the reference.
func TestChgrp_Reference(t *testing.T) {
	home, to := chgrpHomeGid(), chgrpToGid(t)
	dir := t.TempDir()
	ref := mkFile(t, dir, "ref", "x")
	target := mkFile(t, dir, "target", "x")
	setGid(t, ref, to)
	setGid(t, target, home)

	for _, args := range [][]string{
		{"--reference=ref", "target"},
		{"--reference", "ref", "target"},
	} {
		setGid(t, target, home)
		r := runIn(t, dir, "chgrp", args...)
		wantCode(t, r, 0)
		wantOut(t, r, "")
		wantErr(t, r, "")
		wantGidOf(t, target, to)
	}

	// stat(2) follows the reference, so the link's own gid does not matter.
	link := filepath.Join(dir, "reflink")
	symlink(t, "ref", link)
	setGid(t, link, home)
	setGid(t, target, home)
	r := runIn(t, dir, "chgrp", "--reference=reflink", "target")
	wantCode(t, r, 0)
	wantErr(t, r, "")
	wantGidOf(t, target, to)
	wantLinkGid(t, link, home) // the reference link itself is left alone
}

// TestChgrp_InvalidGroup checks the message and that nothing is touched.
func TestChgrp_InvalidGroup(t *testing.T) {
	home := chgrpHomeGid()
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")

	r := runIn(t, dir, "chgrp", "nosuchgroup_xyz_123", "f.txt")
	wantCode(t, r, 1)
	wantOut(t, r, "")
	wantErr(t, r, "chgrp: invalid group: 'nosuchgroup_xyz_123'\n")
	wantGidOf(t, p, home)
}

// TestChgrp_MissingOperand: no operands, or only a group.
func TestChgrp_MissingOperand(t *testing.T) {
	r := run(t, "chgrp")
	wantCode(t, r, 1)
	wantErrContains(t, r, "missing operand")

	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")
	home := chgrpHomeGid()

	r = runIn(t, dir, "chgrp", strconv.Itoa(home))
	wantCode(t, r, 1)
	wantErrContains(t, r, "missing operand")
	wantGidOf(t, p, home)

	// With --reference a single FILE operand is enough.
	r = runIn(t, dir, "chgrp", "--reference=f.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "missing operand")
}

// TestChgrp_ContinuesAfterError: a missing file is reported, the next operand
// is still changed, and the exit status is 1.
func TestChgrp_ContinuesAfterError(t *testing.T) {
	to := chgrpToGid(t)
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")

	r := runIn(t, dir, "chgrp", strconv.Itoa(to), "nope.txt", "f.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "chgrp: changing group of 'nope.txt': ")
	wantGidOf(t, p, to)
}

// TestChgrp_NotPermitted: changing to gid 0 is refused, -f hides the reason.
func TestChgrp_NotPermitted(t *testing.T) {
	chgrpRequireGid0Unreachable(t)
	home := chgrpHomeGid()
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")

	r := runIn(t, dir, "chgrp", "0", "f.txt")
	wantCode(t, r, 1)
	wantOut(t, r, "")
	wantErrContains(t, r, "changing group of")
	wantErrContains(t, r, "f.txt")
	wantGidOf(t, p, home)

	r = runIn(t, dir, "chgrp", "-f", "0", "f.txt")
	wantCode(t, r, 1)
	wantOut(t, r, "")
	wantErr(t, r, "")
	wantGidOf(t, p, home)
}

// TestChgrp_UnsupportedOptions: --from, --preserve-root/--no-preserve-root and
// --quiet are deliberately not implemented; they must fail as unknown options
// and leave the file alone.
func TestChgrp_UnsupportedOptions(t *testing.T) {
	home := chgrpHomeGid()
	group := strconv.Itoa(home)
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")

	for _, args := range [][]string{
		{"--from=1000", group, "f.txt"},
		{"--preserve-root", group, "f.txt"},
		{"--no-preserve-root", group, "f.txt"},
		{"--quiet", group, "f.txt"},
		{"-Z", group, "f.txt"},
	} {
		r := runIn(t, dir, "chgrp", args...)
		wantCode(t, r, 1)
		wantErrContains(t, r, "chgrp: ")
		wantGidOf(t, p, home)
	}
}
