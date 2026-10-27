package diff

import (
	"bytes"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func optionsGNU(t *testing.T) string {
	t.Helper()
	const gnu = "/usr/bin/diff"
	if out, err := exec.Command(gnu, "--version").Output(); err != nil || !bytes.Contains(out, []byte("GNU diffutils")) {
		t.Skip("/usr/bin/diff is not GNU diffutils")
	}
	return gnu
}

func compareOptionsGNU(t *testing.T, dir string, args ...string) {
	t.Helper()
	compareOptionsGNUInput(t, dir, "", args...)
}

func compareOptionsGNUInput(t *testing.T, dir, stdin string, args ...string) {
	t.Helper()
	cmd := exec.Command(optionsGNU(t), args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC", "TERM=xterm")
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	_ = cmd.Run()
	got := diffRun(t, dir, stdin, args...)
	// GNU prefixes diagnostics with its invocation name.
	wantErr := strings.ReplaceAll(errout.String(), "/usr/bin/diff: ", "")
	gotErr := strings.ReplaceAll(got.stderr, "diff: ", "")
	if got.code != cmd.ProcessState.ExitCode() || got.stdout != out.String() || gotErr != wantErr {
		t.Fatalf("args %q\nmisa code=%d stdout=%q stderr=%q\nGNU  code=%d stdout=%q stderr=%q", args, got.code, got.stdout, got.stderr, cmd.ProcessState.ExitCode(), out.String(), errout.String())
	}
}

func TestDiffOptionsGNU(t *testing.T) {
	optionsGNU(t)
	functionA := "int example(void)\n{\n  one;\n  two;\n  three;\n  old;\n}\n"
	functionB := strings.Replace(functionA, "old", "new", 1)
	cases := []sideCase{
		{"matching_backref", "foofoo\n", "barbar\n", []string{"-I", `^\(.*\)\1$`}},
		{"matching_word", "a foo!\n", "b foo!\n", []string{"-I", `\<foo\>`}},
		{"matching_word_not", "a foobar\n", "b foobar\n", []string{"-I", `\<foo\>`}},
		{"matching_literal_plus", "a+b\n", "other\n", []string{"-I", `a+b\|other`}},
		{"blank_combined", "# old\n\n", "# new\n", []string{"-B", "-I", "^#"}},
		{"tabs_trailing", "a\tb \t\n", "a       b\n", []string{"-EZ"}},
		{"function_many", functionA, functionB, []string{"-u", "-F", "^missing", "-F", "^int"}},
		{"function_long", strings.Repeat("f", 70) + "\n{\n\n\nold\n}\n", strings.Repeat("f", 70) + "\n{\n\n\nnew\n}\n", []string{"-up"}},
		{"function_near", "func()\n{\nold\n}\n", "func()\n{\nnew\n}\n", []string{"-up"}},
		{"regex_option_value", "-u\n", "-u\n-u\n", []string{"-I", "-u"}},
		{"label_option_value", "old\n", "new\n", []string{"-u", "--label", "-c", "--label", "-U5"}},
		{"color_last", "old\n", "new\n", []string{"--color=always", "--color"}},
		{"color_cr", "old\r\n", "new\r\n", []string{"--color=always"}},
		{"color_no_newline", "old", "new", []string{"--color=always", "-u"}},
		{"color_spaces", "old   \n", "new   \n", []string{"--color=always"}},
		{"rcs_tabs", "old\n", "\tnew\n", []string{"-nt"}},
		{"new_left", "", "new\n", []string{"-N"}},
		{"long_matching", "# a\n", "# b\n", []string{"--ignore-matching-lines", "^#"}},
		{"long_function", functionA, functionB, []string{"-u", "--show-function-line=^int"}},
		{"blank", "a\n\nb\n", "a\nb\n", []string{"-B"}},
		{"blank_space", "a\n \nb\n", "a\nb\n", []string{"--ignore-blank-lines"}},
		{"blank_space_ignored", "a\n \nb\n", "a\nb\n", []string{"-B", "-b"}},
		{"blank_mixed", "a\n\nb\n", "A\nb\n", []string{"-B"}},
		{"blank_brief", "a\n\nb\n", "a\nb\n", []string{"-B", "-q"}},
		{"matching", "a\n# old\nb\n", "a\n# new\nb\n", []string{"-I", "^#"}},
		{"matching_mixed", "# old\nold\n", "# new\nnew\n", []string{"--ignore-matching-lines=^#"}},
		{"matching_multiple", "# old\n// old\n", "# new\n// new\n", []string{"-I", "^#", "-I", "^//"}},
		{"matching_bre", "foo12\n", "foo34\n", []string{"-I", `^foo[0-9]\{2\}$`}},
		{"function_default", functionA, functionB, []string{"-p", "--label=old", "--label=new"}},
		{"function_unified", functionA, functionB, []string{"-up"}},
		{"function_regexp", functionA, functionB, []string{"-u", "-F", "^int"}},
		{"function_normal", functionA, functionB, []string{"--show-function-line=^int"}},
		{"function_zero", functionA, functionB, []string{"-U0", "--show-c-function"}},
		{"labels_unified", "old\n", "new\n", []string{"-u", "--label", "before", "--label", "after"}},
		{"labels_context", "old\n", "new\n", []string{"-c", "--label=before", "--label=after"}},
		{"one_label", "old\n", "new\n", []string{"-c", "--label=before"}},
		{"too_many_labels", "old\n", "new\n", []string{"-u", "--label=a", "--label=b", "--label=c"}},
		{"rcs", "a\nb\nc\nd\n", "a\nB\nc\nd\ne\n", []string{"-n"}},
		{"rcs_no_newline", "a\nb\n", "A\nb\nlast", []string{"--rcs"}},
		{"cr_equal", "a\r\nb\r\n", "a\nb\n", []string{"--strip-trailing-cr"}},
		{"cr_changed", "a\r\nold\r\n", "a\r\nnew\r\n", []string{"-u", "--strip-trailing-cr"}},
		{"cr_unterminated", "a\r", "a", []string{"--strip-trailing-cr"}},
		{"tabs", "a\tb\n", "a       b\n", []string{"-E"}},
		{"tabs_four", "a\tb\n", "a   b\n", []string{"--ignore-tab-expansion", "--tabsize=4"}},
		{"tabs_not_all_spaces", "a b\n", "a  b\n", []string{"-E"}},
		{"trailing_no_newline", "a \n", "a", []string{"-Z"}},
		{"trailing_only_newline", "a\n", "a", []string{"-Z"}},
		{"trailing", "a \t\n", "a\n", []string{"-Z"}},
		{"trailing_change", "old  \n", "new\t\n", []string{"--ignore-trailing-space"}},
		{"minimal", "a\nb\n", "a\nB\n", []string{"-d"}},
		{"minimal_long", "a\nb\n", "a\nB\n", []string{"--minimal"}},
	}
	for _, style := range []string{"", "-u", "-c", "-n", "-e"} {
		args := []string{"-B"}
		if style != "" {
			args = append(args, style)
		}
		cases = append(cases, sideCase{"blank_distant_" + style, "x\n\na\nb\nc\nd\ne\nf\ng\nold\n", "x\na\nb\nc\nd\ne\nf\ng\nnew\n", args})
		cases = append(cases, sideCase{"blank_near_" + style, "x\n\na\nb\nc\nold\n", "x\na\nb\nc\nnew\n", args})
	}
	for _, style := range []string{"", "-u", "-c", "-n"} {
		for _, option := range []string{"-T", "--initial-tab", "--color=never", "--color=auto", "--color=always", "--color"} {
			args := []string{option}
			if style != "" {
				args = append(args, style)
			}
			cases = append(cases, sideCase{option + style, "a\n\told\nc\n", "a\n\tnew\nc\n", args})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			diffMkFile(t, dir, "a", tc.a)
			diffMkFile(t, dir, "b", tc.b)
			compareOptionsGNU(t, dir, append(append([]string{}, tc.args...), "a", "b")...)
		})
	}
}

func TestDiffNewFilesGNU(t *testing.T) {
	optionsGNU(t)
	for _, option := range []string{"-N", "--new-file", "--unidirectional-new-file"} {
		for _, style := range []string{"", "-u", "-c", "-n"} {
			for _, missing := range []string{"a", "b"} {
				t.Run(option+style+missing, func(t *testing.T) {
					dir := t.TempDir()
					name := "a"
					if missing == "a" {
						name = "b"
					}
					diffMkFile(t, dir, name, "new\n")
					args := []string{option}
					if style != "" {
						args = append(args, style)
					}
					compareOptionsGNU(t, dir, append(args, "a", "b")...)
				})
			}
		}
	}
}

func TestDiffDirectoryOptionsGNU(t *testing.T) {
	optionsGNU(t)
	for _, args := range [][]string{
		{"-r", "--"}, {"-N"}, {"--unidirectional-new-file"}, {"-N", "-u"}, {"-r", "-u", "--label=before old", "--label=after new"}, {"-r", "-x", "sub"}, {"-r", "-x", "[of]*"}, {"-r", "-X", "patterns", "-x", "left"},
		{"-r", "-x", "*.tmp"}, {"-r", "--exclude=*.tmp", "--exclude=only*"},
		{"-r", "-X", "patterns"}, {"-r", "--exclude-from=patterns"},
		{"-rN"}, {"-r", "--unidirectional-new-file"}, {"-ruN", "--exclude=*.tmp"},
		{"-r", "-u", "--label=left", "--label=right"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{"a/sub/file": "old\n", "b/sub/file": "new\n", "a/only-left": "left\n", "b/only-right": "right\n", "a/left/sub": "left\n", "b/right/sub": "right\n", "a/file.tmp": "a\n", "b/file.tmp": "b\n", "patterns": "*.tmp\nonly*\n"} {
				diffMkFile(t, dir, name, content)
			}
			compareOptionsGNU(t, dir, append(append([]string{}, args...), "a", "b")...)
		})
	}
}

