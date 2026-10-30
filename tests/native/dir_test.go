package native

import "testing"

// dir and vdir are ls output-mode aliases (dir = ls -C -b, vdir = ls -l -b).
func TestDir_ColumnsByDefault(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "")
	mkFile(t, dir, "b.txt", "")
	r := runIn(t, dir, "dir")
	wantSuccess(t, r)
	wantOut(t, r, "a.txt  b.txt\n")
}

func TestDir_LongFlag(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "")
	r := runIn(t, dir, "dir", "-l")
	wantSuccess(t, r)
	wantOutContains(t, r, "-rw-")
}

func TestDir_InvalidOption(t *testing.T) {
	r := run(t, "dir", "-/")
	wantCode(t, r, 2)
	wantErrContains(t, r, "dir: unknown option")
}

func TestVdir_LongByDefault(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "")
	r := runIn(t, dir, "vdir")
	wantSuccess(t, r)
	wantOutContains(t, r, "total ")
	wantOutContains(t, r, "-rw-")
}

func TestVdir_ColumnFlag(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "")
	r := runIn(t, dir, "vdir", "-C")
	wantSuccess(t, r)
	wantOutContains(t, r, "a.txt")
	if r.stdout != "" && r.stdout[0] == '-' {
		t.Errorf("vdir -C printed a long listing: %q", r.stdout)
	}
}

func TestDirVdir_EscapeNames(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a b", "")
	for _, name := range []string{"dir", "vdir"} {
		r := runIn(t, dir, name)
		wantSuccess(t, r)
		wantOutContains(t, r, `a\ b`)
	}
}
