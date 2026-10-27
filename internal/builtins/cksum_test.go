package builtins

import (
	"encoding/hex"
	"testing"
)

func TestCksumFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.txt", "hello\n")
	r := runBuiltinIn(t, dir, "cksum", "hello.txt")
	assertExitCode(t, r, 0)
	// POSIX cksum of "hello\n" (6 bytes)
	assertStdoutContains(t, r, "6")
	assertStdoutContains(t, r, "hello.txt")
}

func TestCksumStdin(t *testing.T) {
	r := runBuiltinStdin(t, "cksum", "hello\n")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "6")
}

func TestCksumMultiple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")
	r := runBuiltinIn(t, dir, "cksum", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "a.txt")
	assertStdoutContains(t, r, "b.txt")
}

func TestCksumEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "empty.txt", "")
	r := runBuiltinIn(t, dir, "cksum", "empty.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "0")
	assertStdoutContains(t, r, "empty.txt")
}

func TestCksumMissing(t *testing.T) {
	r := runBuiltin(t, "cksum", "nonexistent.txt")
	assertExitCode(t, r, 1)
}

func TestCksumKnownValue(t *testing.T) {
	// POSIX cksum of empty string is 4294967295
	dir := t.TempDir()
	writeFile(t, dir, "empty.txt", "")
	r := runBuiltinIn(t, dir, "cksum", "empty.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "4294967295 0 empty.txt\n")
}

func TestBlake2bEmpty(t *testing.T) {
	got := hex.EncodeToString(blake2bHash([]byte{}, 64))
	want := "786a02f742015903c6c6fd852552d272912f4740e15847618a86e217f71f5419d25e1031afee585313896444934eb04b903a685b1448b755d56f701afe9be2ce"
	if got != want {
		t.Errorf("blake2b empty mismatch:\ngot  %s\nwant %s", got, want)
	}
}

func TestBlake2bHello(t *testing.T) {
	got := hex.EncodeToString(blake2bHash([]byte("hello\n"), 64))
	want := "f60ce482e5cc1229f39d71313171a8d9f4ca3a87d066bf4b205effb528192a75f14f3271e2c1a90e1de53f275b4d4793eef2f5e31ea90d2ce29d2e481c36435f"
	if got != want {
		t.Errorf("blake2b hello mismatch:\ngot  %s\nwant %s", got, want)
	}
}
