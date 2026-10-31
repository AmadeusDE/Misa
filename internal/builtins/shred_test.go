// Behaviour adapted from uutils coreutils shred (MIT), tests/by-util/test_shred.rs
// at fee61ff5d4f87e0f912a52e639942b5f7ad58663.

package builtins

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runShredCtx runs the shred builtin with a custom stdout, so a test can hand
// it a real *os.File (or a bytes.Buffer) as standard output.
func runShredCtx(t *testing.T, dir string, stdout io.Writer, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("shred")
	if !ok {
		t.Fatal(`builtin "shred" not registered`)
	}
	var out, stderr bytes.Buffer
	w := stdout
	if w == nil {
		w = &out
	}
	ctx := &Context{
		Dir:    dir,
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: w,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: out.String(), stderr: stderr.String()}
}

func readShredFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func shredFileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

func assertShredStderrEmpty(t *testing.T, r runResult) {
	t.Helper()
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestShredOverwritesFile(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("A", 4096) // already block aligned: the size must not change
	path := writeFile(t, dir, "f", content)

	r := runBuiltinIn(t, dir, "shred", "f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	assertShredStderrEmpty(t, r)

	if got := shredFileSize(t, path); got != 4096 {
		t.Errorf("size = %d, want 4096", got)
	}
	if bytes.Equal(readShredFile(t, path), []byte(content)) {
		t.Error("content was not overwritten")
	}
}

func TestShredNoPasses(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("B", 4096)
	path := writeFile(t, dir, "f", content)

	r := runBuiltinIn(t, dir, "shred", "-n0", "-v", "f")
	assertExitCode(t, r, 0)
	assertShredStderrEmpty(t, r)
	if got := readShredFile(t, path); !bytes.Equal(got, []byte(content)) {
		t.Error("-n0 changed the file")
	}
	if got := shredFileSize(t, path); got != 4096 {
		t.Errorf("size = %d, want 4096", got)
	}
}

func TestShredZeroPass(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f", "hello world\n")

	r := runBuiltinIn(t, dir, "shred", "-n1", "-z", "-x", "-v", "f")
	assertExitCode(t, r, 0)
	assertStderrContains(t, r, "shred: f: pass 1/2 (random)...")
	assertStderrContains(t, r, "shred: f: pass 2/2 (000000)...")

	got := readShredFile(t, path)
	if len(got) != 12 {
		t.Fatalf("size = %d, want 12", len(got))
	}
	if !bytes.Equal(got, make([]byte, 12)) {
		t.Errorf("content = %q, want 12 zero bytes", got)
	}
}

func TestShredExactAndSize(t *testing.T) {
	t.Run("exact keeps the length", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "f", "12345")
		r := runBuiltinIn(t, dir, "shred", "-n1", "-x", "f")
		assertExitCode(t, r, 0)
		if got := shredFileSize(t, path); got != 5 {
			t.Errorf("size = %d, want 5", got)
		}
	})

	t.Run("default rounds up to 4096", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "f", "12345")
		r := runBuiltinIn(t, dir, "shred", "-n1", "f")
		assertExitCode(t, r, 0)
		if got := shredFileSize(t, path); got != 4096 {
			t.Errorf("size = %d, want 4096", got)
		}
	})

	t.Run("size grows the file", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "f", "12345")
		r := runBuiltinIn(t, dir, "shred", "-n1", "-s", "100", "f")
		assertExitCode(t, r, 0)
		if got := shredFileSize(t, path); got != 100 {
			t.Errorf("size = %d, want 100", got)
		}
	})

	t.Run("size accepts hex", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "f", "12345")
		r := runBuiltinIn(t, dir, "shred", "-n1", "-s0x10", "f")
		assertExitCode(t, r, 0)
		if got := shredFileSize(t, path); got != 16 {
			t.Errorf("size = %d, want 16", got)
		}
	})

	t.Run("size accepts a K suffix", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "f", "12345")
		r := runBuiltinIn(t, dir, "shred", "-n1", "-s", "2K", "f")
		assertExitCode(t, r, 0)
		if got := shredFileSize(t, path); got != 2048 {
			t.Errorf("size = %d, want 2048", got)
		}
	})

	t.Run("zero size writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "f", "12345")
		r := runBuiltinIn(t, dir, "shred", "-n1", "-s", "0", "-v", "f")
		assertExitCode(t, r, 0)
		assertShredStderrEmpty(t, r)
		if got := shredFileSize(t, path); got != 5 {
			t.Errorf("size = %d, want 5", got)
		}
	})

	t.Run("empty file gets no passes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "")
		r := runBuiltinIn(t, dir, "shred", "-v", "-u", "f")
		assertExitCode(t, r, 0)
		if strings.Contains(r.stderr, "(random)") {
			t.Errorf("stderr = %q, want no pass lines for an empty file", r.stderr)
		}
		assertStderrContains(t, r, "shred: f: removed")
	})
}

