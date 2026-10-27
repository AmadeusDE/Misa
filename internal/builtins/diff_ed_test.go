// Behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
package builtins

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffEdBasic(t *testing.T) {
	a, b := diffLines([]byte("a\n")), diffLines([]byte("b\n"))
	got, err := diffEd(a, b, diffMyers(a, b, diffOptions{}))
	if err != nil || got != "1c\nb\n.\n" {
		t.Fatalf("%q, %v", got, err)
	}
}

func TestDiffEdStopEarly(t *testing.T) {
	a, b := diffLines([]byte("a\nb\nc\n")), diffLines([]byte("a\nd\nc\n"))
	got, err := diffEd(a, b, diffMyers(a, b, diffOptions{}))
	if err != nil || got != "2c\nd\n.\n" {
		t.Fatalf("%q, %v", got, err)
	}
	if got, err := diffEd(a, a, diffMyers(a, a, diffOptions{})); err != nil || got != "" {
		t.Fatalf("%q, %v", got, err)
	}
}

func TestDiffEdMissingNewline(t *testing.T) {
	for _, tc := range [][2]string{{"a", "b\n"}, {"a\n", "b"}} {
		a, b := diffLines([]byte(tc[0])), diffLines([]byte(tc[1]))
		if got, err := diffEd(a, b, diffMyers(a, b, diffOptions{})); err == nil || got != "" {
			t.Fatalf("%q %q: %q, %v", tc[0], tc[1], got, err)
		} else if err.Error() != "No newline at end of file" {
			t.Fatal(err)
		}
	}
}

// Expectations match GNU diff 3.12 -e: hunks last-to-first, original numbers.
func TestDiffEdRanges(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"1\n2\n3\n", "x\ny\n2\n3\nz\n", "3a\nz\n.\n1c\nx\ny\n.\n"},
		{"1\n2\n3\n4\n", "1\n4\n", "2,3d\n"},
		{"1\n2\n3\n", "1\n3\n", "2d\n"},
		{"1\n2\n", "1\n2\n3\n4\n", "2a\n3\n4\n.\n"},
	} {
		al, bl := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
		got, err := diffEd(al, bl, diffMyers(al, bl, diffOptions{}))
		if err != nil || got != tc.want {
			t.Fatalf("%q %q: %q != %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestDiffEdDotLine(t *testing.T) {
	for _, tc := range []struct{ a, b, want string }{
		{"q\n", ".\n", "1c\n..\n.\ns/.//\n"},
		{"q\n", "x\n.\ny\n", "1c\nx\n..\n.\ns/.//\na\ny\n.\n"},
	} {
		al, bl := diffLines([]byte(tc.a)), diffLines([]byte(tc.b))
		got, err := diffEd(al, bl, diffMyers(al, bl, diffOptions{}))
		if err != nil || got != tc.want {
			t.Fatalf("%q %q: %q != %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func diffEdRoundTrip(t *testing.T, variant string) {
	t.Helper()
	ed, err := exec.LookPath("ed")
	if err != nil {
		t.Skip("ed unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	for n := 0; n < 729; n++ {
		a, b := diffPermutationCase(n, variant)
		al, bl := diffLines([]byte(a)), diffLines([]byte(b))
		script, err := diffEd(al, bl, diffMyers(al, bl, diffOptions{}))
		if err != nil {
			t.Fatalf("case %d: %v", n, err)
		}
		if a == b {
			if script != "" {
				t.Fatalf("case %d: same input differs: %q", n, script)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(a), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(ed, path)
		cmd.Stdin = strings.NewReader(script + "w " + path + "\n")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("case %d: %v %s\n%s", n, err, out, script)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != b {
			t.Fatalf("case %d: %q != %q\n%s", n, got, b, script)
		}
	}
}

func TestDiffEdPermutations(t *testing.T) {
	for _, variant := range []string{"", "empty_lines", "missing_lines", "reverse"} {
		name := variant
		if name == "" {
			name = "permutations"
		}
		t.Run(name, func(t *testing.T) { diffEdRoundTrip(t, variant) })
	}
}
