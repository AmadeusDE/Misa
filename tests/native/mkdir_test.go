package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMkdir_Basic(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "newdir")
	wantSuccess(t, r)
	info, err := os.Stat(dir + "/newdir")
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestMkdir_ParentsVerbosePreservesDotDot(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "-p", "-v", "one/../two/../three")
	wantSuccess(t, r)
	wantOutContains(t, r, "created directory 'one/../two/../three'")
}

func TestMkdir_Parents(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "-p", "a/b/c")
	wantSuccess(t, r)
	info, err := os.Stat(dir + "/a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestMkdir_ParentsExisting(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(dir+"/existing", 0755)
	r := runIn(t, dir, "mkdir", "-p", "existing")
	wantSuccess(t, r)
}

func TestMkdir_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(dir+"/existing", 0755)
	r := runIn(t, dir, "mkdir", "existing")
	wantFail(t, r)
}

func TestMkdir_Multiple(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "a", "b", "c")
	wantSuccess(t, r)
	for _, name := range []string{"a", "b", "c"} {
		info, err := os.Stat(dir + "/" + name)
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
		} else if !info.IsDir() {
			t.Errorf("%s is not a directory", name)
		}
	}
}

func TestMkdir_Mode(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "-m", "700", "restricted")
	wantSuccess(t, r)
	info, err := os.Stat(dir + "/restricted")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("mode = %o, want 700", info.Mode().Perm())
	}
}

func TestMkdir_VerboseAndParentMode(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "-vp", "-m", "777", "parent/child")
	wantSuccess(t, r)
	wantOutContains(t, r, "mkdir: created directory")
	info, err := os.Stat(dir + "/parent/child")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0777 {
		t.Errorf("child mode = %o, want 777", info.Mode().Perm())
	}
}

func TestMkdir_TrailingDot(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mkdir", "-pv", "sub/.")
	wantSuccess(t, r)
	wantOutContains(t, r, "created directory 'sub'")
}

func TestMkdir_ParentsModeWithoutSearchBit(t *testing.T) {
	dir := t.TempDir()
	r := runInWithUmask(t, dir, 0, "mkdir", "-p", "-m", "653", "one/two")
	wantSuccess(t, r)
	for path, want := range map[string]os.FileMode{"one": 0777, "one/two": 0653} {
		info, err := os.Stat(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", path, got, want)
		}
	}
}

func TestMkdir_ParentsLeavesWorkingDirectoryAlone(t *testing.T) {
	before, _ := os.Getwd()
	dir := t.TempDir()
	wantSuccess(t, runIn(t, dir, "mkdir", "-p", "a/b/../c"))
	if after, _ := os.Getwd(); after != before {
		t.Errorf("process cwd changed from %s to %s", before, after)
	}
	if _, err := os.Stat(filepath.Join(dir, "a/c")); err != nil {
		t.Error(err)
	}
}
