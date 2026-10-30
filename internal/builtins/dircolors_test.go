// Native behaviour tests for dircolors. Option set, output syntax, error
// words and exit statuses follow uutils coreutils (MIT)
// src/uu/dircolors/src/dircolors.rs at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663; several cases are native
// rewrites of tests/by-util/test_dircolors.rs from that revision.

package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runDircolors runs the builtin with an explicit environment, so neither
// the host SHELL nor TERM can leak into a case.
func runDircolorsEnv(t *testing.T, env []string, stdin string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("dircolors")
	if !ok {
		t.Fatal("dircolors not registered")
	}
	var stdout, stderr strings.Builder
	ctx := &Context{
		Dir:     t.TempDir(),
		Args:    args,
		CmdName: "dircolors",
		Width:   80,
		Height:  24,
		Env:     env,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Stdin:   strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestDircolorsShOutputDefault(t *testing.T) {
	r := runDircolorsEnv(t, []string{"SHELL=/bin/bash", "TERM=screen"}, "")
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, "LS_COLORS='rs=0:di=01;34:ln=01;36:") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if !strings.HasSuffix(r.stdout, "*.rpmsave=00;90:';\nexport LS_COLORS\n") {
		t.Errorf("stdout = %q", r.stdout)
	}
}

func TestDircolorsCshOutputDefault(t *testing.T) {
	r := runDircolorsEnv(t, []string{"SHELL=/bin/bash", "TERM=screen"}, "", "-c")
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, "setenv LS_COLORS 'rs=0:di=01;34:") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if !strings.HasSuffix(r.stdout, "*\n") && !strings.HasSuffix(r.stdout, "'\n") {
		t.Errorf("stdout = %q", r.stdout)
	}
}

func TestDircolorsShellTypeFromSHELL(t *testing.T) {
	csh := runDircolorsEnv(t, []string{"SHELL=/bin/tcsh"}, "")
	assertExitCode(t, csh, 0)
	if !strings.HasPrefix(csh.stdout, "setenv LS_COLORS '") {
		t.Errorf("tcsh SHELL: stdout = %q", csh.stdout)
	}

	sh := runDircolorsEnv(t, []string{"SHELL=/usr/bin/zsh"}, "")
	assertExitCode(t, sh, 0)
	if !strings.HasPrefix(sh.stdout, "LS_COLORS='") {
		t.Errorf("zsh SHELL: stdout = %q", sh.stdout)
	}
}

func TestDircolorsNoShell(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if r.stderr != "dircolors: no SHELL environment variable, and no shell type option given\n" {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func TestDircolorsLastShellFlagWins(t *testing.T) {
	bc := runDircolorsEnv(t, []string{}, "", "-bc")
	assertExitCode(t, bc, 0)
	if !strings.HasPrefix(bc.stdout, "setenv LS_COLORS '") {
		t.Errorf("-bc stdout = %q", bc.stdout)
	}

	cb := runDircolorsEnv(t, []string{}, "", "-cb")
	assertExitCode(t, cb, 0)
	if !strings.HasPrefix(cb.stdout, "LS_COLORS='") {
		t.Errorf("-cb stdout = %q", cb.stdout)
	}

	long := runDircolorsEnv(t, []string{"SHELL=/bin/bash"}, "", "--c-shell", "--bourne-shell")
	assertExitCode(t, long, 0)
	if !strings.HasPrefix(long.stdout, "LS_COLORS='") {
		t.Errorf("--c-shell --bourne-shell stdout = %q", long.stdout)
	}
}

func TestDircolorsPrintDatabase(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "", "-p")
	assertExitCode(t, r, 0)
	if r.stdout != dircolorsDatabase+"\n" {
		t.Errorf("stdout does not match the embedded database")
	}
	assertStdoutContains(t, r, "# Configuration file for dircolors")
	assertStdoutContains(t, r, "COLORTERM ?*\n")
	assertStdoutContains(t, r, "TERM screen*\n")
	assertStdoutContains(t, r, "DIR 01;34\n")

	long := runDircolorsEnv(t, []string{}, "", "--print-database")
	assertStdout(t, long, r.stdout)
}

func TestDircolorsPrintDatabaseWithOperandFails(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "", "-p", "file")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "extra operand 'file'")
}

func TestDircolorsExclusiveOptions(t *testing.T) {
	cases := [][]string{
		{"-bp"},
		{"-cp"},
		{"-b", "--print-ls-colors"},
		{"-c", "--print-ls-colors"},
		{"-p", "--print-ls-colors"},
	}
	for _, args := range cases {
		r := runDircolorsEnv(t, []string{"SHELL=/bin/bash"}, "", args...)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		if !strings.Contains(r.stderr, "mutually exclusive") {
			t.Errorf("%v stderr = %q", args, r.stderr)
		}
	}
}

