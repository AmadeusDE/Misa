// Behaviour adapted from toybox shuf.c (0BSD) and uutils coreutils shuf (MIT).
package builtins

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func shufTestRun(t *testing.T, args []string, input string, files map[string]string) (int, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	cmd, ok := Lookup("shuf")
	if !ok {
		t.Fatal("shuf not registered")
	}
	code := cmd.Run(&Context{Dir: dir, Args: args, Stdin: strings.NewReader(input), Stdout: &out, Stderr: &errOut, Env: []string{}})
	return code, out.String(), errOut.String(), dir
}

func TestShufSeed(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		input, want string
	}{
		{"echo repeat", []string{"--random-seed=🌱", "-ern10", "foo", "bar", "baz", "qux"}, "", "baz\nqux\nbaz\nbar\nfoo\nqux\nfoo\nbar\nbar\nqux\n"},
		{"echo", []string{"--random-seed=🌱", "-e", "foo", "bar", "baz", "qux"}, "", "baz\nqux\nbar\nfoo\n"},
		{"range repeat", []string{"--random-seed=🦀", "-ri1-99", "-n10"}, "", "29\n13\n77\n29\n26\n84\n64\n29\n3\n37\n"},
		{"range", []string{"--random-seed=12345", "-i1-10"}, "", "2\n8\n1\n10\n4\n9\n7\n5\n3\n6\n"},
		{"stdin", []string{"--random-seed=12345"}, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n", "2\n8\n1\n10\n4\n9\n7\n5\n3\n6\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, stderr, _ := shufTestRun(t, tc.args, tc.input, nil)
			if code != 0 || stderr != "" || out != tc.want {
				t.Fatalf("code=%d stderr=%q stdout=%q want=%q", code, stderr, out, tc.want)
			}
		})
	}
	want := "21\n25\n44\n82\n70\n6\n75\n91\n45\n95\n55\n13\n26\n49\n31\n67\n97\n23\n7\n29\n33\n76\n89\n72\n62\n99\n84\n27\n86\n98\n2\n11\n32\n3\n96\n81\n34\n57\n83\n28\n50\n19\n63\n30\n38\n92\n15\n10\n35\n43\n"
	var input strings.Builder
	for i := 1; i <= 100; i++ {
		input.WriteString(strconv.Itoa(i) + "\n")
	}
	for _, args := range [][]string{{"--random-seed=67890", "-i1-100", "-n50"}, {"--random-seed=67890", "-n50"}} {
		code, out, stderr, _ := shufTestRun(t, args, input.String(), nil)
		if code != 0 || stderr != "" || out != want {
			t.Fatalf("code=%d stderr=%q stdout=%q", code, stderr, out)
		}
	}
}

func TestShufGNUCompatibility(t *testing.T) {
	random := "\xd1\xfd\xb9\x9a\xf5\x81\x71\x42\xf9\x7a\x59\x79\xd4\x9c\x8c\x7d"
	for _, tc := range []struct {
		name                        string
		args                        []string
		input, random, want, stderr string
		code                        int
	}{
		{"echo", []string{"-e", "1", "2", "3", "4", "5", "6", "7"}, "", random, "7\n1\n2\n5\n3\n4\n6\n", "", 0},
		{"stdin", nil, "1\n2\n3\n4\n5\n6\n7\n", random, "7\n1\n2\n5\n3\n4\n6\n", "", 0},
		{"file", []string{"input"}, "", random, "7\n1\n2\n5\n3\n4\n6\n", "", 0},
		{"limited file", []string{"input", "-n5"}, "", random, "7\n1\n2\n5\n3\n", "", 0},
		{"range", []string{"-i1-10"}, "", random, "10\n2\n8\n7\n3\n9\n6\n5\n1\n4\n", "", 0},
		{"repeat exhausted", []string{"-ri1-99"}, "", "\xfb\x83\x8f\x21\x9b\x3c\x2d\xc5\x73\xa5\x58\x6c\x54\x2f\x59\xf8", "38\n30\n10\n26\n23\n61\n46\n99\n75\n43\n10\n89\n10\n44\n24\n59\n22\n51\n", "shuf: end of random source\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--random-source=random"}, tc.args...)
			code, out, stderr, _ := shufTestRun(t, args, tc.input, map[string]string{"random": tc.random, "input": "1\n2\n3\n4\n5\n6\n7\n"})
			if code != tc.code || out != tc.want || stderr != tc.stderr {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out, stderr)
			}
		})
	}
}

