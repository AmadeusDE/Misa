package native

import (
	"testing"
)

func TestBase32_EncodeStdin(t *testing.T) {
	r := runStdin(t, "base32", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "NBSWY3DP\n")
}

func TestBase32_EncodeFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in.txt", "foobar")
	r := runIn(t, dir, "base32", "in.txt")
	wantSuccess(t, r)
	wantOut(t, r, "MZXW6YTBOI======\n")
}

func TestBase32_DecodeStdin(t *testing.T) {
	r := runStdin(t, "base32", "NBSWY3DP\n", "-d")
	wantSuccess(t, r)
	wantOut(t, r, "hello")
}

func TestBase32_DecodeFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "encoded.txt", "MZXW6YTBOI======\n")
	r := runIn(t, dir, "base32", "-d", "encoded.txt")
	wantSuccess(t, r)
	wantOut(t, r, "foobar")
}

func TestBase32_DecodeInvalid(t *testing.T) {
	r := runStdin(t, "base32", "!!!invalid!!!", "-d")
	wantFail(t, r)
}

func TestBase32_Wrap(t *testing.T) {
	r := runStdin(t, "base32", "abcdefghijklmnopqrstuvwxyz", "-w", "10")
	wantSuccess(t, r)
	wantOutContains(t, r, "\n")
}
