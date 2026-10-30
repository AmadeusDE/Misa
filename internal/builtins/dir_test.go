// Native behaviour tests for dir and vdir. They are ls output-mode aliases
// ("ls -C -b" and "ls -l -b"): the escape quoting and the column/long
// default come from uutils coreutils (MIT) src/uu/ls/src/config.rs
// (ProgramMode::Dir / ProgramMode::Vdir) at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663; the expectations below are
// native, using the uutils tests tests/by-util/test_dir.rs and
// test_vdir.rs as the reference for what the aliases must do.

package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirListsInColumnsWhenNotATerminal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "")
	writeFile(t, dir, "b.txt", "")
	writeFile(t, dir, "some-file1", "")

	r := runBuiltinIn(t, dir, "dir")
	assertExitCode(t, r, 0)
	// One line, columns separated by two spaces, unlike plain piped ls.
	assertStdout(t, r, "a.txt  b.txt  some-file1\n")
}

func TestDirIsNotLongFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "some-file1", "")

	r := runBuiltinIn(t, dir, "dir")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "some-file1")
	if strings.Contains(r.stdout, "rw-") {
		t.Errorf("dir printed a long listing: %q", r.stdout)
	}

	long := runBuiltinIn(t, dir, "dir", "-l")
	assertExitCode(t, long, 0)
	assertStdoutContains(t, long, "-rw-")
	assertStdoutContains(t, long, "some-file1")
}

func TestDirEscapesNongraphicNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a b", "")
	writeFile(t, dir, "tab\there", "")
	writeFile(t, dir, "back\\slash", "")

	r := runBuiltinIn(t, dir, "dir")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\\ b  back\\\\slash  tab\\there\n")
}

func TestDirAcceptsExplicitColumnAndQuoteOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a b", "")

	r := runBuiltinIn(t, dir, "dir", "--quoting-style=literal")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a b\n")
}

func TestVdirDefaultsToLongFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "some-file1", "")

	r := runBuiltinIn(t, dir, "vdir")
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, "total ") {
		t.Errorf("vdir did not print a long listing: %q", r.stdout)
	}
	assertStdoutContains(t, r, "-rw-")
	assertStdoutContains(t, r, "some-file1")
}

func TestVdirColumnOptionLeavesLongFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "some-file1", "")

	r := runBuiltinIn(t, dir, "vdir", "-C")
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, "rw-") {
		t.Errorf("vdir -C printed a long listing: %q", r.stdout)
	}
	assertStdoutContains(t, r, "some-file1")
}

func TestVdirEscapesNongraphicNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a b", "")

	r := runBuiltinIn(t, dir, "vdir")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "a\\ b\n")
	if strings.Contains(r.stdout, "a b\n") {
		t.Errorf("vdir did not escape the name: %q", r.stdout)
	}
}

func TestDirVdirProgramNameInDiagnostics(t *testing.T) {
	for _, name := range []string{"dir", "vdir"} {
		r := runBuiltin(t, name, "-/")
		assertExitCode(t, r, 2)
		assertStderrContains(t, r, name+": unknown option")

		missing := runBuiltin(t, name, "no-such-file")
		assertExitCode(t, missing, 1)
		assertStderrContains(t, missing, name+": no-such-file: No such file or directory")
	}
}

func TestDirVdirHelpNamesTheCommand(t *testing.T) {
	for _, name := range []string{"dir", "vdir"} {
		cmd, ok := Lookup(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		var stdout, stderr strings.Builder
		code := cmd.Run(&Context{Dir: t.TempDir(), CmdName: name, Args: []string{"--help"},
			Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")})
		if code != 0 {
			t.Fatalf("%s --help exit = %d", name, code)
		}
		if !strings.Contains(stdout.String(), "Usage: "+name+" [OPTION]") {
			t.Errorf("%s --help = %q", name, stdout.String())
		}
		if strings.Contains(stdout.String(), "ls [OPTION]") {
			t.Errorf("%s --help mentions ls: %q", name, stdout.String())
		}
	}
}

// TestDirVdirListDirectoriesKeepsAliasDefaults makes sure the mode survives
// the directory-target path (which prints a header per directory).
func TestDirVdirListDirectoriesKeepsAliasDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.Join("sub", "x y"), "")

	r := runBuiltinIn(t, dir, "dir", "-R")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "x\\ y\n")
}
