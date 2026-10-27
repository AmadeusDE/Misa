package native

import (
	"testing"
)

func TestSha1sum_Stdin(t *testing.T) {
	r := runStdin(t, "sha1sum", "")
	wantSuccess(t, r)
	wantOutContains(t, r, "da39a3ee5e6b4b0d3255bfef95601890afd80709  -")
}

func TestSha1sum_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "hello\n")
	r := runIn(t, dir, "sha1sum", "input.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "f572d396fae9206628714fb2ce00f72e94f2258f  input.txt")
}

func TestSha1sum_Check(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "hello\n")
	mkFile(t, dir, "check.sha1", "f572d396fae9206628714fb2ce00f72e94f2258f  file.txt\n")
	r := runIn(t, dir, "sha1sum", "-c", "check.sha1")
	wantSuccess(t, r)
	wantOutContains(t, r, "file.txt: OK")
}

func TestSha1sum_MissingFile(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "sha1sum", "nonexistent.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "no such file or directory")
}