func TestShufPermutation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		input string
		want  []string
		sep   string
	}{
		{"stdin", nil, "a\nb\nc", []string{"a", "b", "c"}, "\n"},
		{"explicit stdin", []string{"-"}, "a\nb\nc\n", []string{"a", "b", "c"}, "\n"},
		{"echo postfix", []string{"a", "b", "c", "-e"}, "", []string{"a", "b", "c"}, "\n"},
		{"echo repeated", []string{"-e", "a", "b", "-e", "c"}, "", []string{"a", "b", "c"}, "\n"},
		{"nul echo", []string{"-ez", "a", "b", "c"}, "", []string{"a", "b", "c"}, "\x00"},
		{"nul input", []string{"-zz"}, "a\x00b\x00c\x00", []string{"a", "b", "c"}, "\x00"},
		{"empty", nil, "", nil, "\n"},
		{"blank", nil, "\n", []string{""}, "\n"},
		{"embedded newline", []string{"-e", "a\nb"}, "", []string{"a", "b"}, "\n"},
		{"raw argument", []string{"-e", "a\xffb"}, "", []string{"a\xffb"}, "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, stderr, _ := shufTestRun(t, tc.args, tc.input, nil)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			var got []string
			if out != "" {
				got = strings.Split(strings.TrimSuffix(out, tc.sep), tc.sep)
			}
			sort.Strings(got)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") || len(got) != len(tc.want) {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestShufCountsAndRanges(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		count  int
		lo, hi uint64
	}{
		{[]string{"-i1-100000000000", "-n256"}, 256, 1, 100000000000},
		{[]string{"-i1234567890-2147483647", "-n10"}, 10, 1234567890, 2147483647},
		{[]string{"-i0-18446744073709551615", "-n1"}, 1, 0, ^uint64(0)},
		{[]string{"-ri1-18446744073709551615", "-n1"}, 1, 1, ^uint64(0)},
		{[]string{"-i0-18446744073709551614", "-n1"}, 1, 0, ^uint64(0) - 1},
		{[]string{"-i1-10", "-n5", "-n6"}, 6, 1, 10},
		{[]string{"-i1-10", "-n6", "-n5"}, 5, 1, 10},
		{[]string{"-rri1-10", "-n15000"}, 15000, 1, 10},
		{[]string{"-i1-1", "-n4294967296"}, 1, 1, 1},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			code, out, stderr, _ := shufTestRun(t, tc.args, "", nil)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if len(lines) != tc.count {
				t.Fatalf("count=%d want=%d", len(lines), tc.count)
			}
			seen := map[uint64]bool{}
			repeat := strings.Contains(strings.Join(tc.args, ""), "-r")
			for _, s := range lines {
				v, err := strconv.ParseUint(s, 10, 64)
				if err != nil || v < tc.lo || v > tc.hi || (!repeat && seen[v]) {
					t.Fatalf("invalid draw %q", s)
				}
				seen[v] = true
			}
		})
	}
}

func TestShufOutputSafety(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"missing input", []string{"-o", "out", "missing"}, 1, "keep me\n"},
		{"missing random", []string{"-o", "out", "--random-source=missing", "input"}, 1, "keep me\n"},
		{"zero skips input and random", []string{"-n0", "-o", "out", "--random-source=missing", "missing"}, 0, ""},
		{"same input output", []string{"-o", "input", "input"}, 0, "hello\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr, dir := shufTestRun(t, tc.args, "", map[string]string{"out": "keep me\n", "input": "hello\n"})
			if code != tc.code {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			name := "out"
			if tc.name == "same input output" {
				name = "input"
			}
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil || string(data) != tc.want {
				t.Fatalf("output=%q err=%v", data, err)
			}
		})
	}
}

