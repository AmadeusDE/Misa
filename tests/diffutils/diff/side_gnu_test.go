// Side-by-side (-y) behaviour checked against GNU diff.

package diff

import (
	"os/exec"
	"strings"
	"testing"
)

type sideCase struct {
	name string
	a, b string
	args []string
}

var sideCases = []sideCase{
	{"change", "a\nb\nc\n", "a\nB\nc\n", nil},
	{"multi_change_uneven", "a\nb\nc\nd\ne\n", "a\nX\nY\nZ\ne\n", nil},
	{"more_dels_than_adds", "a\nb\nc\nd\ne\n", "a\nX\ne\n", nil},
	{"pure_add", "a\nc\n", "a\nb\nc\n", nil},
	{"pure_delete", "a\nb\nc\n", "a\nc\n", nil},
	{"all_different", "1\n2\n", "x\ny\nz\n", nil},
	{"identical", "a\nb\n", "a\nb\n", nil},
	{"width_40", "hello world line\nsecond\n", "hello there line\nsecond\n", []string{"-W", "40"}},
	{"width_20_truncate", "a very long line that will be cut\nx\n", "another very long line here\nx\n", []string{"--width=20"}},
	{"tabs", "a\tb\nc\n", "a\tB\nc\n", nil},
	{"tabs_expanded", "a\tb\nc\n", "a\tB\nc\n", []string{"-t"}},
	{"suppress_common", "a\nb\nc\nd\n", "a\nB\nc\nd\ne\n", []string{"--suppress-common-lines"}},
	{"left_column", "a\nb\nc\n", "a\nB\nc\nd\n", []string{"--left-column"}},
	{"left_column_suppress", "a\nb\nc\n", "a\nB\nc\n", []string{"--left-column", "--suppress-common-lines"}},
	{"empty_left", "", "a\nb\n", nil},
	{"empty_right", "a\nb\n", "", nil},
	{"no_eol_left", "a\nb", "a\nB\n", nil},
	{"no_eol_right", "a\nb\n", "a\nB", nil},
	{"no_eol_both", "a\nb", "a\nB", nil},
	{"utf8_wide", "\u65e5\u672c\u8a9e\nx\n", "\u65e5\u672c\nx\n", []string{"-W", "30"}},
	{"tabsize_4", "\ta\tb\n", "\ta\tc\n", []string{"--tabsize=4", "-W", "50"}},
	{"ignore_case", "Hello\nb\n", "hello\nc\n", []string{"-i"}},
	{"default_width_long", strings.Repeat("x", 100) + "\n", strings.Repeat("y", 100) + "\n", nil},
	{"interleaved", "1\n2\n3\n4\n5\n6\n", "1\nb\n3\n5\nX\nY\n6\n7\n", []string{"-W", "60"}},
}

func TestDiffSideBySideChangeGolden(t *testing.T) {
	dir := t.TempDir()
	diffMkFile(t, dir, "a", "a\nb\nc\n")
	diffMkFile(t, dir, "b", "a\nB\nc\n")
	r := diffRun(t, dir, "", "-y", "-W", "20", "a", "b")
	want := "a\ta\nb     |\tB\nc\tc\n" // GNU diff 3.12 output
	if r.code != 1 || r.stdout != want {
		t.Fatalf("code=%d stdout=%q want %q", r.code, r.stdout, want)
	}
}

func TestDiffSideBySideMatchesGNU(t *testing.T) {
	gnu, err := exec.LookPath("diff")
	if err != nil {
		t.Skip("GNU diff not installed")
	}
	if out, err := exec.Command(gnu, "--version").Output(); err != nil || !bytesContains(out, "GNU diffutils") {
		t.Skip("system diff is not GNU diffutils")
	}
	for _, tc := range sideCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			diffMkFile(t, dir, "a", tc.a)
			diffMkFile(t, dir, "b", tc.b)
			args := append(append([]string{"-y"}, tc.args...), "a", "b")
			cmd := exec.Command(gnu, args...)
			cmd.Dir = dir
			// Misa is always UTF-8 aware, so compare against GNU in a UTF-8 locale.
			cmd.Env = []string{"LC_ALL=C.UTF-8"}
			want, _ := cmd.Output()
			wantCode := cmd.ProcessState.ExitCode()
			got := diffRun(t, dir, "", args...)
			if got.stdout != string(want) || got.code != wantCode {
				t.Fatalf("args %v\nmisa code=%d %q\ngnu  code=%d %q", args, got.code, got.stdout, wantCode, want)
			}
		})
	}
}

func bytesContains(b []byte, s string) bool {
	return len(s) == 0 || indexOf(string(b), s) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestDiffEdMatchesGNU(t *testing.T) {
	gnu, err := exec.LookPath("diff")
	if err != nil {
		t.Skip("GNU diff not installed")
	}
	if out, err := exec.Command(gnu, "--version").Output(); err != nil || !bytesContains(out, "GNU diffutils") {
		t.Skip("system diff is not GNU diffutils")
	}
	for _, tc := range []sideCase{
		{"multi_hunk", "x\n\na\nb\nc\nd\ne\nf\ng\n", "x\na\nb\nC\nd\ne\nf\ng\nh\n", nil},
		{"single_delete", "1\n2\n3\n", "1\n3\n", nil},
		{"range_change", "1\n2\n3\n4\n", "1\nx\ny\n4\n", nil},
		{"dot_lines", "q\n", ".\n.\nz\n.\n", nil},
		{"append_end", "1\n", "1\n2\n3\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			diffMkFile(t, dir, "a", tc.a)
			diffMkFile(t, dir, "b", tc.b)
			cmd := exec.Command(gnu, "-e", "a", "b")
			cmd.Dir = dir
			want, _ := cmd.Output()
			got := diffRun(t, dir, "", "-e", "a", "b")
			if got.stdout != string(want) || got.code != cmd.ProcessState.ExitCode() {
				t.Fatalf("misa code=%d %q\ngnu  code=%d %q", got.code, got.stdout, cmd.ProcessState.ExitCode(), want)
			}
		})
	}
}
