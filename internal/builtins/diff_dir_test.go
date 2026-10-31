package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffDirectories(t *testing.T) {
	d := t.TempDir()
	for _, name := range []string{"left/sub", "right/sub"} {
		if err := os.MkdirAll(filepath.Join(d, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{"left/sub/f": "old\n", "right/sub/f": "new\n", "left/only": "x"} {
		if err := os.WriteFile(filepath.Join(d, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := diffRun(&Context{Dir: d, Args: []string{"-r", "left", "right"}, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errout})
	want := "Only in left: only\ndiff -r " + filepath.FromSlash("left/sub/f right/sub/f") + "\n1c1\n< old\n---\n> new\n"
	if code != 1 || out.String() != want || errout.Len() != 0 {
		t.Fatalf("%d %q %q", code, out.String(), errout.String())
	}
}

func TestDiffOriginalLines(t *testing.T) {
	a, b := diffLines([]byte("A  line\noriginal\n")), diffLines([]byte("a line\nnew\n"))
	o := diffOptions{ignoreCase: true, ignoreSpace: true}
	got := diffNormal(a, b, diffMyers(a, b, o), o)
	if got != "2c2\n< original\n---\n> new\n" {
		t.Fatal(got)
	}
}
