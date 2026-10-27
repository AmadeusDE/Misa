package native

import (
	"testing"
)

func TestSha256sum_Stdin(t *testing.T) {
	r := runStdin(t, "sha256sum", "")
	wantSuccess(t, r)
	wantOutContains(t, r, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  -")
}

func TestSha256sum_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "hello\n")
	r := runIn(t, dir, "sha256sum", "input.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  input.txt")
}

func TestSha256sum_Check(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "hello\n")
	mkFile(t, dir, "check.sha256", "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  file.txt\n")
	r := runIn(t, dir, "sha256sum", "-c", "check.sha256")
	wantSuccess(t, r)
	wantOutContains(t, r, "file.txt: OK")
}

func TestSha256sum_MissingFile(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "sha256sum", "nonexistent.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "no such file or directory")
}
