// Ported from the `mod cmp` integration tests of
// https://github.com/uutils/diffutils (tests/integration.rs, MIT
// licensed), adapted to the in-process builtins harness: the Rust
// Command-based cases become TestCmp_<name> functions.
package cmp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

type result struct {
	code   int
	stdout string
	stderr string
}

func runCmpIn(t *testing.T, dir string, env []string, stdin string, args ...string) result {
	t.Helper()
	cmd, ok := builtins.Lookup("cmp")
	if !ok {
		t.Fatalf("builtin %q not registered", "cmp")
	}
	var out, errOut bytes.Buffer
	ctx := &builtins.Context{
		Dir:     dir,
		Args:    args,
		CmdName: "cmp",
		Width:   80,
		Height:  24,
		Stdout:  &out,
		Stderr:  &errOut,
		Stdin:   strings.NewReader(stdin),
		Env:     env,
	}
	code := cmd.Run(ctx)
	return result{code: code, stdout: out.String(), stderr: errOut.String()}
}

func writeCmpFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmp_unknown_param(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "")
	writeCmpFile(t, dir, "file2", "")

	r := runCmpIn(t, dir, nil, "", "--foobar", "file1", "file2")

	if r.code != 2 {
		t.Errorf("exit = %d, want 2", r.code)
	}
	if !strings.HasSuffix(r.stderr, "unrecognized option '--foobar'\n") {
		t.Errorf("stderr = %q, want suffix %q", r.stderr, "unrecognized option '--foobar'\n")
	}
}

func TestCmp_cannot_read_files(t *testing.T) {
	dir := t.TempDir()
	nopath := filepath.Join(dir, "doesnotexist")
	writeCmpFile(t, dir, "existing", "abc")

	for _, files := range [][]string{{nopath, "existing"}, {"existing", nopath}} {
		r := runCmpIn(t, dir, nil, "", files[0], files[1])
		if r.code != 2 {
			t.Errorf("cmp %v: exit = %d, want 2", files, r.code)
		}
		wantSuffix := ": " + nopath + ": No such file or directory\n"
		if !strings.HasSuffix(r.stderr, wantSuffix) {
			t.Errorf("cmp %v: stderr = %q, want suffix %q", files, r.stderr, wantSuffix)
		}
	}
}

func TestCmp_verbose_zero_length_metadata(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip("uutils-only: /dev/zero unavailable")
	}
	dir := t.TempDir()
	writeCmpFile(t, dir, "file150.bin", strings.Repeat("\xff", 150))

	r := runCmpIn(t, dir, nil, "", "-l", "/dev/zero", "file150.bin")

	if r.code != 1 {
		t.Errorf("exit = %d, want 1", r.code)
	}
	if !strings.Contains(r.stdout, "150   0 377") {
		t.Errorf("stdout = %q, want it to contain %q", r.stdout, "150   0 377")
	}
	if !strings.Contains(r.stderr, " EOF on") {
		t.Errorf("stderr = %q, want it to contain %q", r.stderr, " EOF on")
	}
}

func TestCmp_incompatible_params(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "")
	writeCmpFile(t, dir, "file2", "")

	r := runCmpIn(t, dir, nil, "", "-l", "-s", "file1", "file2")

	if r.code != 2 {
		t.Errorf("exit = %d, want 2", r.code)
	}
	if !strings.HasSuffix(r.stderr, ": options -l and -s are incompatible\n") {
		t.Errorf("stderr = %q, want suffix %q", r.stderr, ": options -l and -s are incompatible\n")
	}
}

func TestCmp_stdin(t *testing.T) {
	dir := t.TempDir()
	path := writeCmpFile(t, dir, "file1", "a\n")

	r := runCmpIn(t, dir, nil, "a\n", path)
	if r.code != 0 {
		t.Errorf("exit = %d, want 0", r.code)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}

	r = runCmpIn(t, dir, []string{"LC_ALL=C"}, "b\n", path)
	if r.code != 1 {
		t.Errorf("exit = %d, want 1", r.code)
	}
	if !strings.HasSuffix(r.stdout, " - differ: char 1, line 1\n") {
		t.Errorf("stdout = %q, want suffix %q", r.stdout, " - differ: char 1, line 1\n")
	}
}

func TestCmp_equal_files(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "abc")
	writeCmpFile(t, dir, "file2", "abc")

	r := runCmpIn(t, dir, nil, "", "file1", "file2")

	if r.code != 0 {
		t.Errorf("exit = %d, want 0", r.code)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestCmp_one_file_empty(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "abc")
	writeCmpFile(t, dir, "file2", "")

	r := runCmpIn(t, dir, nil, "", "file1", "file2")

	if r.code != 1 {
		t.Errorf("exit = %d, want 1", r.code)
	}
	if !strings.Contains(r.stderr, " EOF on ") {
		t.Errorf("stderr = %q, want it to contain %q", r.stderr, " EOF on ")
	}
	if !strings.HasSuffix(r.stderr, " which is empty\n") {
		t.Errorf("stderr = %q, want suffix %q", r.stderr, " which is empty\n")
	}
}

