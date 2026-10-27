package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffCommandEdges(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"a": "same", "b": "same", "nl": "same\n", "other": "different"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name     string
		args     []string
		stdin    string
		code     int
		out, err string
	}{
		{"same_stdin", []string{"-", "-"}, "unread", 0, "", ""},
		{"same_file_ed", []string{"-e", "a", "a"}, "", 0, "", ""},
		{"identical_unterminated_ed", []string{"-e", "a", "b"}, "", 2, "", "No newline at end of file\n"},
		{"brief_unterminated_ed", []string{"-qe", "a", "other"}, "", 2, "", "No newline at end of file\n"},
		{"no_newline", []string{"a", "nl"}, "", 1, "1c1\n< same\n\\ No newline at end of file\n---\n> same\n", ""},
		{"stdin_unified_zero", []string{"-a", "-U", "0", "-", "nl"}, "same\n", 0, "", ""},
		{"missing_both", []string{"absent", "absent"}, "", 2, "", "diff: absent: No such file or directory\ndiff: absent: No such file or directory\n"},
		{"bad_option", []string{"--foobar"}, "", 2, "", "unrecognized option '--foobar'\n"},
		{"missing_operand", nil, "", 2, "", "Usage: diff <from> <to>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			code := diffRun(&Context{Dir: dir, CmdName: "diff", Args: tc.args, Stdin: strings.NewReader(tc.stdin), Stdout: &out, Stderr: &errout})
			if code != tc.code || out.String() != tc.out || errout.String() != tc.err {
				t.Fatalf("%d %q %q", code, out.String(), errout.String())
			}
		})
	}
}

func TestDiffBriefLarge(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"left": strings.Repeat("left\n", 10000), "right": strings.Repeat("right\n", 10000)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := diffRun(&Context{Dir: dir, Args: []string{"-q", "left", "right"}, Stdout: &out, Stderr: &errout})
	if code != 1 || out.String() != "Files left and right differ\n" || errout.Len() != 0 {
		t.Fatalf("%d %q %q", code, out.String(), errout.String())
	}
}

func TestDiffSideExitStatus(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"a": "a\n", "b": "b\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := diffRun(&Context{Dir: dir, Args: []string{"-y", "a", "b"}, Stdout: &out, Stderr: &errout})
	// uutils: its side formatter returns no difference sentinel; Misa reports exit 1.
	if code != 1 || out.Len() == 0 || errout.Len() != 0 {
		t.Fatalf("%d %q %q", code, out.String(), errout.String())
	}
}
