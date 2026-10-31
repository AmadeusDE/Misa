package builtins

import (
	"strings"
	"testing"
)

// TestGetoptToybox covers the cases from Toybox tests/getopt.test, plus the
// hand-rolled option parsing and error paths.
func TestGetoptToybox(t *testing.T) {
	tests := []struct {
		name string
		args []string
		out  string
		err  string
		code int
	}{
		// Traditional behavior was to take the first argument as OPTSTR and
		// not quote.
		{"legacy operands", []string{"a", "b", "c"}, " -- b c\n", "", 0},
		{"legacy option", []string{"a", "-a", "b", "c"}, " -a -- b c\n", "", 0},
		{"legacy dashdash", []string{"a", "--", "-a", "b", "c"}, " -- -a b c\n", "", 0},

		// Modern -o mode.
		{"modern empty", []string{"-o", "a", "--"}, " --\n", "", 0},
		{"modern operands quoted", []string{"-o", "a", "--", "-a", "b", "c"}, " -a -- 'b' 'c'\n", "", 0},
		{"modern short arg", []string{"-o", "a:", "--", "-a", "b", "c"}, " -a 'b' -- 'c'\n", "", 0},

		// Long options.
		{"long option", []string{"-o", "a", "-l", "long", "--", "-a", "--long", "a"}, " -a --long -- 'a'\n", "", 0},
		{"long repeated", []string{"-o", "a", "-l", "one", "-l", "two", "--", "-a", "--one", "--two"}, " -a --one --two --\n", "", 0},
		{"long comma", []string{"-o", "a", "-l", "one,two", "--", "-a", "--one", "--two"}, " -a --one --two --\n", "", 0},
		{"long required arg", []string{"-o", "a", "-l", "one:", "--", "-a", "--one", "arg"}, " -a --one 'arg' --\n", "", 0},
		{"long optional no arg", []string{"-o", "a", "-l", "one::", "--", "-a", "--one"}, " -a --one '' --\n", "", 0},
		{"long optional next arg is operand", []string{"-o", "a", "-l", "one::", "--", "-a", "--one", "arg"}, " -a --one '' -- 'arg'\n", "", 0},
		{"long optional attached", []string{"-o", "a", "-l", "one::", "--", "-a", "--one=arg"}, " -a --one 'arg' --\n", "", 0},

		// "Alternative" long options.
		{"alternative", []string{"-o", "a", "-a", "-l", "long", "--", "-long", "--long", "a"}, " --long --long -- 'a'\n", "", 0},

		// -u lets you avoid quoting even with modern -o.
		{"unquoted", []string{"-u", "-o", "a:", "--", "-a", "b", "c"}, " -a b -- c\n", "", 0},

		// Quoting.
		{"quote escaping", []string{"-o", "a", "--", "it's"}, " -- 'it'\\''s'\n", "", 0},
		{"unquoted quote", []string{"-o", "a", "-u", "--", "it's"}, " -- it's\n", "", 0},
		// tests/getopt.test passes "it\'s" (bash keeps the backslash inside
		// double quotes), which is what its expectation string contains.
		{"toybox quote escaping", []string{"-o", "a", "--", `it\'s`}, " -- 'it\\'\\''s'\n", "", 0},
		{"toybox unquoted quote", []string{"-o", "a", "-u", "--", `it\'s`}, " -- it\\'s\n", "", 0},

		// Odds and ends.
		{"test mode", []string{"-T"}, "", "", 4},
		{"name in errors", []string{"-n", "unlikely", "a", "-x"}, " --\n", "unlikely: invalid option 'x'\n", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "getopt", tc.args...)
			assertStdout(t, r, tc.out)
			if r.stderr != tc.err {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.err)
			}
			assertExitCode(t, r, tc.code)
		})
	}
}

