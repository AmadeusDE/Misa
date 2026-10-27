// Derived from uutils/diffutils tests/integration.rs (MIT License).
//
// Go port of the upstream `mod diff` integration tests plus the `mod common`
// cases relevant to `diff`. Runs the `diff` builtin in-process via
// builtins.Lookup + builtins.Context, modelled on tests/native/helpers_test.go.
// Stdlib only.

package diff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

type diffResult struct {
	code   int
	stdout string
	stderr string
}

func diffRun(t *testing.T, dir, stdin string, args ...string) diffResult {
	t.Helper()
	cmd, ok := builtins.Lookup("diff")
	if !ok {
		t.Fatal(`builtins.Lookup("diff") failed`)
	}
	var out, errb bytes.Buffer
	ctx := &builtins.Context{
		Dir:    dir,
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &out,
		Stderr: &errb,
		Stdin:  strings.NewReader(stdin),
		Env:    []string{"LC_ALL=C"},
	}
	code := cmd.Run(ctx)
	return diffResult{code: code, stdout: out.String(), stderr: errb.String()}
}

var diffMtime = time.Date(2024, 3, 24, 23, 43, 5, 189597645, time.UTC)

func diffMkFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, diffMtime, diffMtime); err != nil {
		t.Fatal(err)
	}
	return path
}

var diffTimestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+ [+-]\d{4}`)

// diffAssertDiffEq compares diff output with mtime header stamps normalised to
// "TIMESTAMP", like upstream's assert_diff_eq! macro.
func diffAssertDiffEq(t *testing.T, got, want string) {
	t.Helper()
	if got := diffTimestampRe.ReplaceAllString(got, "TIMESTAMP"); got != want {
		t.Errorf("diff output = %q, want %q", got, want)
	}
}

func diffWantCode(t *testing.T, r diffResult, want int) {
	t.Helper()
	if r.code != want {
		t.Errorf("exit code = %d, want %d (stderr: %s)", r.code, want, r.stderr)
	}
}

func diffWantStdout(t *testing.T, r diffResult, want string) {
	t.Helper()
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func diffWantStderrSuffix(t *testing.T, r diffResult, suffix string) {
	t.Helper()
	if !strings.HasSuffix(r.stderr, suffix) {
		t.Errorf("stderr %q does not end with %q", r.stderr, suffix)
	}
}

func diffRequireTool(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skip("requires tool: " + tool)
	}
}

var diffOpts = []string{"", "-u", "-c", "-e"}

func diffOptName(opt string) string {
	if opt == "" {
		return "default"
	}
	return strings.TrimPrefix(opt, "-")
}

func diffOptArgs(opt string, rest ...string) []string {
	args := []string{}
	if opt != "" {
		args = append(args, opt)
	}
	return append(args, rest...)
}

func TestDiff_no_differences(t *testing.T) {
	dir := t.TempDir()
	path := diffMkFile(t, dir, "file", "")
	for _, opt := range diffOpts {
		t.Run(diffOptName(opt), func(t *testing.T) {
			r := diffRun(t, dir, "", diffOptArgs(opt, path, path)...)
			diffWantCode(t, r, 0)
			diffWantStdout(t, r, "")
		})
	}
}

func TestDiff_no_differences_report_identical_files(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "foo\n")
	file2 := diffMkFile(t, dir, "file2", "foo\n")
	for _, tc := range []struct{ name, from, to string }{
		{"same_file", file1, file1},
		{"equal_content", file1, file2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, opt := range diffOpts {
				t.Run(diffOptName(opt), func(t *testing.T) {
					r := diffRun(t, dir, "", diffOptArgs(opt, "-s", tc.from, tc.to)...)
					diffWantCode(t, r, 0)
					diffWantStdout(t, r, fmt.Sprintf("Files %s and %s are identical\n", tc.from, tc.to))
				})
			}
		})
	}
}

func TestDiff_differences(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "foo\n")
	file2 := diffMkFile(t, dir, "file2", "bar\n")
	for _, opt := range diffOpts {
		t.Run(diffOptName(opt), func(t *testing.T) {
			r := diffRun(t, dir, "", diffOptArgs(opt, file1, file2)...)
			diffWantCode(t, r, 1)
			if r.stdout == "" {
				t.Error("expected non-empty stdout")
			}
		})
	}
}

func TestDiff_differences_brief(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "foo\n")
	file2 := diffMkFile(t, dir, "file2", "bar\n")
	for _, opt := range diffOpts {
		t.Run(diffOptName(opt), func(t *testing.T) {
			r := diffRun(t, dir, "", diffOptArgs(opt, "-q", file1, file2)...)
			diffWantCode(t, r, 1)
			diffWantStdout(t, r, fmt.Sprintf("Files %s and %s differ\n", file1, file2))
		})
	}
}

func TestDiff_missing_newline(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "foo")
	file2 := diffMkFile(t, dir, "file2", "bar")
	r := diffRun(t, dir, "", "-e", file1, file2)
	diffWantCode(t, r, 2)
	if !strings.HasPrefix(r.stderr, "No newline at end of file") {
		t.Errorf("stderr = %q, want prefix %q", r.stderr, "No newline at end of file")
	}
}

func TestDiff_read_from_stdin(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "foo\n")
	file2 := diffMkFile(t, dir, "file2", "bar\n")

	t.Run("file_dash", func(t *testing.T) {
		r := diffRun(t, dir, "bar\n", "-u", file1, "-")
		diffWantCode(t, r, 1)
		diffAssertDiffEq(t, r.stdout,
			fmt.Sprintf("--- %s\tTIMESTAMP\n+++ -\tTIMESTAMP\n@@ -1 +1 @@\n-foo\n+bar\n", file1))
	})
	t.Run("dash_file", func(t *testing.T) {
		r := diffRun(t, dir, "foo\n", "-u", "-", file2)
		diffWantCode(t, r, 1)
		diffAssertDiffEq(t, r.stdout,
			fmt.Sprintf("--- -\tTIMESTAMP\n+++ %s\tTIMESTAMP\n@@ -1 +1 @@\n-foo\n+bar\n", file2))
	})
	t.Run("dash_dash", func(t *testing.T) {
		r := diffRun(t, dir, "", "-u", "-", "-")
		diffWantCode(t, r, 0)
		diffWantStdout(t, r, "")
	})
	t.Run("dev_stdin", func(t *testing.T) {
		t.Skip("uutils-only: /dev/stdin fast path")
	})
}

func TestDiff_compare_file_to_directory(t *testing.T) {
	dir := t.TempDir()
	aPath := diffMkFile(t, dir, "a", "a\n")
	daPath := diffMkFile(t, dir, filepath.Join("d", "a"), "da\n")

	t.Run("dir_to_file", func(t *testing.T) {
		r := diffRun(t, dir, "", "-u", filepath.Join(dir, "d"), aPath)
		diffWantCode(t, r, 1)
		diffAssertDiffEq(t, r.stdout,
			fmt.Sprintf("--- %s\tTIMESTAMP\n+++ %s\tTIMESTAMP\n@@ -1 +1 @@\n-da\n+a\n", daPath, aPath))
	})
	t.Run("file_to_dir", func(t *testing.T) {
		r := diffRun(t, dir, "", "-u", aPath, filepath.Join(dir, "d"))
		diffWantCode(t, r, 1)
		diffAssertDiffEq(t, r.stdout,
			fmt.Sprintf("--- %s\tTIMESTAMP\n+++ %s\tTIMESTAMP\n@@ -1 +1 @@\n-a\n+da\n", aPath, daPath))
	})
}

// Regression for upstream #269: the -y gutter marker must sit at the middle
// of the gutter. Expectations follow GNU diff 3.12, which pairs a changed
// line onto one row with "|" (upstream uutils printed "<" then ">").
func TestDiff_sdiff_gutter_marker_column(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "aa\n")
	file2 := diffMkFile(t, dir, "file2", "bb\n")

	t.Run("expand_tabs", func(t *testing.T) {
		r := diffRun(t, dir, "", "-y", "--width=40", "--expand-tabs", file1, file2)
		diffWantStdout(t, r, "aa                 |  bb\n")
	})
	t.Run("default_tabs", func(t *testing.T) {
		r := diffRun(t, dir, "", "-y", "--width=40", file1, file2)
		diffWantStdout(t, r, "aa\t\t   |\tbb\n")
	})
}

func TestDiff_unknown_param(t *testing.T) {
	t.Run("diff", func(t *testing.T) {
		r := diffRun(t, t.TempDir(), "", "--foobar")
		diffWantCode(t, r, 2)
		if !strings.Contains(r.stderr, "unrecognized option '--foobar'") {
			t.Errorf("stderr %q does not contain %q", r.stderr, "unrecognized option '--foobar'")
		}
	})
	for _, sub := range []string{"cmp", "uu-diff", "uucmp"} {
		t.Run(sub, func(t *testing.T) {
			t.Skip("uutils-only: multi-call dispatcher subcommand")
		})
	}
	t.Run("patch_dispatch", func(t *testing.T) {
		t.Skip("uutils-only: multi-call dispatcher (patch: utility not supported)")
	})
	t.Run("no_args_dispatch", func(t *testing.T) {
		t.Skip("uutils-only: multi-call dispatcher (Expected utility name as second argument)")
	})
}

func TestDiff_cannot_read_files(t *testing.T) {
	dir := t.TempDir()
	file := diffMkFile(t, dir, "file", "foo\n")
	missing := filepath.Join(dir, "nopath")
	wantSuffix := ": " + missing + ": No such file or directory\n"

	t.Run("missing_from", func(t *testing.T) {
		r := diffRun(t, dir, "", missing, file)
		diffWantCode(t, r, 2)
		diffWantStderrSuffix(t, r, wantSuffix)
	})
	t.Run("missing_to", func(t *testing.T) {
		r := diffRun(t, dir, "", file, missing)
		diffWantCode(t, r, 2)
		diffWantStderrSuffix(t, r, wantSuffix)
	})
	t.Run("both_missing", func(t *testing.T) {
		r := diffRun(t, dir, "", missing, missing)
		diffWantCode(t, r, 2)
		if n := strings.Count(r.stderr, wantSuffix); n != 2 {
			t.Errorf("stderr contains error %d times, want 2: %q", n, r.stderr)
		}
	})
}

// Coverage beyond upstream integration.rs: -a treats files as text (no-op).
func TestDiff_coverage_a_flag(t *testing.T) {
	dir := t.TempDir()

	t.Run("identical", func(t *testing.T) {
		file1 := diffMkFile(t, dir, "same1", "text\n")
		file2 := diffMkFile(t, dir, "same2", "text\n")
		r := diffRun(t, dir, "", "-a", file1, file2)
		diffWantCode(t, r, 0)
		diffWantStdout(t, r, "")
	})
	t.Run("binary_bytes", func(t *testing.T) {
		file1 := diffMkFile(t, dir, "bin1", "a\x00b\n")
		file2 := diffMkFile(t, dir, "bin2", "a\x00c\n")
		r := diffRun(t, dir, "", "-a", file1, file2)
		diffWantCode(t, r, 1)
		if r.stdout == "" {
			t.Error("expected non-empty stdout")
		}
	})
	t.Run("normal_format", func(t *testing.T) {
		file1 := diffMkFile(t, dir, "norm1", "foo\n")
		file2 := diffMkFile(t, dir, "norm2", "bar\n")
		r := diffRun(t, dir, "", "-a", file1, file2)
		diffWantCode(t, r, 1)
		diffWantStdout(t, r, "1c1\n< foo\n---\n> bar\n")
	})
}

// Coverage beyond upstream integration.rs: toybox cat.test runs
// `cat - file1 | diff -a -U 0 - file1 | tail -n 1` and wants "-hello\n".
func TestDiff_coverage_unified_zero_stdin(t *testing.T) {
	dir := t.TempDir()
	file1 := diffMkFile(t, dir, "file1", "hello\n")
	r := diffRun(t, dir, "hello\nhello\n", "-a", "-U", "0", "-", file1)
	diffWantCode(t, r, 1)
	diffAssertDiffEq(t, r.stdout,
		fmt.Sprintf("--- -\tTIMESTAMP\n+++ %s\tTIMESTAMP\n@@ -2 +1,0 @@\n-hello\n", file1))
}

// Coverage beyond upstream integration.rs: toybox cp.test runs `diff -r one dir`.
func TestDiff_coverage_recursive(t *testing.T) {
	dir := t.TempDir()
	diffMkFile(t, dir, filepath.Join("a", "f1"), "same\n")
	diffMkFile(t, dir, filepath.Join("b", "f1"), "same\n")
	diffMkFile(t, dir, filepath.Join("a", "f2"), "x\n")
	diffMkFile(t, dir, filepath.Join("b", "f2"), "y\n")
	diffMkFile(t, dir, filepath.Join("a", "only_a"), "a\n")
	diffMkFile(t, dir, filepath.Join("b", "only_b"), "b\n")

	t.Run("differing", func(t *testing.T) {
		r := diffRun(t, dir, "", "-r", "a", "b")
		diffWantCode(t, r, 1)
		for _, want := range []string{
			"diff -r a/f2 b/f2",
			"Only in a: only_a\n",
			"Only in b: only_b\n",
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("stdout %q does not contain %q", r.stdout, want)
			}
		}
		if strings.Contains(r.stdout, "f1") {
			t.Errorf("stdout %q should not mention identical file f1", r.stdout)
		}
	})
	t.Run("identical", func(t *testing.T) {
		diffMkFile(t, dir, filepath.Join("c", "f1"), "same\n")
		diffMkFile(t, dir, filepath.Join("d", "f1"), "same\n")
		r := diffRun(t, dir, "", "-r", "c", "d")
		diffWantCode(t, r, 0)
		diffWantStdout(t, r, "")
	})
}

// Coverage beyond upstream integration.rs: -i, -b, -w whitespace folding.
func TestDiff_coverage_whitespace(t *testing.T) {
	dir := t.TempDir()

	t.Run("i", func(t *testing.T) {
		from := diffMkFile(t, dir, "i1", "foo\n")
		to := diffMkFile(t, dir, "i2", "FOO\n")
		diffWantCode(t, diffRun(t, dir, "", from, to), 1)
		diffWantCode(t, diffRun(t, dir, "", "-i", from, to), 0)
	})
	t.Run("b", func(t *testing.T) {
		from := diffMkFile(t, dir, "b1", "a  b\n")
		to := diffMkFile(t, dir, "b2", "a b\n")
		diffWantCode(t, diffRun(t, dir, "", from, to), 1)
		diffWantCode(t, diffRun(t, dir, "", "-b", from, to), 0)
	})
	t.Run("b_eol", func(t *testing.T) {
		from := diffMkFile(t, dir, "be1", "a \n")
		to := diffMkFile(t, dir, "be2", "a\n")
		diffWantCode(t, diffRun(t, dir, "", "-b", from, to), 0)
	})
	t.Run("w", func(t *testing.T) {
		from := diffMkFile(t, dir, "w1", "a b c\n")
		to := diffMkFile(t, dir, "w2", "abc\n")
		diffWantCode(t, diffRun(t, dir, "", from, to), 1)
		diffWantCode(t, diffRun(t, dir, "", "-w", from, to), 0)
	})
}

func TestDiff_coverage_patch_roundtrip(t *testing.T) {
	diffRequireTool(t, "patch")
	dir := t.TempDir()
	diffMkFile(t, dir, "old", "line1\nline2\n")
	diffMkFile(t, dir, "new", "line1\nline2 changed\n")

	r := diffRun(t, dir, "", "-u", "old", "new")
	diffWantCode(t, r, 1)

	// Only "old" remains so patch has a single candidate to apply to.
	if err := os.Remove(filepath.Join(dir, "new")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("patch", "--batch", "-p0")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(r.stdout)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("patch: %v: %s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "old"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line1\nline2 changed\n" {
		t.Errorf("patched old = %q, want %q", got, "line1\nline2 changed\n")
	}
}

func TestDiff_coverage_ed_roundtrip(t *testing.T) {
	diffRequireTool(t, "ed")
	dir := t.TempDir()
	diffMkFile(t, dir, "old", "line1\nline2\n")
	diffMkFile(t, dir, "new", "line1\nline2 changed\n")

	r := diffRun(t, dir, "", "-e", "old", "new")
	diffWantCode(t, r, 1)

	cmd := exec.Command("ed", "-s", "old")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(r.stdout + "w\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ed: %v: %s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "old"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line1\nline2 changed\n" {
		t.Errorf("ed-scripted old = %q, want %q", got, "line1\nline2 changed\n")
	}
}
