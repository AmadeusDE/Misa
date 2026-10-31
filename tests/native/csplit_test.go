package native

import (
	"os"
	"path/filepath"
	"testing"
)

func csplitReadAll(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	out := make([]string, len(names))
	for i, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out[i] = string(data)
	}
	return out
}

func TestCsplit_LineNumber(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in", "1\n2\n3\n4\n")
	r := runIn(t, dir, "csplit", "in", "3")
	wantSuccess(t, r)
	wantOut(t, r, "4\n4\n")
	got := csplitReadAll(t, dir, "xx00", "xx01")
	if got[0] != "1\n2\n" || got[1] != "3\n4\n" {
		t.Errorf("splits = %q, want [\"1\\n2\\n\" \"3\\n4\\n\"]", got)
	}
}

func TestCsplit_Stdin(t *testing.T) {
	r := runStdin(t, "csplit", "a\nb\nc\n", "-", "/b/")
	wantSuccess(t, r)
	wantOut(t, r, "2\n4\n")
}

func TestCsplit_Repeat(t *testing.T) {
	r := runStdin(t, "csplit", "a\nb\nc\nd\n", "-", "2", "{1}")
	wantSuccess(t, r)
	wantOut(t, r, "2\n4\n2\n")
}

func TestCsplit_PrefixAndFormat(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in", "1\n2\n3\n")
	r := runIn(t, dir, "csplit", "-f", "part-", "-b", "%02d.txt", "in", "2")
	wantSuccess(t, r)
	got := csplitReadAll(t, dir, "part-00.txt", "part-01.txt")
	if got[0] != "1\n" || got[1] != "2\n3\n" {
		t.Errorf("splits = %q", got)
	}
}

func TestCsplit_MatchNotFound(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in", "1\n2\n")
	r := runIn(t, dir, "csplit", "in", "/nope/")
	wantFail(t, r)
	wantErr(t, r, "csplit: '/nope/': match not found\n")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "in" {
		t.Errorf("created files were not cleaned up: %v", entries)
	}
}

func TestCsplit_KeepFilesOnError(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in", "1\n2\n")
	r := runIn(t, dir, "csplit", "-k", "in", "/nope/")
	wantFail(t, r)
	csplitReadAll(t, dir, "xx00")
}

func TestCsplit_InvalidSuffixFormat(t *testing.T) {
	r := run(t, "csplit", "-b", "%c", "in", "1")
	wantFail(t, r)
	wantErr(t, r, "csplit: incorrect conversion specification in suffix\n")
}

func TestCsplit_HelpAndVersion(t *testing.T) {
	r := run(t, "csplit", "--help")
	wantSuccess(t, r)
	wantOutContains(t, r, "--suppress-matched")
	r = run(t, "csplit", "--version")
	wantSuccess(t, r)
}
