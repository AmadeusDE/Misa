package builtins

import (
	"math"
	"testing"
)

func TestCmpIdentical(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "same content\n")
	writeFile(t, dir, "b.txt", "same content\n")
	r := runBuiltinIn(t, dir, "cmp", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestCmpDifferent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "hello\n")
	writeFile(t, dir, "b.txt", "world\n")
	r := runBuiltinIn(t, dir, "cmp", "a.txt", "b.txt")
	assertExitCode(t, r, 1)
	assertStdoutContains(t, r, "differ")
	assertStdoutContains(t, r, "byte 1")
	assertStdoutContains(t, r, "line 1")
}

func TestCmpSilent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")
	r := runBuiltinIn(t, dir, "cmp", "-s", "a.txt", "b.txt")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
}

func TestCmpSilentSame(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "same\n")
	writeFile(t, dir, "b.txt", "same\n")
	r := runBuiltinIn(t, dir, "cmp", "-s", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
}

func TestCmpVerbose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "abc\n")
	writeFile(t, dir, "b.txt", "axc\n")
	r := runBuiltinIn(t, dir, "cmp", "-l", "a.txt", "b.txt")
	assertExitCode(t, r, 1)
	assertStdoutContains(t, r, "2 ")
}

func TestCmpDifferentLengths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "short\n")
	writeFile(t, dir, "b.txt", "short\nand longer\n")
	r := runBuiltinIn(t, dir, "cmp", "a.txt", "b.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "EOF")
}

func TestCmpMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "data\n")
	r := runBuiltinIn(t, dir, "cmp", "a.txt", "missing.txt")
	assertExitCode(t, r, 2)
}

func TestCmpMissingOperand(t *testing.T) {
	r := runBuiltin(t, "cmp", "onlyone.txt")
	assertExitCode(t, r, 2)
}

// The following tests port the unit tests of parse_params from
// uutils diffutils src/cmp.rs (MIT licensed).

func TestCmp_positional(t *testing.T) {
	type want struct {
		from, to           string
		skipA, skipB       uint64
		skipASet, skipBSet bool
	}
	cases := []struct {
		name string
		args []string
		want want
		err  string
	}{
		{name: "two_operands", args: []string{"foo", "bar"},
			want: want{from: "foo", to: "bar"}},
		{name: "one_operand_defaults_to_stdin", args: []string{"foo"},
			want: want{from: "foo", to: "-"}},
		{name: "double_dash_ends_options", args: []string{"foo", "--", "--help"},
			want: want{from: "foo", to: "--help"}},
		{name: "positional_skip_a", args: []string{"foo", "bar", "1"},
			want: want{from: "foo", to: "bar", skipA: 1, skipASet: true}},
		{name: "positional_skips_suffix", args: []string{"foo", "bar", "1", "2Y"},
			want: want{from: "foo", to: "bar", skipA: 1, skipASet: true, skipB: math.MaxUint64, skipBSet: true}},
		{name: "too_many_operands", args: []string{"foo", "bar", "1", "2", "3"},
			err: "Usage: cmp <from> <to>"},
		{name: "no_operands", args: nil,
			err: "Usage: cmp <from> <to>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := cmpParseArgs("cmp", tc.args)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if p.from != tc.want.from || p.to != tc.want.to {
				t.Errorf("from,to = %q,%q, want %q,%q", p.from, p.to, tc.want.from, tc.want.to)
			}
			if p.skipA != tc.want.skipA || p.skipASet != tc.want.skipASet {
				t.Errorf("skipA = %d (set=%v), want %d (set=%v)", p.skipA, p.skipASet, tc.want.skipA, tc.want.skipASet)
			}
			if p.skipB != tc.want.skipB || p.skipBSet != tc.want.skipBSet {
				t.Errorf("skipB = %d (set=%v), want %d (set=%v)", p.skipB, p.skipBSet, tc.want.skipB, tc.want.skipBSet)
			}
		})
	}
}

func TestCmp_execution_modes(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		printBytes bool
		verbose    bool
		quiet      bool
		err        string
	}{
		{name: "short_print_bytes", args: []string{"-b", "foo", "bar"}, printBytes: true},
		{name: "long_print_bytes", args: []string{"--print-bytes", "foo", "bar"}, printBytes: true},
		{name: "short_verbose", args: []string{"-l", "foo", "bar"}, verbose: true},
		{name: "long_verbose", args: []string{"--verbose", "foo", "bar"}, verbose: true},
		{name: "separate_l_b", args: []string{"-l", "-b", "foo", "bar"}, verbose: true, printBytes: true},
		{name: "combined_lb", args: []string{"-lb", "foo", "bar"}, verbose: true, printBytes: true},
		{name: "combined_bl", args: []string{"-bl", "foo", "bar"}, verbose: true, printBytes: true},
		{name: "short_quiet", args: []string{"-s", "foo", "bar"}, quiet: true},
		{name: "incompatible", args: []string{"-l", "-s", "foo", "bar"},
			err: "cmp: options -l and -s are incompatible"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := cmpParseArgs("cmp", tc.args)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if p.printBytes != tc.printBytes || p.verbose != tc.verbose || p.quiet != tc.quiet {
				t.Errorf("printBytes,verbose,quiet = %v,%v,%v, want %v,%v,%v",
					p.printBytes, p.verbose, p.quiet, tc.printBytes, tc.verbose, tc.quiet)
			}
			if p.from != "foo" || p.to != "bar" {
				t.Errorf("from,to = %q,%q, want foo,bar", p.from, p.to)
			}
		})
	}
}

