package native

import (
	"testing"
)

func TestSha512sum_Stdin(t *testing.T) {
	r := runStdin(t, "sha512sum", "")
	wantSuccess(t, r)
	wantOutContains(t, r, "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e  -")
}

func TestSha512sum_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input.txt", "hello\n")
	r := runIn(t, dir, "sha512sum", "input.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629  input.txt")
}

func TestSha512sum_Check(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "hello\n")
	mkFile(t, dir, "check.sha512", "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629  file.txt\n")
	r := runIn(t, dir, "sha512sum", "-c", "check.sha512")
	wantSuccess(t, r)
	wantOutContains(t, r, "file.txt: OK")
}

func TestSha512sum_MissingFile(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "sha512sum", "nonexistent.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "no such file or directory")
}
