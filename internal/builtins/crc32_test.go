// Behaviour adapted from toybox toys/posix/cksum.c (crc32 alias, 0BSD,
// commit 6a0c0ef). The expected checksums for "123456789", "hello", "abc" and
// the empty input were produced by a toybox binary built from that commit.
package builtins

import (
	"bytes"
	"hash/crc32"
	"math/rand"
	"testing"
)

// crc32Vectors are the standard IEEE CRC-32 values, which is the variant
// toybox's crc32 computes (reflected table, pre- and post-inversion).
var crc32Vectors = []struct {
	in   string
	want string
}{
	{"", "00000000"},
	{"123456789", "cbf43926"},
	{"hello", "3610a686"},
	{"abc", "352441c2"},
	{"a", "e8b7be43"},
}

func TestCrc32Stdin(t *testing.T) {
	for _, tc := range crc32Vectors {
		r := runBuiltinStdin(t, "crc32", tc.in)
		assertExitCode(t, r, 0)
		assertStdout(t, r, tc.want+"\n")
	}
}

func TestCrc32MatchesStdlib(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, size := range []int{0, 1, 3, 7, 64, 1023, 65536 + 5} {
		data := make([]byte, size)
		rng.Read(data)
		r := runBuiltinStdin(t, "crc32", string(data))
		assertExitCode(t, r, 0)
		want := crc32SumHex(data)
		if r.stdout != want+"\n" {
			t.Errorf("size %d: stdout = %q, want %q", size, r.stdout, want+"\n")
		}
	}
}

func crc32SumHex(data []byte) string {
	const hexdigits = "0123456789abcdef"
	sum := crc32.ChecksumIEEE(data)
	out := make([]byte, 8)
	for i := 0; i < 8; i++ {
		out[7-i] = hexdigits[sum>>(4*uint(i))&0xf]
	}
	return string(out)
}

func TestCrc32Files(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one.txt", "hello")
	writeFile(t, dir, "two.txt", "abc")

	t.Run("single file", func(t *testing.T) {
		r := runBuiltinIn(t, dir, "crc32", "one.txt")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "3610a686 one.txt\n")
	})

	t.Run("multiple files", func(t *testing.T) {
		r := runBuiltinIn(t, dir, "crc32", "one.txt", "two.txt")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "3610a686 one.txt\n352441c2 two.txt\n")
	})

	t.Run("dash is stdin", func(t *testing.T) {
		r := runBuiltinStdinIn(t, dir, "crc32", "hello", "-")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "3610a686\n")
	})

	t.Run("missing file still hashes the rest", func(t *testing.T) {
		r := runBuiltinIn(t, dir, "crc32", "missing.txt", "two.txt")
		assertExitCode(t, r, 1)
		assertStdout(t, r, "352441c2 two.txt\n")
		assertStderrContains(t, r, "crc32: missing.txt: No such file or directory")
	})

	t.Run("empty file", func(t *testing.T) {
		writeFile(t, dir, "empty.txt", "")
		r := runBuiltinIn(t, dir, "crc32", "empty.txt")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "00000000 empty.txt\n")
	})
}

func TestCrc32UnsupportedOptions(t *testing.T) {
	// cksum's -H/-I/-P/-L/-N are cksum options; toybox's crc32 takes none and
	// would try to open "-H" as a file. Misa rejects them honestly instead.
	for _, arg := range []string{"-H", "-N", "-PI", "--hex"} {
		r := runBuiltinStdin(t, "crc32", "x", arg)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "unknown option")
	}
}

func TestCrc32Help(t *testing.T) {
	r := runBuiltin(t, "crc32", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage:")
}

func TestCrc32NoStdin(t *testing.T) {
	cmd, _ := Lookup("crc32")
	var stdout, stderr bytes.Buffer
	code := cmd.Run(&Context{Dir: t.TempDir(), Stdout: &stdout, Stderr: &stderr})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	assertStderrContains(t, runResult{stderr: stderr.String()}, "stdin not available")
}