func TestCmp_max_bytes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want *uint64
		err  string
	}{
		{name: "short_n", args: []string{"-n", "1", "foo", "bar"}, want: uint64Ptr(1)},
		{name: "long_separate", args: []string{"--bytes", "1", "foo", "bar"}, want: uint64Ptr(1)},
		{name: "long_equals", args: []string{"--bytes=1", "foo", "bar"}, want: uint64Ptr(1)},
		{name: "overflow_saturates", args: []string{
			"--bytes=99999999999999999999999999999999999999999999999999999999999",
			"foo", "bar"}, want: uint64Ptr(math.MaxUint64)},
		{name: "unit_suffix_rejected", args: []string{"--bytes=1K", "foo", "bar"},
			err: "cmp: invalid --bytes value '1K'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := cmpParseArgs("cmp", tc.args)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.want == nil != (p.maxBytes == nil) {
				t.Fatalf("maxBytes set = %v, want %v", p.maxBytes != nil, tc.want != nil)
			}
			if tc.want != nil && *p.maxBytes != *tc.want {
				t.Errorf("maxBytes = %d, want %d", *p.maxBytes, *tc.want)
			}
		})
	}
}

func uint64Ptr(v uint64) *uint64 { return &v }

func TestCmp_skips(t *testing.T) {
	suffixCases := []struct {
		suffix string
		want   uint64
	}{
		{"kB", 1000},
		{"K", 1024},
		{"MB", 1000 * 1000},
		{"M", 1024 * 1024},
		{"GB", 1000 * 1000 * 1000},
		{"G", 1024 * 1024 * 1024},
		{"TB", 1000 * 1000 * 1000 * 1000},
		{"T", 1024 * 1024 * 1024 * 1024},
		{"PB", 1000 * 1000 * 1000 * 1000 * 1000},
		{"P", 1024 * 1024 * 1024 * 1024 * 1024},
		{"EB", 1000 * 1000 * 1000 * 1000 * 1000 * 1000},
		{"E", 1024 * 1024 * 1024 * 1024 * 1024 * 1024},
		{"ZB", math.MaxUint64},
		{"Z", math.MaxUint64},
		{"YB", math.MaxUint64},
		{"Y", math.MaxUint64},
	}
	cases := []struct {
		name  string
		args  []string
		skipA uint64
		skipB uint64
		err   string
	}{
		{name: "short_i", args: []string{"-i", "1", "foo", "bar"}, skipA: 1, skipB: 1},
		{name: "long_equals", args: []string{"--ignore-initial=1", "foo", "bar"}, skipA: 1, skipB: 1},
		{name: "overflow_saturates", args: []string{
			"-i", "99999999999999999999999999999999999999999999999999999999999",
			"foo", "bar"}, skipA: math.MaxUint64, skipB: math.MaxUint64},
		{name: "colon_pair", args: []string{"--ignore-initial=1:2", "foo", "bar"}, skipA: 1, skipB: 2},
		{name: "colon_suffixes", args: []string{"--ignore-initial=1GB:2E", "foo", "bar"},
			skipA: 1000 * 1000 * 1000, skipB: 2 * 1152921504606846976},
		{name: "positional_ignored", args: []string{"-i", "1:2", "foo", "bar", "3", "4"},
			skipA: 1, skipB: 2},
		{name: "invalid_unit", args: []string{"--ignore-initial=1mb", "foo", "bar"},
			err: "cmp: invalid --ignore-initial value '1mb'"},
		{name: "two_colons", args: []string{"--ignore-initial=1:2:3", "foo", "bar"},
			err: "cmp: invalid --ignore-initial value '1:2:3'"},
		{name: "negative", args: []string{"--ignore-initial=-1", "foo", "bar"},
			err: "cmp: invalid --ignore-initial value '-1'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := cmpParseArgs("cmp", tc.args)
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if p.skipA != tc.skipA || p.skipB != tc.skipB {
				t.Errorf("skipA,skipB = %d,%d, want %d,%d", p.skipA, p.skipB, tc.skipA, tc.skipB)
			}
		})
	}
	for _, sc := range suffixCases {
		t.Run("suffix_"+sc.suffix, func(t *testing.T) {
			p, err := cmpParseArgs("cmp", []string{"-i", "1" + sc.suffix + ":2", "foo", "bar"})
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if p.skipA != sc.want || p.skipB != 2 {
				t.Errorf("skipA,skipB = %d,%d, want %d,2", p.skipA, p.skipB, sc.want)
			}
		})
	}
}
