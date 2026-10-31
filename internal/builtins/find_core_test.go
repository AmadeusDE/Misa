// Tests informed by Toybox find.test, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb.
package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findTestRun(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	var out, err bytes.Buffer
	c := &Context{Dir: dir, Args: args, Stdout: &out, Stderr: &err}
	code := runFind(c)
	return out.String(), err.String(), code
}
func TestFindExpressionCore(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-true", "-o", "-false", "-a", "-print"}, ""},
		{[]string{"-false", "-a", "-print", "-o", "-print"}, ".\n"},
		{[]string{"!", "(", "-false", "-o", "-false", ")"}, ".\n"},
		{[]string{"-print", ",", "-print"}, ".\n.\n"},
		{[]string{"-false", "-a", "-print"}, ""},
		{[]string{"-prune"}, ".\n"},
		{[]string{"-quit"}, ""},
	}
	for _, tc := range cases {
		out, err, code := findTestRun(t, dir, tc.args...)
		if code != 0 || err != "" || out != tc.want {
			t.Errorf("%v: %q %q %d", tc.args, out, err, code)
		}
	}
}
func TestFindParserErrors(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"("}, {")"}, {"-true", "-o"}, {"-a", "-true"}, {"-true", ")"}, {"-maxdepth"}, {"-mindepth", "-1"}, {"-maxdepth", "x"}, {"-printf"}, {"-exec"}, {"-regextype", "posix"}, {"-unknown"}} {
		out, err, code := findTestRun(t, dir, args...)
		if code == 0 || err == "" || out != "" {
			t.Errorf("%v: %q %q %d", args, out, err, code)
		}
	}
}
func TestFindWalkCore(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "a"), 0755)
	os.WriteFile(filepath.Join(dir, "a", "f"), nil, 0644)
	os.Symlink("a", filepath.Join(dir, "link"))
	os.Symlink("missing", filepath.Join(dir, "dangling"))
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-maxdepth", "1"}, ".\n./a\n./dangling\n./link\n"},
		{[]string{"-mindepth", "1", "-maxdepth", "1"}, "./a\n./dangling\n./link\n"},
		{[]string{"-depth"}, "./a/f\n./a\n./dangling\n./link\n.\n"},
		{[]string{"-name", "a", "-prune", "-o", "-print"}, ".\n./dangling\n./link\n"},
		{[]string{"-print", "-quit"}, ".\n"},
		{[]string{"-L", "link"}, "link\nlink/f\n"},
		{[]string{"-H", "link"}, "link\nlink/f\n"},
		{[]string{"-P", "link"}, "link\n"},
		{[]string{"-L", "dangling"}, "dangling\n"},
	}
	for _, tc := range cases {
		out, err, code := findTestRun(t, dir, tc.args...)
		if code != 0 || err != "" || out != tc.want {
			t.Errorf("%v: %q %q %d want %q", tc.args, out, err, code, tc.want)
		}
	}
	out, err, code := findTestRun(t, dir, "missing", "a")
	if code != 1 || !strings.Contains(err, "missing") || out != "a\na/f\n" {
		t.Errorf("errors: %q %q %d", out, err, code)
	}
}

func TestFindLoopAndOutputFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..", filepath.Join(dir, "a", "loop")); err != nil {
		t.Fatal(err)
	}
	out, err, code := findTestRun(t, dir, "-L")
	if code != 1 || !strings.Contains(err, "loop") || !strings.Contains(out, "./a\n") {
		t.Fatalf("loop: %q %q %d", out, err, code)
	}
	var stderr bytes.Buffer
	c := &Context{Dir: dir, Args: []string{"-maxdepth", "0"}, Stdout: findBadWriter{}, Stderr: &stderr}
	if runFind(c) != 1 || stderr.Len() == 0 {
		t.Fatalf("write error: %q", stderr.String())
	}
}

type findBadWriter struct{}

func (findBadWriter) Write([]byte) (int, error) { return 0, os.ErrPermission }

