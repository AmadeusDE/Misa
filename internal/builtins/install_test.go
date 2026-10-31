package builtins

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

// installRawMode returns the full 07777 mode of path (permissions plus
// setuid/setgid/sticky), which os.FileMode.Perm alone hides.
func installRawMode(t *testing.T, path string) uint32 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat %s: no syscall.Stat_t", path)
	}
	return uint32(st.Mode) & 07777
}

func installWantMode(t *testing.T, path string, want uint32) {
	t.Helper()
	if got := installRawMode(t, path); got != want {
		t.Errorf("%s: mode = %04o, want %04o", path, got, want)
	}
}

func installRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// installTempFiles lists the temporary files install may have left behind.
func installTempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func installWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestInstallCopiesWithDefaultMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	installWrite(t, src, "data\n")
	if err := os.Chmod(src, 0600); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "install", "src", "dst")
	assertExitCode(t, r, 0)
	if r.stdout != "" || r.stderr != "" {
		t.Errorf("output = %q / %q, want silence", r.stdout, r.stderr)
	}
	if got := installRead(t, filepath.Join(dir, "dst")); got != "data\n" {
		t.Errorf("content = %q, want %q", got, "data\n")
	}
	installWantMode(t, filepath.Join(dir, "dst"), 0755)
	if got := installRawMode(t, src); got != 0600 {
		t.Errorf("source mode changed to %04o, want 0600", got)
	}
}

func TestInstallMode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want uint32
	}{
		{"octal", []string{"-m", "0644"}, 0644},
		{"octal setuid setgid", []string{"-m", "2750"}, 02750},
		{"octal sticky", []string{"-m", "1777"}, 01777},
		{"symbolic who op", []string{"-m", "o+wx"}, 0003},
		{"symbolic comma separated", []string{"-m", "ug+rwX,o+rX"}, 0664},
		{"symbolic no who", []string{"-m", "+w"}, 0222},
		{"symbolic equal", []string{"-m", "a=r"}, 0444},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			installWrite(t, filepath.Join(dir, "src"), "x")
			args := append(append([]string{}, tc.args...), "src", "dst")
			r := runBuiltinIn(t, dir, "install", args...)
			assertExitCode(t, r, 0)
			installWantMode(t, filepath.Join(dir, "dst"), tc.want)
		})
	}
}

// A file's mode differs from a directory's for the symbolic X clause: install
// -d must give "X" the execute bit, install of a file must not.
func TestInstallModeDirectoryUsesDirSemantics(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "install", "-d", "-m", "ug+rwX,o+rX", "test_dir")
	assertExitCode(t, r, 0)
	installWantMode(t, filepath.Join(dir, "test_dir"), 0775)

	r = runBuiltinIn(t, dir, "install", "-d", "-m", "+x", "xdir")
	assertExitCode(t, r, 0)
	installWantMode(t, filepath.Join(dir, "xdir"), 0111)
}

func TestInstallModeInvalid(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	for _, mode := range []string{"999", "u+q", ""} {
		r := runBuiltinIn(t, dir, "install", "-m", mode, "src", "dst")
		assertExitCode(t, r, 1)
		if r.stderr == "" {
			t.Errorf("-m %q: no diagnostic", mode)
		}
		if _, err := os.Stat(filepath.Join(dir, "dst")); err == nil {
			t.Errorf("-m %q: destination was created", mode)
		}
	}
}

func TestInstallDirectoryCreatesParents(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "install", "-d", "a/b/c")
	assertExitCode(t, r, 0)
	for _, p := range []string{"a", "a/b", "a/b/c"} {
		if !isDirPath(filepath.Join(dir, p)) {
			t.Fatalf("%s was not created", p)
		}
		// Every component, not just the target, is created with the default
		// mode and is not limited by the umask.
		installWantMode(t, filepath.Join(dir, p), 0755)
	}
}

func TestInstallDirectoryModeAppliesToTargetOnly(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "install", "-d", "-m", "02750", "a/b")
	assertExitCode(t, r, 0)
	installWantMode(t, filepath.Join(dir, "a"), 0755)
	installWantMode(t, filepath.Join(dir, "a/b"), 02750)
}

func TestInstallDirectoryExistingPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "existing"), 0700); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "install", "-d", "existing")
	assertExitCode(t, r, 0)
	installWantMode(t, filepath.Join(dir, "existing"), 0755)

	installWrite(t, filepath.Join(dir, "file"), "x")
	r = runBuiltinIn(t, dir, "install", "-d", "file")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot create directory 'file'")
}