// TestGetoptOwnOptions covers getopt's own option forms: clusters, attached
// values, long names and unambiguous prefixes.
func TestGetoptOwnOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		out  string
		err  string
		code int
	}{
		{"attached optstr", []string{"-oa:", "--", "-a", "x"}, " -a 'x' --\n", "", 0},
		{"attached name", []string{"-nfoo", "-o", "a", "--", "-q"}, " --\n", "foo: invalid option 'q'\n", 1},
		{"long name value", []string{"--nam", "foo", "--options", "a", "--", "-q"}, " --\n", "foo: invalid option 'q'\n", 1},
		{"long prefix alt", []string{"--alt", "-o", "a", "--", "-a"}, " -a --\n", "", 0},
		{"long prefix ambiguous is an error", []string{"--l"}, "", "getopt: ambiguous option '--l'\n", 1},
		{"long options attached", []string{"--long=one", "-o", "a", "--", "--one"}, " --one --\n", "", 0},
		{"longoptions comma", []string{"--longoptions", "one,two", "-o", "a", "--", "--two"}, " --two --\n", "", 0},
		{"cluster alt+long", []string{"-al", "one", "-o", "a", "--", "-one"}, " --one --\n", "", 0},
		{"double dash ends own options", []string{"-o", "a", "--", "--alt"}, " --\n", "getopt: invalid option '--alt'\n", 1},

		{"unknown short", []string{"-q"}, "", "getopt: unknown option '-q'\n", 1},
		{"unknown upper", []string{"-Q"}, "", "getopt: unknown option '-Q'\n", 1},
		{"unknown sh", []string{"-s", "sh"}, "", "getopt: unknown option '-s'\n", 1},
		{"unknown long test", []string{"--test"}, "", "getopt: unknown option '--test'\n", 1},
		{"unknown long unquoted", []string{"--unquoted"}, "", "getopt: unknown option '--unquoted'\n", 1},
		{"unknown long bogus", []string{"--bogus"}, "", "getopt: unknown option '--bogus'\n", 1},
		{"missing -o value", []string{"-o"}, "", "getopt: option '-o' requires an argument\n", 1},
		{"missing -n value", []string{"-n"}, "", "getopt: option '-n' requires an argument\n", 1},
		{"missing -l value", []string{"-l"}, "", "getopt: option '-l' requires an argument\n", 1},
		{"missing --name value", []string{"--name"}, "", "getopt: option '--name' requires an argument\n", 1},
		{"no value for flag", []string{"--alternative=x"}, "", "getopt: option '--alternative' takes no argument\n", 1},

		{"bad -l entry", []string{"-l", "bad:::name"}, "", "getopt: bad -l 'bad:::name'\n", 1},
		{"three colons", []string{"-l", "bad:::"}, "", "getopt: bad -l 'bad:::'\n", 1},
		{"empty -l entry", []string{"-l", ""}, "", "getopt: bad -l ''\n", 1},
		{"bad -l entry among good", []string{"-l", "one,bad:::x", "-o", "a", "--"}, "", "getopt: bad -l 'bad:::x'\n", 1},

		{"no optstr", []string{}, "", "getopt: no OPTSTR\n", 1},
		{"empty optstr operand", []string{"", "x"}, "", "getopt: no OPTSTR\n", 1},
		{"empty optstr option", []string{"-o", "", "--"}, "", "getopt: no OPTSTR\n", 1},

		// -T is checked before -l validation and before the OPTSTR.
		{"test ignores bad -l", []string{"-T", "-l", "bad:::x"}, "", "", 4},
		{"test ignores operands", []string{"-T", "operand"}, "", "", 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "getopt", tc.args...)
			assertStdout(t, r, tc.out)
			if r.stderr != tc.err {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.err)
			}
			assertExitCode(t, r, tc.code)
		})
	}
}

