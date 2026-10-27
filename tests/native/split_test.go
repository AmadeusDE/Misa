package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplit_DefaultLines(t *testing.T) {
	dir := t.TempDir()
	var lines string
	for i := 1; i <= 2500; i++ {
		lines += "line\n"
	}
	mkFile(t, dir, "input.txt", lines)

	r := runIn(t, dir, "split", "input.txt")
	wantSuccess(t, r)

	if _, err := os.Stat(filepath.Join(dir, "xaa")); err != nil {
		t.Error("xaa missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "xab")); err != nil {
		t.Error("xab missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "xac")); err != nil {
		t.Error("xac missing")
	}
}

func TestSplit_CombinedShortOptions(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input", "abc")
	r := runIn(t, dir, "split", "-dxen", "4", "input", "x")
	wantSuccess(t, r)
	wantOut(t, r, "")
	if got := readFile(t, filepath.Join(dir, "x00")); got != "a" {
		t.Errorf("x00 = %q, want a", got)
	}
}

func TestSplit_NumericSuffixes(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "1\n2\n3\n4\n5\n")

	r := runIn(t, dir, "split", "-d", "-l", "2", "in.txt", "out.")
	wantSuccess(t, r)

	if _, err := os.Stat(filepath.Join(dir, "out.00")); err != nil {
		t.Error("out.00 missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "out.01")); err != nil {
		t.Error("out.01 missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "out.02")); err != nil {
		t.Error("out.02 missing")
	}
}

func TestSplit_NumberChunkStdout(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "1\n2\n3\n4\n5\n6\n")

	r := runIn(t, dir, "split", "-n", "1/2", "in.txt")
	wantSuccess(t, r)
	if r.stdout != "1\n2\n3\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "1\n2\n3\n")
	}
}

func TestSplit_MissingFileStderr(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "split", "non-existing")
	wantCode(t, r, 1)
	wantErrContains(t, r, "cannot open 'non-existing' for reading")
}

func TestSplit_LineBytesOverflow(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "a\nb\nc\n")
	r := runIn(t, dir, "split", "-C", "18446744073709551616", "in.txt")
	wantSuccess(t, r)
	data, err := os.ReadFile(filepath.Join(dir, "xaa"))
	if err != nil || string(data) != "a\nb\nc\n" {
		t.Errorf("xaa content = %q, err = %v", string(data), err)
	}
}

func TestSplit_SuffixExhaustion(t *testing.T) {
	dir := t.TempDir()
	var buf []byte
	for i := 0; i < 91; i++ {
		buf = append(buf, 'x')
	}
	mkFile(t, dir, "in.txt", string(buf))
	r := runIn(t, dir, "split", "-b", "1", "--numeric-suffixes=89", "-a", "2", "in.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "output file suffixes exhausted")
}

func TestSplit_LineChunksKth(t *testing.T) {
	dir := t.TempDir()
	var lines string
	for i := 0; i < 100; i++ {
		lines += "line\n"
	}
	mkFile(t, dir, "in.txt", lines)
	r := runIn(t, dir, "split", "-n", "l/3/10", "in.txt")
	wantSuccess(t, r)
	if r.stdout == "" {
		t.Error("expected non-empty stdout for 3rd chunk of 10")
	}
}

func TestSplit_CustomSeparator(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "a;b;c;d;e;")
	r := runIn(t, dir, "split", "-t", ";", "-l", "2", "in.txt")
	wantSuccess(t, r)
	data, err := os.ReadFile(filepath.Join(dir, "xaa"))
	if err != nil || string(data) != "a;b;" {
		t.Errorf("xaa content = %q, want %q, err = %v", string(data), "a;b;", err)
	}
}

func TestSplit_HexSuffixes(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "1\n2\n3\n")
	r := runIn(t, dir, "split", "-x", "-a", "3", "-l", "1", "in.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(filepath.Join(dir, "x000")); err != nil {
		t.Error("x000 missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "x001")); err != nil {
		t.Error("x001 missing")
	}
}
