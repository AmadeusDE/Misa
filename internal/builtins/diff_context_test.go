// Behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffContextStopEarly(t *testing.T) {
	a, b := diffLines([]byte("a\nb\nc\n")), diffLines([]byte("a\nd\nc\n"))
	o := diffOptions{context: 3}
	got := diffContext(a, b, diffMyers(a, b, o), o, [2]string{"foo\tTIMESTAMP", "bar\tTIMESTAMP"})
	want := "*** foo\tTIMESTAMP\n--- bar\tTIMESTAMP\n" +
		"***************\n*** 1,3 ****\n  a\n! b\n  c\n--- 1,3 ----\n  a\n! d\n  c\n"
	if got != want {
		t.Fatalf("%q != %q", got, want)
	}
	if got := diffContext(a, a, diffMyers(a, a, o), o, [2]string{"foo", "bar"}); got != "" {
		t.Fatal(got)
	}
}

func TestDiffContextSuppress(t *testing.T) {
	for _, tc := range []struct {
		a, b, want string
		ctx        int
	}{
		{"x\nd\ny\nz\n", "x\ny\nz\n",
			"***************\n*** 1,4 ****\n  x\n- d\n  y\n  z\n--- 1,3 ----\n", 2},
		{"x\ny\nz\n", "x\ni\ny\nz\n",
			"***************\n*** 1,3 ****\n--- 1,4 ----\n  x\n+ i\n  y\n  z\n", 2},
		{"x\nd\ny\nz\n", "x\ny\nz\n",
			"***************\n*** 2 ****\n- d\n--- 1 ----\n", 0},
		{"d\ny\n", "y\n",
			"***************\n*** 1 ****\n- d\n--- 0 ----\n", 0},
		{"x\ny\nz\n", "n\nx\ny\nz\n",
			"***************\n*** 0 ****\n--- 1 ----\n+ n\n", 0},
	} {
		al, bl := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
		o := diffOptions{context: tc.ctx}
		got := diffContext(al, bl, diffMyers(al, bl, o), o, [2]string{"a", "b"})
		want := "*** a\n--- b\n" + tc.want
		if got != want {
			t.Fatalf("%q %q: %q != %q", tc.a, tc.b, got, want)
		}
	}
}

func TestDiffContextMissingNewline(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"a\nb\nc\n", "a\nb\nc",
			"***************\n*** 1,3 ****\n  a\n  b\n! c\n--- 1,3 ----\n  a\n  b\n! c\n" +
				"\\ No newline at end of file\n"},
		{"x\nc", "x\nd\n",
			"***************\n*** 1,2 ****\n  x\n! c\n\\ No newline at end of file\n" +
				"--- 1,2 ----\n  x\n! d\n"},
	} {
		al, bl := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
		o := diffOptions{context: 3}
		got := diffContext(al, bl, diffMyers(al, bl, o), o, [2]string{"a", "b"})
		want := "*** a\n--- b\n" + tc.want
		if got != want {
			t.Fatalf("%q %q: %q != %q", tc.a, tc.b, got, want)
		}
	}
}

func TestDiffContextHunkMerge(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"a\n1\n2\n3\n4\n5\n6\nb\n", "A\n1\n2\n3\n4\n5\n6\nB\n",
			"***************\n*** 1,8 ****\n! a\n  1\n  2\n  3\n  4\n  5\n  6\n! b\n" +
				"--- 1,8 ----\n! A\n  1\n  2\n  3\n  4\n  5\n  6\n! B\n"},
		{"a\n1\n2\n3\n4\n5\n6\n7\nb\n", "A\n1\n2\n3\n4\n5\n6\n7\nB\n",
			"***************\n*** 1,4 ****\n! a\n  1\n  2\n  3\n--- 1,4 ----\n! A\n  1\n  2\n  3\n" +
				"***************\n*** 6,9 ****\n  5\n  6\n  7\n! b\n--- 6,9 ----\n  5\n  6\n  7\n! B\n"},
	} {
		al, bl := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
		o := diffOptions{context: 3}
		got := diffContext(al, bl, diffMyers(al, bl, o), o, [2]string{"a", "b"})
		want := "*** a\n--- b\n" + tc.want
		if got != want {
			t.Fatalf("%q %q: %q != %q", tc.a, tc.b, got, want)
		}
	}
}

func diffPermutationCase(n int, variant string) (string, string) {
	v := n
	var left, right strings.Builder
	for i := 0; i < 6; i++ {
		x := v % 3
		v /= 3
		l, r := string(rune('b'+2*i)), string(rune('b'+2*i))
		if x == 0 {
			l = string(rune('a' + 2*i))
		}
		if x == 2 {
			r = ""
		}
		if variant == "empty_lines" && x == 0 {
			l = ""
		}
		if variant != "missing_lines" || x == 0 {
			left.WriteString(l + "\n")
		}
		if r != "" {
			right.WriteString(r + "\n")
		}
	}
	a, b := left.String(), right.String()
	if variant == "reverse" {
		a, b = b, a
	}
	if variant == "missing_line_ending" {
		if n%3 != 0 {
			a = strings.TrimSuffix(a, "\n")
		}
		if n%3 != 1 {
			b = strings.TrimSuffix(b, "\n")
		}
	}
	return a, b
}

func diffContextRoundTrip(t *testing.T, variant string) {
	t.Helper()
	patch, err := exec.LookPath("patch")
	if err != nil {
		t.Skip("patch unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	for n := 0; n < 729; n++ {
		a, b := diffPermutationCase(n, variant)
		al, bl := diffLines([]byte(a)), diffLines([]byte(b))
		o := diffOptions{context: 3}
		delta := diffContext(al, bl, diffMyers(al, bl, o), o, [2]string{"source", "target"})
		if a == b {
			if delta != "" {
				t.Fatalf("case %d: same input differs: %q", n, delta)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(a), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(patch, "--batch", "--silent", path)
		cmd.Stdin = strings.NewReader(delta)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("case %d: %v %s\n%s", n, err, out, delta)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != b {
			t.Fatalf("case %d: %q != %q\n%s", n, got, b, delta)
		}
	}
}

func TestDiffContextPermutations(t *testing.T) {
	for _, variant := range []string{"", "empty_lines", "missing_lines", "reverse", "missing_line_ending"} {
		name := variant
		if name == "" {
			name = "permutations"
		}
		t.Run(name, func(t *testing.T) { diffContextRoundTrip(t, variant) })
	}
}