// TestGetoptParseErrors covers errors in the caller's command line: parsing
// continues, the line is still printed and the status becomes 1.
func TestGetoptParseErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		out  string
		err  string
		code int
	}{
		{"bad short continues", []string{"-o", "ab:", "--", "-a", "-z", "-b", "v"}, " -a -b 'v' --\n", "getopt: invalid option 'z'\n", 1},
		{"bad short cluster", []string{"-o", "a", "--", "-az"}, " -a --\n", "getopt: invalid option 'z'\n", 1},
		{"short arg missing", []string{"-o", "a:", "--", "-a"}, " --\n", "getopt: invalid option 'a'\n", 1},
		{"long arg missing", []string{"-o", "a", "-l", "one:", "--", "--one"}, " --\n", "getopt: invalid option '--one'\n", 1},
		{"long arg missing after operand", []string{"-o", "a", "-l", "one:", "--", "op", "--one"}, " -- 'op'\n", "getopt: invalid option '--one'\n", 1},
		{"long no arg given value", []string{"-o", "a", "-l", "one", "--", "--one=v"}, " --\n", "getopt: invalid option '--one'\n", 1},
		{"unknown long", []string{"-o", "a", "--", "--bogus"}, " --\n", "getopt: invalid option '--bogus'\n", 1},
		{"unknown long with name", []string{"-n", "prog", "-o", "a", "--", "--bogus"}, " --\n", "prog: invalid option '--bogus'\n", 1},
		{"ambiguous long", []string{"-o", "a", "-l", "one,only", "--", "--on"}, " --\n", "getopt: ambiguous option '--on'\n", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "getopt", tc.args...)
			assertStdout(t, r, tc.out)
			if r.stderr != tc.err {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.err)
			}
			assertExitCode(t, r, tc.code)
		})
	}
}

// TestGetoptPermutation checks GNU-style permutation and the remaining output
// format details.
func TestGetoptPermutation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		out  string
		code int
	}{
		{"option after operand", []string{"-o", "a:", "--", "x", "-a", "y"}, " -a 'y' -- 'x'\n", 0},
		{"operands keep order", []string{"-o", "a", "--", "1", "-a", "2", "3"}, " -a -- '1' '2' '3'\n", 0},
		{"dashdash operand", []string{"-o", "a", "--", "--", "--", "x y"}, " -- '--' 'x y'\n", 0},
		{"lone dash is an operand", []string{"-o", "a", "--", "-", "x"}, " -- '-' 'x'\n", 0},
		{"empty operand", []string{"-o", "a", "--", ""}, " -- ''\n", 0},
		{"long abbreviation", []string{"-o", "a", "-l", "one,two", "--", "--tw"}, " --two --\n", 0},
		{"long arg from next param", []string{"-o", "a", "-l", "one:", "--", "--one", "--two"}, " --one '--two' --\n", 0},
		{"long repeats", []string{"-o", "a", "-l", "one:", "--", "--one", "v", "--one=w"}, " --one 'v' --one 'w' --\n", 0},
		{"short optional attached only", []string{"-o", "a::", "--", "-a", "-ax"}, " -a -a 'x' --\n", 0},
		{"short cluster", []string{"-o", "ab", "--", "-ab"}, " -a -b --\n", 0},
		// Documented deviation: in -a mode a single-dash token is tried as a
		// long option first, even when its first character is in OPTSTR.
		{"alternative wins over short", []string{"-a", "-o", "a", "-l", "a", "--", "-a"}, " --a --\n", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "getopt", tc.args...)
			assertStdout(t, r, tc.out)
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
			assertExitCode(t, r, tc.code)
		})
	}
}

func TestGetoptHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		r := runBuiltin(t, "getopt", args...)
		assertExitCode(t, r, 0)
		if r.stderr != "" {
			t.Errorf("%v: stderr = %q, want empty", args, r.stderr)
		}
		for _, want := range []string{
			"usage: getopt [-aTu] [-lo OPTIONS] [-n NAME] [OPTIONS] ARG...",
			"Allow long options starting with a single -",
			"Example:",
			"$ getopt -l long:,arg:: abc command --long -b there --arg",
			"--long '-b' --arg '' -- 'command' 'there'",
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("%v: stdout does not contain %q:\n%s", args, want, r.stdout)
			}
		}
	}
}

func TestGetoptTestModeIsSilent(t *testing.T) {
	r := runBuiltin(t, "getopt", "-T")
	if r.stdout != "" || r.stderr != "" {
		t.Errorf("stdout = %q, stderr = %q, want both empty", r.stdout, r.stderr)
	}
	assertExitCode(t, r, 4)
}
