package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPr_OmitHeader(t *testing.T) {
	r := runStdin(t, "pr", "a\nb\n", "-t")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\n")
}

func TestPr_Columns(t *testing.T) {
	r := runStdin(t, "pr", "a\nb\nc\n", "-t", "-2", "-w", "20")
	wantSuccess(t, r)
	wantOut(t, r, "a        \tc        \nb        \n")
}

func TestPr_PageLayout(t *testing.T) {
	r := runStdin(t, "pr", "a\n", "-D", "DATE")
	wantSuccess(t, r)
	lines := strings.Split(r.stdout, "\n")
	if len(lines) < 6 {
		t.Fatalf("short output: %q", r.stdout)
	}
	if lines[2] != "DATE"+strings.Repeat(" ", 62)+"Page 1" {
		t.Errorf("header = %q", lines[2])
	}
	if got := strings.Count(r.stdout, "\n"); got != 66 {
		t.Errorf("newline count = %d, want 66", got)
	}
}

func TestPr_Merge(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "g"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runIn(t, dir, "pr", "-t", "-l", "11", "-m", "f", "g")
	wantSuccess(t, r)
	wantOutContains(t, r, "a"+strings.Repeat(" ", 34)+"\t"+"b")
}

func TestPr_MissingFile(t *testing.T) {
	r := run(t, "pr", "nonexistent")
	wantFail(t, r)
	wantErrContains(t, r, "pr: nonexistent: ")
}

func TestPr_HeaderDateFromMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Skipf("cannot set mtime: %v", err)
	}
	r := runIn(t, dir, "pr", "f")
	wantSuccess(t, r)
	wantOutContains(t, r, when.Local().Format("2006-01-02 15:04"))
}
