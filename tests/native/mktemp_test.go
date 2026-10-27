package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMktemp_File(t *testing.T) {
	r := run(t, "mktemp")
	wantSuccess(t, r)
	path := strings.TrimSpace(r.stdout)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("temp file %q should exist", path)
	}
	os.Remove(path)
}

func TestMktemp_Dir(t *testing.T) {
	r := run(t, "mktemp", "-d")
	wantSuccess(t, r)
	path := strings.TrimSpace(r.stdout)
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("temp dir %q should exist", path)
	} else if !info.IsDir() {
		t.Errorf("expected directory")
	}
	os.Remove(path)
}

func TestMktemp_Template(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mktemp", "test.XXXXXX")
	wantSuccess(t, r)
	path := strings.TrimSpace(r.stdout)
	if len(path) == 0 {
		t.Error("expected output path")
	}
	os.Remove(path)
}

func TestMktemp_DryRun(t *testing.T) {
	r := run(t, "mktemp", "-u")
	wantSuccess(t, r)
	path := strings.TrimSpace(r.stdout)
	if _, err := os.Stat(path); err == nil {
		t.Errorf("with -u, file should not be created")
		os.Remove(path)
	}
}

func TestMktemp_Suffix(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mktemp", "--suffix=.txt", "tmp.XXXXXX")
	wantSuccess(t, r)
	path := strings.TrimSpace(r.stdout)
	if !strings.HasSuffix(path, ".txt") {
		t.Errorf("expected suffix .txt, got %q", path)
	}
	os.Remove(path)
}

func TestMktemp_SuffixMustEndInX(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mktemp", "--suffix=.txt", "tmp.XXXXXX.txt")
	wantFail(t, r)
	wantErrContains(t, r, "must end in X")
}

func TestMktemp_TooFewXs(t *testing.T) {
	r := run(t, "mktemp", "aXX")
	wantFail(t, r)
	wantErrContains(t, r, "too few X's in template 'aXX'")
}

func TestMktemp_TmpdirRejectsAbsoluteTemplate(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mktemp", "--tmpdir=a", "/XXX")
	wantFail(t, r)
	wantErrContains(t, r, "with --tmpdir, it may not be absolute")
}

func TestMktemp_TmpdirFlag(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mktemp", "-p", ".", "-t", "foo.XXXXXX")
	wantSuccess(t, r)
	path := strings.TrimSpace(r.stdout)
	if len(path) == 0 {
		t.Error("expected output path")
	}
	os.Remove(path)
}

func TestMktemp_Quiet(t *testing.T) {
	dir := t.TempDir()
	nonexistent := filepath.Join(dir, "no_such_dir")
	r := runIn(t, dir, "mktemp", "-q", "-p", nonexistent)
	wantFail(t, r)
	if strings.TrimSpace(r.stderr) != "" {
		t.Errorf("expected empty stderr with -q, got %q", r.stderr)
	}
}

func TestMktemp_NonexistentDir(t *testing.T) {
	dir := t.TempDir()
	nonexistent := filepath.Join(dir, "no_such_dir")
	r := runIn(t, dir, "mktemp", "-p", nonexistent)
	wantFail(t, r)
	wantErrContains(t, r, "No such file or directory")
}