func TestCmp_immediate_difference(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "abc\n")
	writeCmpFile(t, dir, "file2", "bcd\n")
	env := []string{"LC_ALL=C"}

	steps := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{name: "plain", args: []string{"file1", "file2"}, wantCode: 1, wantStdout: "file1 file2 differ: char 1, line 1\n"},
		{name: "print_bytes", args: []string{"-b", "file1", "file2"}, wantCode: 1,
			wantStdout: "file1 file2 differ: byte 1, line 1 is 141 a 142 b\n"},
		{name: "verbose", args: []string{"-l", "file1", "file2"}, wantCode: 1,
			wantStdout: "1 141 142\n2 142 143\n3 143 144\n"},
		{name: "verbose_print_bytes", args: []string{"-l", "-b", "file1", "file2"}, wantCode: 1,
			wantStdout: "1 141 a    142 b\n2 142 b    143 c\n3 143 c    144 d\n"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			r := runCmpIn(t, dir, env, "", step.args...)
			if r.code != step.wantCode {
				t.Errorf("exit = %d, want %d", r.code, step.wantCode)
			}
			if r.stdout != step.wantStdout {
				t.Errorf("stdout = %q, want %q", r.stdout, step.wantStdout)
			}
		})
	}
}

func TestCmp_newline_difference(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "abc\ndefg")
	writeCmpFile(t, dir, "file2", "abc\ndef\ng")
	env := []string{"LC_ALL=C"}

	r := runCmpIn(t, dir, env, "", "file1", "file2")
	if r.code != 1 {
		t.Errorf("exit = %d, want 1", r.code)
	}
	if !strings.HasSuffix(r.stdout, " differ: char 8, line 2\n") {
		t.Errorf("stdout = %q, want suffix %q", r.stdout, " differ: char 8, line 2\n")
	}

	r = runCmpIn(t, dir, env, "", "-b", "file1", "file2")
	if r.code != 1 {
		t.Errorf("-b exit = %d, want 1", r.code)
	}
	if !strings.HasSuffix(r.stdout, " differ: byte 8, line 2 is 147 g  12 ^J\n") {
		t.Errorf("-b stdout = %q, want suffix %q", r.stdout, " differ: byte 8, line 2 is 147 g  12 ^J\n")
	}

	r = runCmpIn(t, dir, env, "", "-l", "file1", "file2")
	if r.code != 1 {
		t.Errorf("-l exit = %d, want 1", r.code)
	}
	if !strings.HasPrefix(r.stdout, "8 147  12\n") {
		t.Errorf("-l stdout = %q, want prefix %q", r.stdout, "8 147  12\n")
	}
	if !strings.Contains(r.stderr, " EOF on") || !strings.HasSuffix(r.stderr, " after byte 8\n") {
		t.Errorf("-l stderr = %q, want it to contain %q and end with %q", r.stderr, " EOF on", " after byte 8\n")
	}

	r = runCmpIn(t, dir, env, "", "-l", "-b", "file1", "file2")
	if r.code != 1 {
		t.Errorf("-l -b exit = %d, want 1", r.code)
	}
	if !strings.HasPrefix(r.stdout, "8 147 g     12 ^J\n") {
		t.Errorf("-l -b stdout = %q, want prefix %q", r.stdout, "8 147 g     12 ^J\n")
	}
	if !strings.Contains(r.stderr, " EOF on") || !strings.HasSuffix(r.stderr, " after byte 8\n") {
		t.Errorf("-l -b stderr = %q, want it to contain %q and end with %q", r.stderr, " EOF on", " after byte 8\n")
	}
}

func TestCmp_max_bytes(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "abc efg ijkl\n")
	writeCmpFile(t, dir, "file2", "abcdefghijkl\n")

	steps := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{name: "equal_within_limit", args: []string{"-l", "-b", "-n", "3", "file1", "file2"},
			wantCode: 0, wantStdout: ""},
		{name: "limit_four", args: []string{"-l", "-b", "-n", "4", "file1", "file2"},
			wantCode: 1, wantStdout: "4  40      144 d\n"},
		{name: "limit_thirteen", args: []string{"-l", "-b", "-n", "13", "file1", "file2"},
			wantCode: 1, wantStdout: " 4  40      144 d\n 8  40      150 h\n"},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			r := runCmpIn(t, dir, nil, "", step.args...)
			if r.code != step.wantCode {
				t.Errorf("exit = %d, want %d", r.code, step.wantCode)
			}
			if r.stdout != step.wantStdout {
				t.Errorf("stdout = %q, want %q", r.stdout, step.wantStdout)
			}
		})
	}
}

