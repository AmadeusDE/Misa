package native

// The toybox `file` cases vendored in tests/toybox/file.test reach most of its
// fixtures through its test_line helper, which the static adapter does not
// expand into cases. These tests cover those upstream expectations directly
// against the same pinned fixture bytes (tests/toybox/files, provenance in
// tests/provenance/fixtures.json).

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func fileFixture(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "toybox", "files", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFileToyboxFixtureDescriptions(t *testing.T) {
	// Expected strings are copied from tests/file.test at toybox 6a0c0ef.
	for _, tc := range []struct{ fixture, want string }{
		{"elf/ndk-elf-note-full", "ELF shared object, 64-bit LSB arm64, dynamic (/system/bin/linker64), for Android 24, built by NDK r19b (5304403), BuildID=0c712b8af424d57041b85326f0000fadad38ee0a, not stripped\n"},
		{"elf/ndk-elf-note-short", "ELF shared object, 32-bit LSB arm, EABI5, soft float, dynamic (/system/bin/linker), for Android 28, BuildID=da6a5f4ca8da163b9339326e626d8a3c, stripped\n"},
		{"elf/fdstatic", "ELF executable (fdpic), 32-bit MSB sh, static, stripped\n"},
		{"tar/tar.tar", "POSIX tar archive (GNU)\n"},
		{"tar/tar.7z", "7-zip archive data, version 0.4\n"},
	} {
		r := run(t, "file", "-b", fileFixture(t, tc.fixture))
		wantSuccess(t, r)
		wantOut(t, r, tc.want)
	}
}

func TestFileToyboxFixtureSample(t *testing.T) {
	// file.test greps these rather than comparing whole lines.
	for _, tc := range []struct{ fixture, want string }{
		{"java.class", "Java class file, version 53.0 (Java 1.9)"},
		{"utf8/japan.txt", "UTF-8 text"},
		{"tar/tar.tgz", "gzip compressed data"},
		{"tar/tar.tbz2", "bzip2 compressed data, block size = 900k"},
	} {
		r := run(t, "file", "-b", fileFixture(t, tc.fixture))
		wantSuccess(t, r)
		wantOut(t, r, tc.want+"\n")
	}
}

func TestFileTruncatedELF(t *testing.T) {
	// Truncating a real ELF fixture must not panic: the builtin either describes
	// what it can or reports the truncated table, always with exit status 0 or 1.
	data, err := os.ReadFile(fileFixture(t, "elf/ndk-elf-note-short"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, n := range []int{0, 1, 4, 16, 45, 52, 100, 500, len(data) / 2} {
		if n > len(data) {
			continue
		}
		name := "trunc" + strconv.Itoa(n)
		if err := os.WriteFile(filepath.Join(dir, name), data[:n], 0644); err != nil {
			t.Fatal(err)
		}
		r := runIn(t, dir, "file", "-b", name)
		if r.code != 0 && r.code != 1 {
			t.Fatalf("%s: exit = %d, want 0 or 1 (stdout %q, stderr %q)", name, r.code, r.stdout, r.stderr)
		}
		if !strings.HasSuffix(r.stdout, "\n") {
			t.Fatalf("%s: output does not end in a newline: %q", name, r.stdout)
		}
	}
}