// These parse and evaluate a synthetic entry directly, without any filesystem walk.
func TestFindParserSyntheticExpressions(t *testing.T) {
	cases := []struct {
		args   []string
		result bool
		output string
	}{
		{[]string{"!", "-false", "-o", "-true", "-false"}, true, ""},
		{[]string{"!", "-true", "-o", "-true", "-false"}, false, ""},
		{[]string{"-not", "-false", "-and", "-true", "-or", "-false"}, true, ""},
		{[]string{"(", "-true", "-o", "-true", ")", "-false"}, false, ""},
		{[]string{"-true", "-o", "-print"}, true, ""},
		{[]string{"-false", "-a", "-print"}, false, ""},
		{[]string{"-false", ",", "-print"}, true, "synthetic\n"},
		{[]string{"-print", ",", "-false"}, false, "synthetic\n"},
		{[]string{"-print0"}, true, "synthetic\x00"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out bytes.Buffer
			c := &Context{Stdout: &out}
			p := findParser{ctx: c, args: tc.args}
			n, err := p.parse()
			if err != nil {
				t.Fatal(err)
			}
			e := &findEntry{ctx: c, path: "synthetic"}
			result := n(e)
			if result != tc.result || out.String() != tc.output || e.err != nil {
				t.Fatalf("result=%v output=%q error=%v", result, out.String(), e.err)
			}
		})
	}
}

func TestFindParserSyntheticErrors(t *testing.T) {
	for _, args := range [][]string{{"("}, {"(", "-true"}, {"(", ")"}, {"-true", ")"}, {"-true", "-or"}, {"-true", "-and"}, {"!"}, {"-true", ","}, {"-name"}, {"-printf"}, {"-maxdepth"}, {"-maxdepth", "-1"}, {"-exec"}, {"-unknown"}} {
		p := findParser{ctx: &Context{}, args: args}
		if _, err := p.parse(); err == nil {
			t.Errorf("%q: expected parse error", args)
		}
	}
}

func TestFindNodeShortCircuit(t *testing.T) {
	for _, tc := range []struct {
		name              string
		combine           func(findNode, findNode) findNode
		left, right, want bool
		calls             int
	}{
		{"and false", findAnd, false, true, false, 0}, {"and true", findAnd, true, false, false, 1},
		{"or true", findOr, true, false, true, 0}, {"or false", findOr, false, true, true, 1},
		{"comma false", findComma, false, true, true, 1}, {"comma true", findComma, true, false, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			left := func(*findEntry) bool { return tc.left }
			right := func(*findEntry) bool { calls++; return tc.right }
			if got := tc.combine(left, right)(&findEntry{}); got != tc.want || calls != tc.calls {
				t.Fatalf("got %v, calls %d", got, calls)
			}
		})
	}
	calls := 0
	n := findNot(func(*findEntry) bool { calls++; return false })
	if !n(&findEntry{}) || calls != 1 {
		t.Fatalf("not calls=%d", calls)
	}
}

func TestFindFollowPrefixOrderAndNestedLinks(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"tree", "target"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "target", "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"root": "tree", "tree/nested": "../target"} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skip(err)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-H", "root"}, "root\nroot/nested\n"},
		{[]string{"-L", "root"}, "root\nroot/nested\nroot/nested/file\n"},
		{[]string{"-L", "-H", "root"}, "root\nroot/nested\n"},
		{[]string{"-H", "-L", "root"}, "root\nroot/nested\nroot/nested/file\n"},
		{[]string{"-L", "-P", "root"}, "root\n"},
	} {
		out, err, code := findTestRun(t, dir, tc.args...)
		if code != 0 || err != "" || out != tc.want {
			t.Errorf("%v: %q %q %d", tc.args, out, err, code)
		}
	}
}

func TestFindPermissionErrorContinues(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "a-blocked")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0700) })
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("permissions do not restrict this process")
	}
	if err := os.WriteFile(filepath.Join(dir, "z-good"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	out, err, code := findTestRun(t, dir)
	if code != 1 || !strings.Contains(err, "a-blocked") || !strings.Contains(out, "./z-good\n") {
		t.Fatalf("%q %q %d", out, err, code)
	}
}

func TestFindCancellation(t *testing.T) {
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, stderr bytes.Buffer
	c := &Context{Dir: t.TempDir(), Stdout: &out, Stderr: &stderr, Ctx: cctx}
	if code := runFind(c); code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatalf("%q %q %d", out.String(), stderr.String(), code)
	}
}

func TestFindEmptyRootContinues(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dir", "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := findTestRun(t, dir, "", "dir", "-name", "file")
	if code != 1 || stderr == "" || out != "dir/file\n" {
		t.Fatalf("output=%q stderr=%q code=%d", out, stderr, code)
	}
}