func TestDiffMissingBothGNU(t *testing.T) {
	dir := t.TempDir()
	for _, option := range []string{"-N", "--unidirectional-new-file"} {
		compareOptionsGNU(t, dir, option, "a", "b")
	}
}

func TestDiffContextLocaleGNU(t *testing.T) {
	optionsGNU(t)
	dir := t.TempDir()
	diffMkFile(t, dir, "a", "old\n")
	diffMkFile(t, dir, "b", "new\n")
	// The GNU C.UTF-8 catalog uses ISO timestamps, unlike the C locale.
	cmd := exec.Command("/usr/bin/diff", "-c", "a", "b")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	want, _ := cmd.Output()
	command, _ := builtins.Lookup("diff")
	var out, errout bytes.Buffer
	code := command.Run(&builtins.Context{Dir: dir, Args: []string{"-c", "a", "b"}, Env: []string{"LC_ALL=C.UTF-8"}, Stdout: &out, Stderr: &errout})
	if out.String() != string(want) || code != cmd.ProcessState.ExitCode() || errout.Len() != 0 {
		t.Fatalf("misa code=%d %q %q; GNU code=%d %q", code, out.String(), errout.String(), cmd.ProcessState.ExitCode(), want)
	}
}

func TestDiffMissingDirectoriesGNU(t *testing.T) {
	optionsGNU(t)
	for _, option := range []string{"-N", "--unidirectional-new-file"} {
		for _, recursive := range []bool{false, true} {
			for _, missing := range []string{"a", "b"} {
				t.Run(option+missing+fmt.Sprint(recursive), func(t *testing.T) {
					dir := t.TempDir()
					name := "a"
					if missing == "a" {
						name = "b"
					}
					diffMkFile(t, dir, name+"/file", "new\n")
					diffMkFile(t, dir, name+"/sub/file", "nested\n")
					args := []string{option, "-u"}
					if recursive {
						args = append(args, "-r")
					}
					compareOptionsGNU(t, dir, append(args, "a", "b")...)
				})
			}
		}
	}
}

func TestDiffExcludeStdinGNU(t *testing.T) {
	optionsGNU(t)
	dir := t.TempDir()
	for name, data := range map[string]string{"a/skip.tmp": "old\n", "b/skip.tmp": "new\n", "a/f": "old\n", "b/f": "new\n"} {
		diffMkFile(t, dir, name, data)
	}
	compareOptionsGNUInput(t, dir, "*.tmp\n", "-r", "-X", "-", "a", "b")
	compareOptionsGNU(t, dir, "-r", "-X", "absent", "a", "b")
}
