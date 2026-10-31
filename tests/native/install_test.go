package native

// End-to-end tests for the install builtin, through the builtins registry.
//
// Case list ported from toybox tests/install.test
// (https://github.com/landley/toybox/blob/6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb/tests/install.test,
// 0BSD licence, © Rob Landley et al.): the -D/-Dt, -d, -dm octal and -dm
// symbolic cases run with the umask the toybox case sets. The remaining cases
// mirror uutils coreutils' tests/by-util/test_install.rs
// (https://github.com/uutils/coreutils/blob/fee61ff5d4f87e0f912a52e639942b5f7ad58663/tests/by-util/test_install.rs,
// MIT) where Misa implements the behaviour: ancestor modes, several -d
// operands, mode parsing, -t/-T, timestamp preservation and the usage errors.
// Cases that need root, SELinux contexts, backups or an external strip program
// are not ported; -s is rejected by design (see internal/builtins/install.go).

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func installMode(t *testing.T, path string) uint32 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return uint32(info.Sys().(*syscall.Stat_t).Mode) & 07777
}

func installWant(t *testing.T, path string, want uint32) {
	t.Helper()
	if got := installMode(t, path); got != want {
		t.Errorf("%s: mode = %04o, want %04o", path, got, want)
	}
}

// toybox: testing "install -D exists" "mkdir -p a; touch a/b; install -D random a/b && cmp random a/b && echo yes"
func TestInstall_ToyboxCreateLeadingExisting(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a/b", "")
	mkFile(t, dir, "random", "64 random bytes, standing in for dd")
	r := runIn(t, dir, "install", "-D", "random", "a/b")
	wantSuccess(t, r)
	if got, want := readFile(t, filepath.Join(dir, "a/b")), "64 random bytes, standing in for dd"; got != want {
		t.Errorf("a/b = %q, want %q", got, want)
	}
}

// toybox: testing "install -D -t creates directory" "touch a; install -Dt b a"
func TestInstall_ToyboxCreateLeadingTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a", "")
	r := runIn(t, dir, "install", "-Dt", "b", "a")
	wantSuccess(t, r)
	if !isDir(filepath.Join(dir, "b")) {
		t.Fatal("b was not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "b/a")); err != nil {
		t.Errorf("a was not installed into b: %v", err)
	}
}

// toybox: testing "install -d" "umask 0 && install -d potato && stat -c%a potato" -> 755
func TestInstall_ToyboxDirectory(t *testing.T) {
	dir := t.TempDir()
	r := runInWithUmask(t, dir, 0, "install", "-d", "potato")
	wantSuccess(t, r)
	installWant(t, filepath.Join(dir, "potato"), 0755)
}

// toybox: testcmd "-dm" "-dm 02750 potato" -> 2750
func TestInstall_ToyboxDirectoryModeOctal(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "install", "-dm", "02750", "potato")
	wantSuccess(t, r)
	installWant(t, filepath.Join(dir, "potato"), 02750)
}

// toybox: testcmd ” '-dm +x potato' -> 111
func TestInstall_ToyboxDirectoryModeSymbolic(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "install", "-dm", "+x", "potato")
	wantSuccess(t, r)
	installWant(t, filepath.Join(dir, "potato"), 0111)
}

// uutils: test_install_ancestors_mode_directories. Ancestors keep the default
// mode and the umask never narrows it; -m applies to the target only.
func TestInstall_AncestorModes(t *testing.T) {
	dir := t.TempDir()
	r := runInWithUmask(t, dir, 0077, "install", "-m", "200", "-d", "ancestor1/ancestor2/target_dir")
	wantSuccess(t, r)
	installWant(t, filepath.Join(dir, "ancestor1"), 0755)
	installWant(t, filepath.Join(dir, "ancestor1/ancestor2"), 0755)
	installWant(t, filepath.Join(dir, "ancestor1/ancestor2/target_dir"), 0200)
}

// The -Dv creation line names the directory as the user wrote it, not an
// absolute resolved path (uutils test_install_creating_leading_dirs_verbose).
func TestInstall_CreateLeadingVerboseNamesOperand(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "")
	if err := os.Mkdir(filepath.Join(dir, "dir1"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, "install", "-Dv", "file", "dir1/no-dir2/no-dir3/test_file")
	wantSuccess(t, r)
	if strings.Contains(r.stdout, "creating directory '") == false {
		t.Fatalf("stdout = %q", r.stdout)
	}
	if strings.Contains(r.stdout, "creating directory '"+dir) {
		t.Errorf("stdout names the resolved path: %q", r.stdout)
	}
	if !strings.Contains(r.stdout, "no-dir3") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if !isDir(filepath.Join(dir, "dir1/no-dir2/no-dir3")) {
		t.Error("leading directories were not created")
	}
}

// uutils: test_install_several_directories
func TestInstall_SeveralDirectories(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "install", "-d", "dir1", "dir2", "dir3")
	wantSuccess(t, r)
	for _, name := range []string{"dir1", "dir2", "dir3"} {
		if !isDir(filepath.Join(dir, name)) {
			t.Errorf("%s was not created", name)
		}
	}
}

// uutils: test_install_mode_numeric (both "--mode=333" and "-m 0333").
func TestInstall_ModeNumeric(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "")
	tests := []struct {
		dir  string
		args []string
	}{
		{"dir1", []string{"file", "dir1", "--mode=333"}},
		{"dir2", []string{"-m", "0333", "file", "dir2"}},
	}
	for _, tc := range tests {
		target := filepath.Join(dir, tc.dir)
		if err := os.Mkdir(target, 0755); err != nil {
			t.Fatal(err)
		}
		r := runIn(t, dir, "install", tc.args...)
		wantSuccess(t, r)
		installWant(t, filepath.Join(target, "file"), 0333)
	}
	if _, err := os.Stat(filepath.Join(dir, "file")); err != nil {
		t.Errorf("the source was removed: %v", err)
	}
}

