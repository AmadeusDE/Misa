//go:build linux

// Linux-only checks for the parts of dd's conversion and copy behaviour that
// depend on kernel interfaces: SEEK_DATA hole detection for conv=sparse and
// the aligned reads iflag=direct needs.
package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// ddDataAt is the offset of the first real data at or after off, which is how
// a sparse file shows the holes conv=sparse left behind: SEEK_DATA skips them.
func ddDataAt(t *testing.T, file *os.File, off int64) int64 {
	t.Helper()
	pos, err := syscall.Seek(int(file.Fd()), off, 3) // SEEK_DATA
	if err != nil {
		return -1
	}
	return pos
}

func TestDDConvSparseCreatesHoles(t *testing.T) {
	dir := t.TempDir()
	// An all-zero input becomes a file with no allocated blocks at all: the
	// kernel reports no data anywhere in it.
	input := make([]byte, 32*512)
	if err := os.WriteFile(filepath.Join(dir, "in"), input, 0o600); err != nil {
		t.Fatal(err)
	}
	r := ddRun(t, dir, nil, "if=in", "of=out", "bs=512", "conv=sparse", "status=none")
	assertExitCode(t, r, 0)

	out, err := os.Open(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if got := ddDataAt(t, out, 0); got != -1 {
		t.Errorf("first data offset = %d, want no data at all", got)
	}
	info, err := out.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(input)) {
		t.Errorf("size = %d, want %d", info.Size(), len(input))
	}
	got, err := os.ReadFile(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(input) {
		t.Error("conv=sparse did not keep the data identical")
	}
}

func TestDDConvSparseDataStaysPut(t *testing.T) {
	dir := t.TempDir()
	input := make([]byte, 32*512)
	copy(input[len(input)-4:], "tail")
	if err := os.WriteFile(filepath.Join(dir, "in"), input, 0o600); err != nil {
		t.Fatal(err)
	}
	r := ddRun(t, dir, nil, "if=in", "of=out", "bs=512", "conv=sparse", "status=none")
	assertExitCode(t, r, 0)

	out, err := os.Open(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	// Everything but the last filesystem block is a hole, so the copy is not
	// written out block by block.
	first := ddDataAt(t, out, 0)
	if first <= 0 {
		t.Errorf("first data offset = %d, want a hole at the start", first)
	}
	if first > int64(len(input))-512 {
		t.Errorf("first data offset = %d, want it inside the last 512 bytes", first)
	}
	got, err := os.ReadFile(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(input) {
		t.Error("conv=sparse did not keep the data identical")
	}
}

func TestDDIfDirectReadsAligned(t *testing.T) {
	// uutils test_iflag_direct_read_uses_aligned_buffer: the kernel refuses a
	// misaligned O_DIRECT read with EINVAL, so the copy has to be aligned.
	dir := t.TempDir()
	data := make([]byte, 2*4096)
	for i := range data {
		data[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(filepath.Join(dir, "in"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	r := ddRun(t, dir, nil, "if=in", "of=out", "iflag=direct", "bs=4096", "status=none")
	if strings.Contains(r.stderr, "unsupported") {
		t.Skip("this kernel or filesystem has no O_DIRECT")
	}
	assertExitCode(t, r, 0)
	got, err := os.ReadFile(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("iflag=direct copied %d bytes, want %d", len(got), len(data))
	}
}
