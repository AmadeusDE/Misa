package builtins

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
}

func TestAwkArraysAndFunctions(t *testing.T) {
	r := runBuiltinStdin(t, "awk", "b a b c a b\n", `function show(w) { print w, c[w] }
{ for (i = 1; i <= NF; i++) c[$i]++ } END { show("a"); show("b"); print length(c) }`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a 2\nb 3\n3\n")
}

func TestAwkEnviron(t *testing.T) {
	cmd, _ := Lookup("awk")
	var out, errb strings.Builder
	code := cmd.Run(&Context{Dir: t.TempDir(), Args: []string{`BEGIN { print ENVIRON["MISA_AWK_T"] }`}, Stdout: &out, Stderr: &errb, Env: []string{"MISA_AWK_T=ctx"}})
	if code != 0 || out.String() != "ctx\n" {
		t.Fatalf("code %d out %q err %q", code, out.String(), errb.String())
	}
	t.Setenv("MISA_AWK_T", "proc")
	r := runBuiltin(t, "awk", `BEGIN { print ENVIRON["MISA_AWK_T"] }`)
	assertStdout(t, r, "proc\n")
}

func TestAwkRedirectFiles(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "awk", `BEGIN { print "hello" > "out.txt"; close("out.txt"); getline l < "out.txt"; print l }`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestAwkCommandsThroughSh(t *testing.T) {
	requireSh(t)
	r := runBuiltin(t, "awk", `BEGIN {
	"echo hi there" | getline line; print line
	print "b\na" | "sort"; close("sort")
	st = system("exit 3"); print st
	printf "x"; system("printf y"); print "z"
}`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hi there\na\nb\n3\nxyz\n")
}

func TestAwkPipeExitStatus(t *testing.T) {
	requireSh(t)
	r := runBuiltin(t, "awk", `BEGIN { "exit 4" | getline; print close("exit 4"); print system("kill -9 $$") }`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "4\n265\n")
}

func TestAwkCommandRunsInContextDir(t *testing.T) {
	requireSh(t)
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "awk", `BEGIN { "pwd" | getline d; print d }`)
	assertExitCode(t, r, 0)
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(r.stdout))
	if got != want {
		t.Fatalf("pwd = %q, want %q", r.stdout, dir)
	}
}