func TestShredRemoveMethods(t *testing.T) {
	cases := map[string][]string{
		"-u":                 {"-u"},
		"-uwipe":             {"-uwipe"},
		"--remove":           {"--remove"},
		"--remove=unlink":    {"--remove=unlink"},
		"--remove=wipe":      {"--remove=wipe"},
		"--remove=wipesync":  {"--remove=wipesync"},
		"-u with -n0":        {"-u", "-n0"},
		"-uwipe with -n0":    {"-uwipe", "-n0"},
		"--remove with -n0":  {"--remove", "-n0"},
		"-u with -z":         {"-u", "-z"},
		"-u after -f":        {"-f", "-u"},
		"-u and --remove":    {"-u", "--remove=wipe"},
		"--remove then -u":   {"--remove=unlink", "-u"},
		"-u with a size":     {"-u", "-s", "8"},
		"-u with iterations": {"-u", "-n", "2"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "f", "content of the file")
			all := append(append([]string{}, args...), "f")
			r := runBuiltinIn(t, dir, "shred", all...)
			assertExitCode(t, r, 0)
			assertShredStderrEmpty(t, r)
			if _, err := os.Lstat(filepath.Join(dir, "f")); !os.IsNotExist(err) {
				t.Errorf("file still exists (err = %v)", err)
			}
			if entries := shredDirEntries(t, dir); len(entries) != 0 {
				t.Errorf("directory still contains %v", entries)
			}
		})
	}
}

func shredDirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestShredRemoveVerbose(t *testing.T) {
	t.Run("unlink", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "data")
		r := runBuiltinIn(t, dir, "shred", "--remove=unlink", "-v", "-x", "f")
		assertExitCode(t, r, 0)
		assertStderrContains(t, r, "shred: f: removing")
		assertStderrContains(t, r, "shred: f: removed")
		if strings.Contains(r.stderr, "renamed to") {
			t.Errorf("stderr = %q, want no rename for unlink", r.stderr)
		}
	})

	t.Run("wipe renames", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "data")
		r := runBuiltinIn(t, dir, "shred", "--remove=wipe", "-v", "-x", "f")
		assertExitCode(t, r, 0)
		assertStderrContains(t, r, "renamed to")
		assertStderrContains(t, r, "shred: f: removed")
		if entries := shredDirEntries(t, dir); len(entries) != 0 {
			t.Errorf("directory still contains %v", entries)
		}
	})

	t.Run("wipesync renames", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "data")
		r := runBuiltinIn(t, dir, "shred", "-u", "-v", "-x", "f")
		assertExitCode(t, r, 0)
		assertStderrContains(t, r, "renamed to")
		if entries := shredDirEntries(t, dir); len(entries) != 0 {
			t.Errorf("directory still contains %v", entries)
		}
	})

	t.Run("wipe skips names that exist", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "0", "keep me")
		writeFile(t, dir, "f", "data")
		r := runBuiltinIn(t, dir, "shred", "-u", "-v", "-x", "f")
		assertExitCode(t, r, 0)
		assertStderrContains(t, r, "shred: f: removed")
		entries := shredDirEntries(t, dir)
		if len(entries) != 1 || entries[0] != "0" {
			t.Errorf("directory contains %v, want just [0]", entries)
		}
		if got := string(readShredFile(t, filepath.Join(dir, "0"))); got != "keep me" {
			t.Errorf("existing file changed: %q", got)
		}
	})
}

func TestShredForce(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	dir := t.TempDir()
	path := writeFile(t, dir, "f", strings.Repeat("C", 64))
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "shred", "-x", "f")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Permission denied")
	if got := string(readShredFile(t, path)); got != strings.Repeat("C", 64) {
		t.Error("the read-only file was modified without -f")
	}

	r = runBuiltinIn(t, dir, "shred", "-x", "-f", "f")
	assertExitCode(t, r, 0)
	if got := string(readShredFile(t, path)); got == strings.Repeat("C", 64) {
		t.Error("the file was not overwritten with -f")
	}
}

