package native

import (
	"strings"
	"testing"
)

func TestCksum_Basic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello\n")
	r := runIn(t, dir, "cksum", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "f.txt")
	parts := strings.Fields(r.stdout)
	if len(parts) < 3 {
		t.Errorf("expected at least 3 fields, got %d: %q", len(parts), r.stdout)
	}
}

func TestCksum_Stdin(t *testing.T) {
	r := runStdin(t, "cksum", "hello\n")
	wantSuccess(t, r)
	parts := strings.Fields(r.stdout)
	if len(parts) < 2 {
		t.Errorf("expected at least 2 fields, got %d: %q", len(parts), r.stdout)
	}
}

func TestCksum_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "aaa\n")
	mkFile(t, dir, "b.txt", "bbb\n")
	r := runIn(t, dir, "cksum", "a.txt", "b.txt")
	wantSuccess(t, r)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestCksum_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "empty.txt", "")
	r := runIn(t, dir, "cksum", "empty.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "0")
}

func TestCksum_Algorithms(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello\n")

	// MD5 tagged
	r := runIn(t, dir, "cksum", "-a", "md5", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "MD5 (f.txt) = b1946ac92492d2347c6235b4d2611184")

	// SHA256 untagged
	r = runIn(t, dir, "cksum", "-a", "sha256", "--untagged", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  f.txt")

	// BLAKE2b default length
	r = runIn(t, dir, "cksum", "-a", "blake2b", "--length=0", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "BLAKE2b (f.txt) = f60ce482e5cc1229f39d71313171a8d9f4ca3a87d066bf4b205effb528192a75f14f3271e2c1a90e1de53f275b4d4793eef2f5e31ea90d2ce29d2e481c36435f")
}

func TestCksum_BinaryAndZero(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello\n")

	r := runIn(t, dir, "cksum", "-a", "sha256", "--untagged", "-b", "-z", "f.txt")
	wantSuccess(t, r)
	if !strings.HasPrefix(r.stdout, "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03 *f.txt\x00") {
		t.Errorf("unexpected binary zero format: %q", r.stdout)
	}
}

func TestCksum_CheckMode(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello\n")
	mkFile(t, dir, "checksums.txt", "MD5 (f.txt) = b1946ac92492d2347c6235b4d2611184\n")

	r := runIn(t, dir, "cksum", "-c", "checksums.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "f.txt: OK")
}

func TestCksum_Sha2MissingLength(t *testing.T) {
	r := runStdin(t, "cksum", "hello\n", "-a", "sha2")
	wantCode(t, r, 1)
	wantErrContains(t, r, "--algorithm=sha2 requires specifying --length")
}

func TestCksum_Blake2bInvalidLength(t *testing.T) {
	r := runStdin(t, "cksum", "hello\n", "-a", "blake2b", "--length=01")
	wantCode(t, r, 1)
	wantErrContains(t, r, "invalid length: '01'")

	r2 := runStdin(t, "cksum", "hello\n", "-a", "blake2b", "--length=513")
	wantCode(t, r2, 1)
	wantErrContains(t, r2, "maximum digest length for 'BLAKE2b' is 512 bits")
}