// Several -d operands are processed independently: one failure does not stop
// the rest, and the exit status still reports it.
func TestInstallDirectoryMultipleOperands(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "file"), "x")
	r := runBuiltinIn(t, dir, "install", "-d", "dir1", "file", "dir2")
	assertExitCode(t, r, 1)
	for _, p := range []string{"dir1", "dir2"} {
		if !isDirPath(filepath.Join(dir, p)) {
			t.Errorf("%s was not created", p)
		}
	}
	assertStderrContains(t, r, "cannot create directory 'file'")
}

func TestInstallDirectoryDotOperands(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"dir1/.", "dir6/./"} {
		r := runBuiltinIn(t, dir, "install", "-d", "-v", name)
		assertExitCode(t, r, 0)
		assertStdoutContains(t, r, "install: creating directory '"+installDirDisplay(name)+"'")
	}
	if !isDirPath(filepath.Join(dir, "dir1")) || !isDirPath(filepath.Join(dir, "dir6")) {
		t.Errorf("dot operands did not create their directories")
	}
}

func TestInstallCreateLeadingDirectories(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "payload\n")
	r := runBuiltinIn(t, dir, "install", "-D", "src", "d1/d2/d3/target")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "d1/d2/d3/target")); got != "payload\n" {
		t.Errorf("content = %q", got)
	}
	installWantMode(t, filepath.Join(dir, "d1"), 0755)
	installWantMode(t, filepath.Join(dir, "d1/d2/d3"), 0755)
	installWantMode(t, filepath.Join(dir, "d1/d2/d3/target"), 0755)
}

func TestInstallCreateLeadingVerbose(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	r := runBuiltinIn(t, dir, "install", "-Dv", "src", "d1/d2/target")
	assertExitCode(t, r, 0)
	// The creation line names the directory the user wrote, not the resolved path.
	assertStdoutContains(t, r, "install: creating directory 'd1/d2'")
	assertStdoutContains(t, r, "'src' -> 'd1/d2/target'")
}

func TestInstallCreateLeadingWithTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	r := runBuiltinIn(t, dir, "install", "-D", "-t", "missing", "src")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "missing/src")); got != "x" {
		t.Errorf("content = %q", got)
	}
}

// Without -t, -D must not invent a directory target for several sources: that
// would silently install files somewhere the user did not name.
func TestInstallCreateLeadingMultipleSourcesNeedsTargetDir(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "a"), "a")
	installWrite(t, filepath.Join(dir, "b"), "b")
	r := runBuiltinIn(t, dir, "install", "-D", "a", "b", "missing")
	assertExitCode(t, r, 1)
	if isDirPath(filepath.Join(dir, "missing")) {
		t.Error("missing target directory was created without -t")
	}
	assertStderrContains(t, r, "missing")
}

func TestInstallMultipleSourcesIntoDirectory(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "a"), "a")
	installWrite(t, filepath.Join(dir, "b"), "b")
	installWrite(t, filepath.Join(dir, "x"), "x")
	if err := os.Mkdir(filepath.Join(dir, "dst"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "install", "a", "b", "x", "dst")
	assertExitCode(t, r, 0)
	if r.stderr != "" {
		t.Errorf("stderr = %q", r.stderr)
	}
	for _, name := range []string{"a", "b", "x"} {
		if got := installRead(t, filepath.Join(dir, "dst", name)); got != name {
			t.Errorf("dst/%s = %q", name, got)
		}
		installWantMode(t, filepath.Join(dir, "dst", name), 0755)
	}
	if got := installTempFiles(t, filepath.Join(dir, "dst")); len(got) != 0 {
		t.Errorf("temporary files left behind: %v", got)
	}
}

func TestInstallTargetDirectoryFlag(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	if err := os.Mkdir(filepath.Join(dir, "dst"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "install", "-t", "dst", "src")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "dst/src")); got != "x" {
		t.Errorf("content = %q", got)
	}

	// -t names a directory even for one source: an existing file target is an
	// error rather than something to overwrite.
	installWrite(t, filepath.Join(dir, "plain"), "old")
	r = runBuiltinIn(t, dir, "install", "-t", "plain", "src")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "failed to access 'plain': Not a directory")
	if got := installRead(t, filepath.Join(dir, "plain")); got != "old" {
		t.Errorf("the -t target was modified: %q", got)
	}

	// A missing -t directory needs -D.
	r = runBuiltinIn(t, dir, "install", "-t", "absent", "src")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "failed to access 'absent'")
}

func TestInstallNoTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "new")
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0755); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "install", "-T", "src", "dst")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "dst")); got != "new" {
		t.Errorf("content = %q", got)
	}
	// Installing again over the same name is an overwrite, not an error.
	r = runBuiltinIn(t, dir, "install", "-T", "src", "dst")
	assertExitCode(t, r, 0)

	r = runBuiltinIn(t, dir, "install", "-T", "src", "adir")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot overwrite directory 'adir' with non-directory")

	r = runBuiltinIn(t, dir, "install", "-T", "src", "src", "adir")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "extra operand 'adir'")

	r = runBuiltinIn(t, dir, "install", "-T", "-t", "adir", "src")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "mutually exclusive")
}

// A read-only destination is still replaced: the copy is renamed over it, and
// renaming needs only a writable directory.
func TestInstallOverwritesReadOnlyDestination(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "new content")
	target := filepath.Join(dir, "dst")
	installWrite(t, target, "old content")
	if err := os.Chmod(target, 0400); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "install", "src", "dst")
	assertExitCode(t, r, 0)
	if got := installRead(t, target); got != "new content" {
		t.Errorf("content = %q, want %q", got, "new content")
	}
	installWantMode(t, target, 0755)
	if got := installTempFiles(t, dir); len(got) != 0 {
		t.Errorf("temporary files left behind: %v", got)
	}
}

func TestInstallFailuresLeaveNoTrace(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "dst"), "original")
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing source", []string{"absent", "new"}, "cannot stat 'absent': No such file or directory"},
		{"directory source", []string{"adir", "new"}, "omitting directory 'adir'"},
		{"same file", []string{"-D", "dst", "dst"}, "'dst' and 'dst' are the same file"},
		{"missing destination parent", []string{"dst", "no/such/dir/file"}, "cannot create regular file 'no/such/dir/file'"},
		{"unknown option", []string{"--strip-program=x", "dst", "new"}, "unknown option --strip-program"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinIn(t, dir, "install", tc.args...)
			assertExitCode(t, r, 1)
			assertStderrContains(t, r, tc.want)
			if got := installRead(t, filepath.Join(dir, "dst")); got != "original" {
				t.Errorf("destination was modified: %q", got)
			}
			if got := installTempFiles(t, dir); len(got) != 0 {
				t.Errorf("temporary files left behind: %v", got)
			}
		})
	}
}

func TestInstallStripRejected(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	for _, args := range [][]string{{"-s", "src", "dst"}, {"--strip", "src", "dst"}} {
		r := runBuiltinIn(t, dir, "install", args...)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "stripping (-s) is not supported")
		if _, err := os.Stat(filepath.Join(dir, "dst")); err == nil {
			t.Error("destination was created despite -s")
		}
	}
}

func TestInstallCompare(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "payload")
	r := runBuiltinIn(t, dir, "install", "src", "dst")
	assertExitCode(t, r, 0)

	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(filepath.Join(dir, "dst"), old, old); err != nil {
		t.Fatal(err)
	}
	r = runBuiltinIn(t, dir, "install", "-C", "-v", "src", "dst")
	assertExitCode(t, r, 0)
	if r.stdout != "" {
		t.Errorf("-C -v reported a skipped copy: %q", r.stdout)
	}
	info, err := os.Stat(filepath.Join(dir, "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("destination was rewritten by -C (%v != %v)", info.ModTime(), old)
	}

	// Different contents force the copy.
	installWrite(t, filepath.Join(dir, "src"), "different")
	r = runBuiltinIn(t, dir, "install", "-C", "src", "dst")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "dst")); got != "different" {
		t.Errorf("content = %q", got)
	}

	// A timestamp difference forces the copy when -p asks for the source's time.
	installWrite(t, filepath.Join(dir, "src"), "different")
	if err := os.Chtimes(filepath.Join(dir, "src"), old, old); err != nil {
		t.Fatal(err)
	}
	r = runBuiltinIn(t, dir, "install", "-C", "-p", "src", "dst")
	assertExitCode(t, r, 0)
	info, err = os.Stat(filepath.Join(dir, "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("-C -p did not preserve the source time: %v != %v", info.ModTime(), old)
	}
}

func TestInstallCompareMode(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	r := runBuiltinIn(t, dir, "install", "src", "dst")
	assertExitCode(t, r, 0)

	// The destination is right except for its mode, so -C -m 0600 must rewrite it.
	r = runBuiltinIn(t, dir, "install", "-C", "-m", "0600", "src", "dst")
	assertExitCode(t, r, 0)
	installWantMode(t, filepath.Join(dir, "dst"), 0600)
}