func TestShredHardLink(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("D", 4096)
	first := writeFile(t, dir, "one", content)
	second := filepath.Join(dir, "two")
	if err := os.Link(first, second); err != nil {
		t.Skipf("hard links unsupported here: %v", err)
	}

	r := runBuiltinIn(t, dir, "shred", "one")
	assertExitCode(t, r, 0)

	st1, err := os.Stat(first)
	if err != nil {
		t.Fatalf("stat one: %v", err)
	}
	st2, err := os.Stat(second)
	if err != nil {
		t.Fatalf("stat two: %v", err)
	}
	if !os.SameFile(st1, st2) {
		t.Error("the two names no longer share an inode")
	}
	got := readShredFile(t, second)
	if bytes.Equal(got, []byte(content)) {
		t.Error("the shared inode was not overwritten")
	}
	if !bytes.Equal(got, readShredFile(t, first)) {
		t.Error("the two names disagree about the content")
	}

	// The link survived: dropping one name keeps the other one alive.
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Errorf("the other link is gone: %v", err)
	}
}

func TestShredSymlink(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("E", 4096)
	target := writeFile(t, dir, "target", content)
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}

	r := runBuiltinIn(t, dir, "shred", "link")
	assertExitCode(t, r, 0)

	if bytes.Equal(readShredFile(t, target), []byte(content)) {
		t.Error("the link target was not overwritten")
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("the link itself is gone: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a regular file")
	}
	if dest, err := os.Readlink(link); err != nil || dest != "target" {
		t.Errorf("readlink = %q, %v; want \"target\"", dest, err)
	}
}

func TestShredOperandErrors(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
			t.Fatal(err)
		}
		r := runBuiltinIn(t, dir, "shred", "sub")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: sub: Is a directory")
	})

	t.Run("missing", func(t *testing.T) {
		dir := t.TempDir()
		r := runBuiltinIn(t, dir, "shred", "nope")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: nope: No such file or directory")
	})

	t.Run("trailing slash on a file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "data")
		r := runBuiltinIn(t, dir, "shred", "f/")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: f/: Not a directory")
	})

	t.Run("one failure does not stop the others", func(t *testing.T) {
		dir := t.TempDir()
		content := strings.Repeat("F", 4096)
		good := writeFile(t, dir, "good", content)
		r := runBuiltinIn(t, dir, "shred", "nope", "good")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "No such file or directory")
		if bytes.Equal(readShredFile(t, good), []byte(content)) {
			t.Error("the second operand was not shredded")
		}
	})

	t.Run("missing file operand", func(t *testing.T) {
		r := runBuiltinIn(t, t.TempDir(), "shred")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: missing file operand")
		assertStderrContains(t, r, "Try 'shred --help' for more information.")
	})
}

func TestShredStdout(t *testing.T) {
	t.Run("pipe is rejected", func(t *testing.T) {
		dir := t.TempDir()
		r := runBuiltinIn(t, dir, "shred", "-")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: -: standard output is not a regular file")
	})

	t.Run("regular file is shredded in place", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "out", "abcdefgh")
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		r := runShredCtx(t, dir, f, "-n1", "-x", "-")
		assertExitCode(t, r, 0)
		assertShredStderrEmpty(t, r)
		if got := shredFileSize(t, path); got != 8 {
			t.Errorf("size = %d, want 8", got)
		}
		if got := string(readShredFile(t, path)); got == "abcdefgh" {
			t.Error("standard output's file was not overwritten")
		}
	})

	t.Run("remove is refused", func(t *testing.T) {
		dir := t.TempDir()
		path := writeFile(t, dir, "out", "abcdefgh")
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		r := runShredCtx(t, dir, f, "-u", "-")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: -: cannot remove standard output")
		if got := string(readShredFile(t, path)); got != "abcdefgh" {
			t.Error("the file was touched despite the error")
		}

		r = runBuiltinIn(t, dir, "shred", "--remove=wipe", "-")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "cannot remove standard output")
	})
}

func TestShredRandomSource(t *testing.T) {
	t.Run("deterministic bytes", func(t *testing.T) {
		dir := t.TempDir()
		source := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		writeFile(t, dir, "src", source)
		path := writeFile(t, dir, "f", "hello")

		r := runBuiltinIn(t, dir, "shred", "-n1", "-x", "--random-source=src", "f")
		assertExitCode(t, r, 0)
		if got := string(readShredFile(t, path)); got != source[:5] {
			t.Errorf("content = %q, want %q", got, source[:5])
		}
	})

	t.Run("source is read sequentially", func(t *testing.T) {
		dir := t.TempDir()
		source := strings.Repeat("0123456789", 10) // 100 bytes
		writeFile(t, dir, "src", source)
		path := writeFile(t, dir, "f", "hello")

		r := runBuiltinIn(t, dir, "shred", "-n2", "-x", "--random-source=src", "f")
		assertExitCode(t, r, 0)
		if got := string(readShredFile(t, path)); got != source[5:10] {
			t.Errorf("content = %q, want %q", got, source[5:10])
		}
	})

	t.Run("missing source", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "f", "hello")
		r := runBuiltinIn(t, dir, "shred", "--random-source=missing", "f")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "shred: missing: No such file or directory")
	})

	t.Run("exhausted source fails the file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src", "ab") // shorter than one pass
		writeFile(t, dir, "f", "hello world")
		r := runBuiltinIn(t, dir, "shred", "-n1", "-x", "--random-source=src", "f")
		assertExitCode(t, r, 1)
		if r.stderr == "" {
			t.Error("no diagnostic for the short random source")
		}
	})
}

