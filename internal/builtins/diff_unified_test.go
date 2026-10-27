// Behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
package builtins

import "testing"

func TestDiffUnifiedPermutations(t *testing.T) { diffPatchPermutations(t, "unified", "") }
func TestDiffUnifiedMissingLineEnding(t *testing.T) {
	diffPatchPermutations(t, "unified", "missing_line_ending")
}
func TestDiffUnifiedEmptyLines(t *testing.T)   { diffPatchPermutations(t, "unified", "empty_lines") }
func TestDiffUnifiedMissingLines(t *testing.T) { diffPatchPermutations(t, "unified", "missing_lines") }
func TestDiffUnifiedReverse(t *testing.T)      { diffPatchPermutations(t, "unified", "reverse") }
func TestDiffUnifiedStopEarly(t *testing.T) {
	a, b := diffLines([]byte("a\nb\nc\n")), diffLines([]byte("a\nd\nc\n"))
	o := diffOptions{context: 3}
	headers := [2]string{"foo\tTIMESTAMP", "bar\tTIMESTAMP"}
	want := "--- foo\tTIMESTAMP\n+++ bar\tTIMESTAMP\n@@ -1,3 +1,3 @@\n a\n-b\n+d\n c\n"
	if got := diffUnified(a, b, diffMyers(a, b, o), o, headers); got != want {
		t.Fatal(got)
	}
	if got := diffUnified(a, a, diffMyers(a, a, o), o, headers); got != "" {
		t.Fatal(got)
	}
	// uutils: brief reporting belongs to diffRun, not formatter sentinels.
}

func TestDiffUnifiedContextEdges(t *testing.T) {
	for _, tc := range []struct {
		name, a, b, want string
		context          int
	}{
		{"delete_zero", "hello\nworld\n", "world\n", "--- a\n+++ b\n@@ -1 +0,0 @@\n-hello\n", 0},
		{"insert_zero", "world\n", "hello\nworld\n", "--- a\n+++ b\n@@ -0,0 +1 @@\n+hello\n", 0},
		{"separate", "a\nsame\nb\n", "A\nsame\nB\n", "--- a\n+++ b\n@@ -1 +1 @@\n-a\n+A\n@@ -3 +3 @@\n-b\n+B\n", 0},
		{"max_context", "a\nsame\n", "A\nsame\n", "--- a\n+++ b\n@@ -1,2 +1,2 @@\n-a\n+A\n same\n", int(^uint(0) >> 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
			o := diffOptions{context: tc.context}
			if got := diffUnified(a, b, diffMyers(a, b, o), o, [2]string{"a", "b"}); got != tc.want {
				t.Fatalf("%q != %q", got, tc.want)
			}
		})
	}
}