func TestInstallPreserveTimestamps(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	installWrite(t, src, "x")
	stamp := time.Unix(1000000, 0)
	if err := os.Chtimes(src, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "install", "-p", "src", "dst")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(stamp) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), stamp)
	}
}

func TestInstallVerbose(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	r := runBuiltinIn(t, dir, "install", "-v", "src", "dst")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "'src' -> 'dst'\n")
}

func TestInstallSymlinkSourceIsFollowed(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "real"), "target content")
	if err := os.Symlink("real", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "install", "link", "dst")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "dst")); got != "target content" {
		t.Errorf("content = %q", got)
	}
	info, err := os.Lstat(filepath.Join(dir, "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("destination is a symlink, want a regular file")
	}
}

// A symlink destination is replaced, not written through: the link's target
// keeps its content and the link becomes a regular file.
func TestInstallReplacesSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "new")
	installWrite(t, filepath.Join(dir, "target"), "old")
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "install", "src", "link")
	assertExitCode(t, r, 0)

	info, err := os.Lstat(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("destination is still a symlink")
	}
	if got := installRead(t, filepath.Join(dir, "link")); got != "new" {
		t.Errorf("link content = %q", got)
	}
	if got := installRead(t, filepath.Join(dir, "target")); got != "old" {
		t.Errorf("link target was modified: %q", got)
	}
}

func TestInstallGroupAndOwner(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	ownGid := os.Getgid()
	r := runBuiltinIn(t, dir, "install", "-g", itoa(ownGid), "src", "dst")
	assertExitCode(t, r, 0)
	st, err := os.Stat(filepath.Join(dir, "dst"))
	if err != nil {
		t.Fatal(err)
	}
	if gid := int(st.Sys().(*syscall.Stat_t).Gid); gid != ownGid {
		t.Errorf("gid = %d, want %d", gid, ownGid)
	}

	r = runBuiltinIn(t, dir, "install", "-o", itoa(os.Geteuid()), "src", "dst2")
	assertExitCode(t, r, 0)

	if os.Geteuid() != 0 && !installInGroup(0) {
		r = runBuiltinIn(t, dir, "install", "-g", "0", "src", "dst3")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "cannot change ownership")
		if _, err := os.Stat(filepath.Join(dir, "dst3")); err == nil {
			t.Error("destination exists despite the failed chown")
		}
		if got := installTempFiles(t, dir); len(got) != 0 {
			t.Errorf("temporary files left behind: %v", got)
		}
	}
}

func TestInstallInvalidOwnerAndGroup(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	r := runBuiltinIn(t, dir, "install", "-o", "definitely-not-a-user", "src", "dst")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid user")
	r = runBuiltinIn(t, dir, "install", "-g", "definitely-not-a-group", "src", "dst")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid group")
}

func TestInstallDirectoryOwnerAndGroup(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "install", "-d", "-g", itoa(os.Getgid()), "d")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "d"))
	if err != nil {
		t.Fatal(err)
	}
	if gid := int(info.Sys().(*syscall.Stat_t).Gid); gid != os.Getgid() {
		t.Errorf("gid = %d, want %d", gid, os.Getgid())
	}
}

func TestInstallMissingOperands(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "install")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing file operand")

	r = runBuiltinIn(t, dir, "install", "onlyone")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing destination file operand after 'onlyone'")

	r = runBuiltinIn(t, dir, "install", "-d")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing file operand")
}

func TestInstallIgnoredCompareFlag(t *testing.T) {
	dir := t.TempDir()
	installWrite(t, filepath.Join(dir, "src"), "x")
	r := runBuiltinIn(t, dir, "install", "-c", "src", "dst")
	assertExitCode(t, r, 0)
	if got := installRead(t, filepath.Join(dir, "dst")); got != "x" {
		t.Errorf("content = %q", got)
	}
}

func TestInstallHelp(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "install", "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{"Usage: install", "--target-directory", "--no-target-directory", "not supported"} {
		assertStdoutContains(t, r, want)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q", r.stderr)
	}
}

// itoa avoids importing strconv in an otherwise table-driven file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// installInGroup reports whether the process belongs to gid.
func installInGroup(gid int) bool {
	if gid == os.Getgid() {
		return true
	}
	gids, err := os.Getgroups()
	if err != nil {
		return false
	}
	for _, g := range gids {
		if g == gid {
			return true
		}
	}
	return false
}