func TestDircolorsRepeatedOptions(t *testing.T) {
	for _, arg := range []string{"-b", "-c", "--print-database", "--print-ls-colors"} {
		r := runDircolorsEnv(t, []string{"SHELL=/bin/bash"}, "", arg, arg)
		assertExitCode(t, r, 0)
		if r.stderr != "" {
			t.Errorf("%s %s stderr = %q", arg, arg, r.stderr)
		}
	}
}

func TestDircolorsStdinDatabase(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "owt 40;33\n", "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='tw=40;33:';\nexport LS_COLORS\n")

	csh := runDircolorsEnv(t, []string{}, "owt 40;33\n", "-c", "-")
	assertExitCode(t, csh, 0)
	assertStdout(t, csh, "setenv LS_COLORS 'tw=40;33:'\n")
}

func TestDircolorsQuotesUnsafeValues(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "exec 'echo Hello;:'\n", "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='ex='\\''echo Hello;\\:'\\'':';\nexport LS_COLORS\n")
}

func TestDircolorsKeywords(t *testing.T) {
	db := "NORMAL 00 # comment\n" +
		"FILE 00\n" +
		"RESET 0\n" +
		"Dir 01;34\n" +
		"LINK 01;36\n" +
		"LEFTCODE    \\e[\n" +
		"RIGHTCODE   m\n" +
		"ENDCODE \\xff\n" +
		"OPTIONS -F\n" +
		"COLOR never\n" +
		"EIGHTBIT 1\n"
	r := runDircolorsEnv(t, []string{}, db, "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='no=00:fi=00:rs=0:di=01;34:ln=01;36:lc=\\e[:rc=m:ec=\\xff:';\nexport LS_COLORS\n")
}

func TestDircolorsExtensionEntries(t *testing.T) {
	// A bare word that is not a keyword and does not look like an extension
	// is an error, not a silently ignored line.
	bad := runDircolorsEnv(t, []string{}, "ext 01;31\n", "-b", "-")
	assertExitCode(t, bad, 1)
	assertStdout(t, bad, "")
	assertStderrContains(t, bad, "unrecognized keyword ext")

	db := ".xspf 00;36\n*.tgz 01;31\n"
	r := runDircolorsEnv(t, []string{}, db, "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='*.xspf=00;36:*.tgz=01;31:';\nexport LS_COLORS\n")
}

func TestDircolorsTermMatching(t *testing.T) {
	match := "LS_COLORS='*.term_matching=00;38;5;61:';\nexport LS_COLORS\n"
	noMatch := "LS_COLORS='';\nexport LS_COLORS\n"

	cases := []struct{ pattern, term, want string }{
		{"matches", "matches", match},
		{"matches", "no_match", noMatch},
		{"[!a]_negation", "a_negation", noMatch},
		{"[!a]_negation", "b_negation", match},
		{"[^a]_negation", "a_negation", noMatch},
		{"[^a]_negation", "b_negation", match},
		{"con[0-9]*x[0-9]*", "con256x12", match},
		{"*color*", "truecolor", match},
		{"?", "a", match},
		{"?", "ab", noMatch},
	}
	for _, c := range cases {
		db := "\nTERM " + c.pattern + "\n\n.term_matching    00;38;5;61\n"
		r := runDircolorsEnv(t, []string{"TERM=" + c.term}, db, "-b", "-")
		assertExitCode(t, r, 0)
		if r.stdout != c.want {
			t.Errorf("TERM %q vs %q: stdout = %q, want %q", c.pattern, c.term, r.stdout, c.want)
		}
	}
}

func TestDircolorsInvalidTermGlobNeverMatches(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "TERM [\nDIR 01;34\n", "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='';\nexport LS_COLORS\n")
}

func TestDircolorsColortermEmptyWithWildcard(t *testing.T) {
	r := runDircolorsEnv(t, []string{"COLORTERM="}, "COLORTERM ?*\nowt 40;33\n", "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='';\nexport LS_COLORS\n")

	// With a COLORTERM set the wildcard matches and entries are emitted.
	r = runDircolorsEnv(t, []string{"COLORTERM=yes"}, "COLORTERM ?*\nowt 40;33\n", "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='tw=40;33:';\nexport LS_COLORS\n")
}

func TestDircolorsGlobalsBeforeTermLine(t *testing.T) {
	db := ".global 00;36\nTERM xterm*\n.dir 01;34\n"
	r := runDircolorsEnv(t, []string{"TERM=xterm-256color"}, db, "-b", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "LS_COLORS='*.global=00;36:*.dir=01;34:';\nexport LS_COLORS\n")
}

func TestDircolorsFileOperand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "db.txt", "# comment\nTERM screen*\nDIR 01;34\n")

	cmd, _ := Lookup("dircolors")
	var stdout, stderr strings.Builder
	code := cmd.Run(&Context{Dir: dir, Args: []string{"-b", "db.txt"}, CmdName: "dircolors",
		Env: []string{"TERM=screen"}, Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")})
	if code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, stderr.String())
	}
	if want := "LS_COLORS='di=01;34:';\nexport LS_COLORS\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestDircolorsMissingFile(t *testing.T) {
	r := runDircolorsEnv(t, []string{"SHELL=/bin/bash"}, "", "-b", "no-such-file")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "dircolors: no-such-file: ")
}

