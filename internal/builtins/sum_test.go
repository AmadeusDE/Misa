// Behaviour adapted from uutils coreutils sum (MIT).
package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSumFixtures(t *testing.T) {
	fixtures := "../../tests/uutils/testdata/tests/fixtures/sum"
	for _, algo := range []string{"bsd", "sysv"} {
		for _, kind := range []string{"single_file", "multiple_files", "stdin"} {
			t.Run(algo+"_"+kind, func(t *testing.T) {
				args := []string{}
				if algo == "sysv" {
					args = append(args, "-s")
				}
				input := ""
				if kind == "stdin" {
					data, e := os.ReadFile(filepath.Join(fixtures, "lorem_ipsum.txt"))
					if e != nil {
						t.Fatal(e)
					}
					input = string(data)
				} else {
					args = append(args, "lorem_ipsum.txt")
					if kind == "multiple_files" {
						args = append(args, "alice_in_wonderland.txt")
					}
				}
				expected, e := os.ReadFile(filepath.Join(fixtures, algo+"_"+kind+".expected"))
				if e != nil {
					t.Fatal(e)
				}
				r := runBuiltinStdinIn(t, fixtures, "sum", input, args...)
				assertExitCode(t, r, 0)
				assertStdout(t, r, string(expected))
			})
		}
	}
}

func TestSum(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		args        []string
		want        string
	}{
		{"empty", "", nil, "00000     0\n"},
		{"bsd", "hello\n", nil, "36979     1\n"},
		{"sysv", "hello\n", []string{"--sysv"}, "542 1\n"},
		{"bsd_blocks", strings.Repeat("\x00", 1025), nil, "00000     2\n"},
		{"sysv_blocks", strings.Repeat("\x00", 513), []string{"-s"}, "0 2\n"},
		{"stdin_twice", "hello\n", []string{"-", "-"}, "36979     1 -\n00000     0 -\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "sum", tc.input, tc.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
		})
	}
	dir := t.TempDir()
	writeFile(t, dir, "f", "hello\n")
	if e := os.Mkdir(filepath.Join(dir, "d"), 0755); e != nil {
		t.Fatal(e)
	}
	r := runBuiltinIn(t, dir, "sum", "d", "missing", "f")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "36979     1 f\n")
	assertStderrContains(t, r, "sum: d: Is a directory\n")
	assertStderrContains(t, r, "sum: missing: No such file or directory\n")
}