type shufTestBadIO struct{}

func (shufTestBadIO) Read([]byte) (int, error)  { return 0, errors.New("read failure") }
func (shufTestBadIO) Write([]byte) (int, error) { return 0, errors.New("write failure") }
func TestShufIOErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		in   io.Reader
		out  io.Writer
		want string
	}{
		{"read", nil, shufTestBadIO{}, io.Discard, "read failure"},
		{"write", []string{"-i1-10"}, strings.NewReader(""), shufTestBadIO{}, "write failed: write failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := runShuf(&Context{Dir: t.TempDir(), Args: tc.args, Stdin: tc.in, Stdout: tc.out, Stderr: &stderr})
			if code != 1 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("code=%d stderr=%q", code, stderr.String())
			}
		})
	}
}

func TestShufErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-i", "0"}, "invalid value '0' for '--input-range <LO-HI>': missing '-'"},
		{[]string{"-i", "a-9"}, "invalid value 'a-9' for '--input-range <LO-HI>': invalid digit found in string"},
		{[]string{"-i", "0-b"}, "invalid value '0-b' for '--input-range <LO-HI>': invalid digit found in string"},
		{[]string{"-i", "5-3"}, "invalid value '5-3' for '--input-range <LO-HI>': start exceeds end"},
		{[]string{"-n", "a"}, "invalid value 'a' for '--head-count <COUNT>': invalid digit found in string"},
		{[]string{"-n", "18446744073709551616"}, "invalid value '18446744073709551616' for '--head-count <COUNT>': number too large to fit in target type"},
		{[]string{"-i1-9", "-i1-9"}, "the argument '--input-range <LO-HI>' cannot be used multiple times"},
		{[]string{"-o", "a", "-o", "b"}, "the argument '--output <FILE>' cannot be used multiple times"},
		{[]string{"--random-seed=a", "--random-seed=b"}, "the argument '--random-seed <STRING>' cannot be used multiple times"},
		{[]string{"--random-source=a", "--random-source=b"}, "the argument '--random-source <FILE>' cannot be used multiple times"},
		{[]string{"-re"}, "no lines to repeat"},
		{[]string{"-ri4-3"}, "no lines to repeat"},
		{[]string{"-r"}, "no lines to repeat"},
		{[]string{"-i1-18446744073709551615"}, "memory exhausted"},
		{[]string{"-i1-18446744073709551615", "-n18446744073709551615"}, "memory exhausted"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			code, out, stderr, _ := shufTestRun(t, tc.args, "", nil)
			if code != 1 || out != "" || stderr != "shuf: "+tc.want+"\n" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out, stderr)
			}
		})
	}
}

func TestShufZeroCount(t *testing.T) {
	for _, args := range [][]string{{"-n0"}, {"-n0", "-"}, {"-n0", "missing"}, {"-n0", "-e", "hello"}, {"-n0", "-i4-8"}, {"-n0", "-o", "out"}} {
		code, out, stderr, dir := shufTestRun(t, args, "", nil)
		if code != 0 || out != "" || stderr != "" {
			t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out, stderr)
		}
		if args[len(args)-1] == "out" {
			data, err := os.ReadFile(filepath.Join(dir, "out"))
			if err != nil || len(data) != 0 {
				t.Fatalf("output=%q err=%v", data, err)
			}
		}
	}
}

func TestShufWriteFull(t *testing.T) {
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("env/host: /dev/full not writable: %v", err)
	}
	full.Close()
	code, out, stderr, _ := shufTestRun(t, []string{"-i1-10", "-o/dev/full"}, "", nil)
	if code != 1 || out != "" || stderr != "shuf: write failed: No space left on device\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out, stderr)
	}
}