func TestDircolorsDirectoryOperand(t *testing.T) {
	cmd, _ := Lookup("dircolors")
	var stdout, stderr strings.Builder
	code := cmd.Run(&Context{Dir: t.TempDir(), Args: []string{"-c", "/"}, CmdName: "dircolors",
		Env: []string{}, Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")})
	assertExitCode(t, runResult{code: code}, 2)
	assertStdout(t, runResult{stdout: stdout.String()}, "")
	if got := strings.TrimSpace(stderr.String()); got != "dircolors: expected file, got directory '/'" {
		t.Errorf("stderr = %q", got)
	}
}

func TestDircolorsExtraOperand(t *testing.T) {
	r := runDircolorsEnv(t, []string{"SHELL=/bin/bash"}, "", "-c", "file1", "file2")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "dircolors: extra operand 'file2'\n")
}

func TestDircolorsInvalidLineReportsPosition(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "# comment\n\nnotoken\n", "-b", "-")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "-:3: invalid line;  missing second token")
}

func TestDircolorsUnrecognizedKeyword(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "BOGUS 01;34\n", "-b", "-")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "dircolors: unrecognized keyword BOGUS")
}

func TestDircolorsInvalidOption(t *testing.T) {
	r := runDircolorsEnv(t, []string{"SHELL=/bin/bash"}, "", "--definitely-invalid")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "dircolors: ")
}

func TestDircolorsPrintLSColors(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "", "--print-ls-colors")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	if len(lines) != len(dircolorsTypes)+len(dircolorsExts) {
		t.Fatalf("got %d lines, want %d", len(lines), len(dircolorsTypes)+len(dircolorsExts))
	}
	if lines[0] != "\x1b[0mrs\t0\x1b[0m" {
		t.Errorf("first line = %q", lines[0])
	}
	if lines[1] != "\x1b[01;34mdi\t01;34\x1b[0m" {
		t.Errorf("second line = %q", lines[1])
	}
	last := lines[len(lines)-1]
	if last != "\x1b[00;90m*.rpmsave\t00;90\x1b[0m" {
		t.Errorf("last line = %q", last)
	}
	if strings.Contains(r.stdout, "LS_COLORS") || strings.Contains(r.stdout, "export") {
		t.Errorf("display output carries shell syntax: %q", r.stdout)
	}
}

func TestDircolorsPrintLSColorsFromStdin(t *testing.T) {
	r := runDircolorsEnv(t, []string{}, "owt 40;33\n", "--print-ls-colors", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\x1b[40;33mtw\t40;33\x1b[0m\n")
}

// TestDircolorsColorsAreUnderstoodByLscore pins the contract with
// internal/lscore, which reads LS_COLORS once at package init
// (internal/lscore/color.go parseLSColors). Every entry dircolors emits is
// either an extension pattern or one of the two-letter keys lscore's colour
// lookup consults; the standard keys it does not consult (rs, do, mi, ca --
// Misa never resets to "normal" mid-name and has no door, missing-target or
// capability colouring) are listed separately.
func TestDircolorsColorsAreUnderstoodByLscore(t *testing.T) {
	understood := map[string]bool{
		"di": true, "ln": true, "or": true, "mh": true, "ex": true,
		"pi": true, "so": true, "bd": true, "cd": true,
		"su": true, "sg": true, "tw": true, "ow": true, "st": true,
	}
	ignored := map[string]bool{"rs": true, "do": true, "mi": true, "ca": true}
	r := runDircolorsEnv(t, []string{}, "", "-b")
	assertExitCode(t, r, 0)
	value := strings.TrimSuffix(strings.TrimPrefix(r.stdout, "LS_COLORS='"), "';\nexport LS_COLORS\n")
	for _, entry := range strings.Split(value, ":") {
		if entry == "" {
			continue
		}
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("malformed entry %q", entry)
		}
		if strings.HasPrefix(key, "*") {
			continue
		}
		if !understood[key] && !ignored[key] {
			t.Errorf("LS_COLORS key %q is not a key lscore knows", key)
		}
	}
}

func TestDircolorsDatabaseFileIsEmbedded(t *testing.T) {
	if !strings.HasPrefix(dircolorsDatabase, "# Configuration file for dircolors") {
		t.Fatalf("embedded database starts with %q", dircolorsDatabase[:40])
	}
	if strings.HasSuffix(dircolorsDatabase, "\n") {
		t.Error("embedded database should not end with a newline; -p adds it")
	}
	if _, err := os.Stat(filepath.Join("data", "dircolors.database")); err != nil {
		t.Errorf("database file missing: %v", err)
	}
}