func TestShredInvalidArguments(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"bad size", []string{"-s", "4vv", "f"}, "shred: invalid file size: '4vv'"},
		{"negative size", []string{"-s", "-5", "f"}, "shred: invalid file size: '-5'"},
		{"negative passes", []string{"-n", "-1", "f"}, "shred: invalid number of passes: '-1'"},
		{"bad remove method", []string{"--remove=shred", "f"}, "invalid argument 'shred' for '--remove'"},
		{"ambiguous remove method", []string{"--remove=w", "f"}, "ambiguous argument 'w' for '--remove'"},
		{"unknown option", []string{"--nope", "f"}, "unknown option --nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "f", "data")
			r := runBuiltinIn(t, dir, "shred", tc.args...)
			assertExitCode(t, r, 1)
			assertStderrContains(t, r, tc.want)
			if got := string(readShredFile(t, filepath.Join(dir, "f"))); got != "data" {
				t.Error("the file was modified despite the argument error")
			}
		})
	}
}

func TestShredOptionForms(t *testing.T) {
	// Every form here implies -u/--remove, so a successful run leaves no file.
	cases := []struct {
		name string
		args []string
	}{
		{"-n3 attached", []string{"-n3", "-x", "-u"}},
		{"-n 3 separate", []string{"-n", "3", "-x", "-u"}},
		{"--iterations=1", []string{"--iterations=1", "-x", "-u"}},
		{"-u", []string{"-u"}},
		{"-uwipe", []string{"-uwipe"}},
		{"-uwipesync", []string{"-uwipesync"}},
		{"-uunlink", []string{"-uunlink"}},
		{"-uv grouped", []string{"-uv"}},
		{"-vu grouped", []string{"-vu"}},
		{"-zuv grouped", []string{"-zuv"}},
		{"--remove=wipe", []string{"--remove=wipe"}},
		{"--remove=u abbreviation", []string{"--remove=u"}},
		{"--remove=wipes abbreviation", []string{"--remove=wipes"}},
		{"bare --remove", []string{"--remove"}},
		{"-x -f -v -z", []string{"-xfvz", "-u"}},
		{"--exact --zero --verbose", []string{"--exact", "--zero", "--verbose", "-u"}},
		{"--random-source", []string{"--random-source=src", "-x", "-u"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "f", "some data")
			writeFile(t, dir, "src", strings.Repeat("Q", 64))
			args := append(append([]string{}, tc.args...), "f")
			r := runBuiltinIn(t, dir, "shred", args...)
			assertExitCode(t, r, 0)
			if _, err := os.Lstat(filepath.Join(dir, "f")); !os.IsNotExist(err) {
				t.Errorf("file still exists (err = %v)", err)
			}
		})
	}

	// A bare -u must mean wipesync, and -uv must stay verbose rather than
	// setting the removal method to "v".
	dir := t.TempDir()
	writeFile(t, dir, "f", "some data")
	r := runBuiltinIn(t, dir, "shred", "-uv", "f")
	assertExitCode(t, r, 0)
	assertStderrContains(t, r, "shred: f: removing")
	assertStderrContains(t, r, "renamed to")
}

func TestShredHelpVersion(t *testing.T) {
	dir := t.TempDir()

	r := runBuiltinIn(t, dir, "shred", "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{
		"Usage: shred [OPTION]... FILE...",
		"--remove[=HOW]",
		"-n, --iterations=N",
		"--random-source=FILE",
		"Exit status",
		"guarantee",
		"copy-on-write (COW)",
		"wear levelling",
		"btrfs",
		"does not verify",
	} {
		assertStdoutContains(t, r, want)
	}

	r = runBuiltinIn(t, dir, "shred", "-h")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "CAUTION")

	for _, flag := range []string{"--version", "-V"} {
		r = runBuiltinIn(t, dir, "shred", flag)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "shred (Misa) 0.1.0\n")
	}
}
