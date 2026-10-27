// Behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffNormalBasic(t *testing.T) {
	a, b := diffLines([]byte("a\n")), diffLines([]byte("b\n"))
	if got := diffNormal(a, b, diffMyers(a, b, diffOptions{}), diffOptions{}); got != "1c1\n< a\n---\n> b\n" {
		t.Fatal(got)
	}
}

func TestDiffNormalRanges(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"", "x\n", "0a1\n> x\n"}, {"x\n", "", "1d0\n< x\n"},
		{"x", "x\n", "1c1\n< x\n\\ No newline at end of file\n---\n> x\n"},
	} {
		a, b := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
		if got := diffNormal(a, b, diffMyers(a, b, diffOptions{}), diffOptions{}); got != tc.want {
			t.Fatalf("%q != %q", got, tc.want)
		}
	}
}

func diffPatchPermutations(t *testing.T, style, variant string) {
	t.Helper()
	patch, err := exec.LookPath("patch")
	if err != nil {
		t.Skip("patch unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	count := 729
	if variant == "missing_line_ending" {
		count *= 3
	}
	if style == "unified" && variant == "empty_lines" {
		count *= 4
	}
	for n := 0; n < count; n++ {
		v := n % 729
		var left, right strings.Builder
		for i := 0; i < 6; i++ {
			x := v % 3
			v /= 3
			c := byte('a' + 2*i)
			target := c + 1
			if x != 0 {
				c++
			}
			switch variant {
			case "empty_lines":
				if x == 0 {
					left.WriteByte('\n')
				} else {
					fmt.Fprintf(&left, "%c\n", c)
				}
			case "missing_lines":
				if x == 0 {
					fmt.Fprintf(&left, "%c\n", c)
				}
			case "reverse":
				target = byte('a' + i)
				c = target
				if x != 0 {
					c = byte('f' - i)
				}
				fmt.Fprintf(&left, "%c\n", c)
			default:
				fmt.Fprintf(&left, "%c\n", c)
			}
			if x != 2 {
				fmt.Fprintf(&right, "%c\n", target)
			}
		}
		a, b := left.String(), right.String()
		if variant == "missing_line_ending" || style == "unified" && variant == "empty_lines" {
			g := n / 729
			if g == 0 || g == 2 {
				a = strings.TrimSuffix(a, "\n")
			}
			if g == 1 || g == 2 {
				b = strings.TrimSuffix(b, "\n")
			}
		}
		al, bl := diffLines([]byte(a)), diffLines([]byte(b))
		o := diffOptions{context: 2}
		edits := diffMyers(al, bl, o)
		var delta string
		if style == "normal" {
			delta = diffNormal(al, bl, edits, o)
		} else {
			delta = diffUnified(al, bl, edits, o, [2]string{"source", "target"})
		}
		if a == b {
			if delta != "" {
				t.Fatal("same input differs")
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

func TestDiffNormalPermutations(t *testing.T) { diffPatchPermutations(t, "normal", "") }
func TestDiffNormalMissingLineEnding(t *testing.T) {
	diffPatchPermutations(t, "normal", "missing_line_ending")
}
func TestDiffNormalEmptyLines(t *testing.T) { diffPatchPermutations(t, "normal", "empty_lines") }
func TestDiffNormalReverse(t *testing.T)    { diffPatchPermutations(t, "normal", "reverse") }
func TestDiffNormalStopEarly(t *testing.T) {
	a, b := diffLines([]byte("a\nb\nc")), diffLines([]byte("a\nd\nc"))
	if got := diffNormal(a, b, diffMyers(a, b, diffOptions{}), diffOptions{}); got != "2c2\n< b\n---\n> d\n" {
		t.Fatal(got)
	}
	if got := diffNormal(a, a, diffMyers(a, a, diffOptions{}), diffOptions{}); got != "" {
		t.Fatal(got)
	}
	// uutils: brief reporting belongs to diffRun, not formatter sentinels.
}