// uutils: test_install_mode_symbolic, test_install_mode_comma_separated,
// test_install_mode_comma_separated_directory.
func TestInstall_ModeSymbolic(t *testing.T) {
	tests := []struct {
		mode  string
		args  []string
		want  uint32
		isDir bool
	}{
		{mode: "o+wx", want: 0003, args: []string{"file", "dir"}},
		{mode: "ug+rwX,o+rX", want: 0664, args: []string{"file", "dir"}},
		{mode: "ug+rwX,o+rX", want: 0775, args: []string{"-d", "newdir"}, isDir: true},
	}
	for _, tc := range tests {
		t.Run(tc.mode+strings.Join(tc.args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			mkFile(t, dir, "file", "")
			if err := os.Mkdir(filepath.Join(dir, "dir"), 0755); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"-m", tc.mode}, tc.args...)
			r := runIn(t, dir, "install", args...)
			wantSuccess(t, r)
			path := filepath.Join(dir, "newdir")
			if !tc.isDir {
				path = filepath.Join(dir, "dir/file")
			}
			installWant(t, path, tc.want)
		})
	}
}

// uutils: test_install_target_file, test_install_target_new_file.
func TestInstall_TargetForms(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "payload")
	if err := os.Mkdir(filepath.Join(dir, "target_dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if r := runIn(t, dir, "install", "file", "target_file"); r.code != 0 {
		t.Fatalf("install file target_file: %s", r.stderr)
	}
	if got := readFile(t, filepath.Join(dir, "target_file")); got != "payload" {
		t.Errorf("target_file = %q", got)
	}
	if r := runIn(t, dir, "install", "file", "target_dir/file"); r.code != 0 {
		t.Fatalf("install file target_dir/file: %s", r.stderr)
	}
	if got := readFile(t, filepath.Join(dir, "target_dir/file")); got != "payload" {
		t.Errorf("target_dir/file = %q", got)
	}
}

// uutils: test_install_preserve_timestamps.
func TestInstall_PreserveTimestamps(t *testing.T) {
	dir := t.TempDir()
	src := mkFile(t, dir, "source_file", "")
	stamp := time.Unix(1000000, 0)
	if err := os.Chtimes(src, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, "install", "-p", "source_file", "target_file")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "target_file"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(stamp) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), stamp)
	}
}

// uutils: test_install_dir, test_t_exist_dir and test_install_target_dir_not_a_directory.
func TestInstall_TargetDirectoryFlagFailures(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "new")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	mkFile(t, dir, "sub/exists", "old")

	r := runIn(t, dir, "install", "-t", "sub/exists", "-D", "file")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "failed to access 'sub/exists': Not a directory") {
		t.Errorf("stderr = %q", r.stderr)
	}

	r = runIn(t, dir, "install", "-t", "absent", "file")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "failed to access 'absent'") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

// uutils: test_install_unusable_target_diagnostics, minus the messages that
// name a status this implementation words differently.
func TestInstall_UnusableTargets(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "source", "new")
	mkFile(t, dir, "other", "other source")
	mkFile(t, dir, "regular", "old")

	r := runIn(t, dir, "install", "source", "other", "regular")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "target 'regular': not a directory") {
		t.Errorf("stderr = %q", r.stderr)
	}
	r = runIn(t, dir, "install", "source", "missing/")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "cannot create regular file 'missing/'") {
		t.Errorf("stderr = %q", r.stderr)
	}
	if readFile(t, filepath.Join(dir, "regular")) != "old" {
		t.Error("regular was modified")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing was created")
	}
}

// uutils: test_install_no_target_directory_*.
func TestInstall_NoTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "")
	if err := os.Mkdir(filepath.Join(dir, "dir1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dir2"), 0755); err != nil {
		t.Fatal(err)
	}

	r := runIn(t, dir, "install", "-T", "dir1", "dir2")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "omitting directory 'dir1'") {
		t.Errorf("stderr = %q", r.stderr)
	}
	r = runIn(t, dir, "install", "-T", "-t", "dir1", "file")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "mutually exclusive") {
		t.Errorf("stderr = %q", r.stderr)
	}
	if r := runIn(t, dir, "install", "-T", "file", "dest"); r.code != 0 {
		t.Fatalf("install -T file dest: %s", r.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "dest")); err != nil {
		t.Errorf("dest was not created: %v", err)
	}
}

// uutils: test_install_missing_arguments, test_install_missing_destination.
func TestInstall_UsageErrors(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "onlyone", "")
	for _, args := range [][]string{{}, {"onlyone"}} {
		r := runIn(t, dir, "install", args...)
		wantFail(t, r)
		if !strings.Contains(r.stderr, "missing file operand") &&
			!strings.Contains(r.stderr, "missing destination file operand after 'onlyone'") {
			t.Errorf("args %v: stderr = %q", args, r.stderr)
		}
	}
}

// A directory operand is never copied: install does not recurse.
func TestInstall_OmittingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dst"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, "install", "src", "dst")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "omitting directory 'src'") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

// A destination directory holding a subdirectory of the same name as the source
// is refused instead of being replaced.
func TestInstall_CannotOverwriteSubdirectory(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "")
	if err := os.MkdirAll(filepath.Join(dir, "dir1/file"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, "install", "file", "dir1")
	wantFail(t, r)
	if !strings.Contains(r.stderr, "cannot overwrite directory") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
