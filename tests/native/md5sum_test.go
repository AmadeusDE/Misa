package native

import (
	"testing"
)

func TestMd5sum_Stdin(t *testing.T) {
	r := runStdin(t, "md5sum", "")
	wantSuccess(t, r)
	wantOutContains(t, r, "d41d8cd98f00b204e9800998ecf8427e  -")
}

func TestMd5sum_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "hello\n")
	r := runIn(t, dir, "md5sum", "input.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "b1946ac92492d2347c6235b4d2611184  input.txt")
}

func TestMd5sum_Check(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "hello\n")
	mkFile(t, dir, "check.md5", "b1946ac92492d2347c6235b4d2611184  file.txt\n")
	r := runIn(t, dir, "md5sum", "-c", "check.md5")
	wantSuccess(t, r)
	wantOutContains(t, r, "file.txt: OK")
}

func TestMd5sum_MissingFile(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "md5sum", "nonexistent.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "no such file or directory")
}