func TestCmp_skip_args_parsing(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "---abc\n")
	writeCmpFile(t, dir, "file2", "###abc\n")
	env := []string{"LC_ALL=C"}

	steps := []struct {
		name       string
		args       []string
		wantCode   int
		wantErrSuf string
	}{
		{name: "flag_skips_both", args: []string{"-i", "3", "file1", "file2"}, wantCode: 0},
		{name: "flag_and_positional", args: []string{"-i", "3", "file1", "file2", "1", "1"}, wantCode: 0},
		{name: "positional_skips_first", args: []string{"file1", "file2", "3"}, wantCode: 1,
			wantErrSuf: " differ: char 1, line 1\n"},
		{name: "two_positionals", args: []string{"file1", "file2", "3", "3"}, wantCode: 0},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			r := runCmpIn(t, dir, env, "", step.args...)
			if r.code != step.wantCode {
				t.Errorf("exit = %d, want %d", r.code, step.wantCode)
			}
			if step.wantErrSuf != "" && !strings.HasSuffix(r.stdout, step.wantErrSuf) {
				t.Errorf("stdout = %q, want suffix %q", r.stdout, step.wantErrSuf)
			}
		})
	}
}

func TestCmp_skip_suffix_parsing(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", strings.Repeat("a", 1024)+"\n")
	writeCmpFile(t, dir, "file2", strings.Repeat("b", 1024)+"\n")

	r := runCmpIn(t, dir, nil, "", "--ignore-initial=1K", "file1", "file2")

	if r.code != 0 {
		t.Errorf("exit = %d, want 0", r.code)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestCmp_skip(t *testing.T) {
	dir := t.TempDir()
	writeCmpFile(t, dir, "file1", "abc efg ijkl\n")
	writeCmpFile(t, dir, "file2", "abcdefghijkl\n")
	env := []string{"LC_ALL=C"}

	r := runCmpIn(t, dir, env, "", "-l", "-b", "-i", "8", "file1", "file2")
	if r.code != 0 {
		t.Errorf("-l -b -i 8: exit = %d, want 0", r.code)
	}
	if r.stdout != "" {
		t.Errorf("-l -b -i 8: stdout = %q, want empty", r.stdout)
	}

	r = runCmpIn(t, dir, env, "", "-b", "-i", "4", "file1", "file2")
	if r.code != 1 {
		t.Errorf("-b -i 4: exit = %d, want 1", r.code)
	}
	wantSuffix := " differ: byte 4, line 1 is  40   150 h\n"
	if !strings.HasSuffix(r.stdout, wantSuffix) {
		t.Errorf("-b -i 4: stdout = %q, want suffix %q", r.stdout, wantSuffix)
	}
}

func TestCmp_binary(t *testing.T) {
	dir := t.TempDir()
	input1 := []byte{0, 15, 31, 32, 33, 40, 64, 126, 127, 128, 129, 200, 254, 255}
	input2 := make([]byte, len(input1))
	for i := range input1 {
		input2[i] = input1[len(input1)-1-i]
	}
	if err := os.WriteFile(filepath.Join(dir, "input"), input1, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input2"), input2, 0o644); err != nil {
		t.Fatal(err)
	}

	r := runCmpIn(t, dir, nil, "", "-l", "-b", "input", "input2")

	if r.code != 1 {
		t.Errorf("exit = %d, want 1", r.code)
	}
	expected := " 1   0 ^@   377 M-^?\n" +
		" 2  17 ^O   376 M-~\n" +
		" 3  37 ^_   310 M-H\n" +
		" 4  40      201 M-^A\n" +
		" 5  41 !    200 M-^@\n" +
		" 6  50 (    177 ^?\n" +
		" 7 100 @    176 ~\n" +
		" 8 176 ~    100 @\n" +
		" 9 177 ^?    50 (\n" +
		"10 200 M-^@  41 !\n" +
		"11 201 M-^A  40  \n" +
		"12 310 M-H   37 ^_\n" +
		"13 376 M-~   17 ^O\n" +
		"14 377 M-^?   0 ^@\n"
	if r.stdout != expected {
		t.Errorf("stdout = %q, want %q", r.stdout, expected)
	}
}

func TestCmp_fast_paths(t *testing.T) {
	t.Skip("uutils-only: /dev/null stdout fast path and sparse14 TiB files")
}
