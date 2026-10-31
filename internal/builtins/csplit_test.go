// Behaviour adapted from uutils coreutils csplit (MIT) and its
// tests/by-util/test_csplit.rs at commit fee61ff5d4f87e0f912a52e639942b5f7ad58663.
package builtins

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// numbers returns the lines from..to-1, each terminated by a newline, which
// is the fixture uutils' test_csplit.rs generates.
func numbers(from, to int) string {
	var b strings.Builder
	for i := from; i < to; i++ {
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\n')
	}
	return b.String()
}

// csplitFiles returns the sorted names of every entry in dir except the input
// file "in".
func csplitFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() == "in" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func csplitContent(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}

type csplitCase struct {
	name   string
	args   []string
	stdin  string
	input  string // content of the file "in"; when empty no file is created
	mkdir  bool   // create "in" as a directory instead of a file
	code   int
	stdout string
	stderr string
	files  []string          // expected output names, sorted
	body   map[string]string // expected contents, by name
}

func runCsplitCases(t *testing.T, cases []csplitCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			switch {
			case tc.mkdir:
				if err := os.Mkdir(filepath.Join(dir, "in"), 0o755); err != nil {
					t.Fatal(err)
				}
			case tc.input != "":
				writeFile(t, dir, "in", tc.input)
			}
			r := runBuiltinStdinIn(t, dir, "csplit", tc.stdin, tc.args...)
			assertExitCode(t, r, tc.code)
			assertStdout(t, r, tc.stdout)
			assertStderr(t, r, tc.stderr)
			got := csplitFiles(t, dir)
			if len(got) != len(tc.files) || len(got) > 0 && !equalStrings(got, tc.files) {
				t.Fatalf("output files = %v, want %v (stdout %q stderr %q)", got, tc.files, r.stdout, r.stderr)
			}
			for name, want := range tc.body {
				if got := csplitContent(t, dir, name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func assertStderr(t *testing.T, r runResult, want string) {
	t.Helper()
	if r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestCsplitRegistered(t *testing.T) {
	if _, ok := Lookup("csplit"); !ok {
		t.Fatal("csplit not registered")
	}
}

func TestCsplitLineNumbers(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:   "up to line",
			args:   []string{"in", "10"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": numbers(10, 51),
			},
		},
		{
			name:   "repeat twice",
			args:   []string{"in", "10", "{2}"},
			input:  numbers(1, 51),
			stdout: "18\n30\n30\n63\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": numbers(10, 20),
				"xx02": numbers(20, 30),
				"xx03": numbers(30, 51),
			},
		},
		{
			name:   "line number sequence",
			args:   []string{"in", "10", "25"},
			input:  numbers(1, 51),
			stdout: "18\n45\n78\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": numbers(10, 25),
				"xx02": numbers(25, 51),
			},
		},
		{
			// a line number at or below the current position makes an
			// empty split and consumes nothing.
			name:   "too small line number",
			args:   []string{"in", "/20/", "10", "/40/"},
			input:  numbers(1, 51),
			stdout: "48\n0\n60\n33\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03"},
			body: map[string]string{
				"xx00": numbers(1, 20),
				"xx01": "",
				"xx02": numbers(20, 40),
				"xx03": numbers(40, 51),
			},
		},
		{
			name:   "too small line number equal",
			args:   []string{"in", "/20/", "20"},
			input:  numbers(1, 51),
			stdout: "48\n0\n93\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 20),
				"xx01": "",
				"xx02": numbers(20, 51),
			},
		},
		{
			name:   "too small line number elided",
			args:   []string{"in", "-z", "/20/", "10", "/40/"},
			input:  numbers(1, 51),
			stdout: "48\n60\n33\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 20),
				"xx01": numbers(20, 40),
				"xx02": numbers(40, 51),
			},
		},
		{
			name:   "corner case line after match",
			args:   []string{"in", "/10/", "11"},
			input:  numbers(1, 51),
			stdout: "18\n3\n120\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": "10\n",
				"xx02": numbers(11, 51),
			},
		},
	})
}

func TestCsplitRegex(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:   "up to match",
			args:   []string{"in", "/9$/"},
			input:  numbers(1, 51),
			stdout: "16\n125\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 9),
				"xx01": numbers(9, 51),
			},
		},
		{
			name:   "up to match repeat twice",
			args:   []string{"in", "/9$/", "{2}"},
			input:  numbers(1, 51),
			stdout: "16\n29\n30\n66\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03"},
			body: map[string]string{
				"xx00": numbers(1, 9),
				"xx01": numbers(9, 19),
				"xx02": numbers(19, 29),
				"xx03": numbers(29, 51),
			},
		},
		{
			name:   "up to match sequence",
			args:   []string{"in", "/9$/", "/5$/"},
			input:  numbers(1, 51),
			stdout: "16\n17\n108\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 9),
				"xx01": numbers(9, 15),
				"xx02": numbers(15, 51),
			},
		},
		{
			name:   "positive offset",
			args:   []string{"in", "/9$/+3"},
			input:  numbers(1, 51),
			stdout: "24\n117\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 12),
				"xx01": numbers(12, 51),
			},
		},
		{
			name:   "positive offset without sign",
			args:   []string{"in", "/9$/3"},
			input:  numbers(1, 51),
			stdout: "24\n117\n",
			files:  []string{"xx00", "xx01"},
		},
		{
			name:   "negative offset",
			args:   []string{"in", "/9$/-3"},
			input:  numbers(1, 51),
			stdout: "10\n131\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 6),
				"xx01": numbers(6, 51),
			},
		},
		{
			name:   "repeat always",
			args:   []string{"in", "/9$/", "{*}"},
			input:  numbers(1, 51),
			stdout: "16\n29\n30\n30\n30\n6\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03", "xx04", "xx05"},
			body: map[string]string{
				"xx00": numbers(1, 9),
				"xx01": numbers(9, 19),
				"xx02": numbers(19, 29),
				"xx03": numbers(29, 39),
				"xx04": numbers(39, 49),
				"xx05": numbers(49, 51),
			},
		},
		{
			// a match on the very first line leaves an empty first split.
			name:   "match on first line",
			args:   []string{"in", "/^1$/"},
			input:  numbers(1, 4),
			stdout: "0\n6\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "",
				"xx01": numbers(1, 4),
			},
		},
		{
			// a match on the last line hands the last line to the final
			// (trailing) split.
			name:   "match on last line",
			args:   []string{"in", "/^4$/"},
			input:  numbers(1, 5),
			stdout: "6\n2\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 4),
				"xx01": "4\n",
			},
		},
		{
			name:   "match last line positive offset overflows",
			args:   []string{"in", "/^4$/+2"},
			input:  numbers(1, 5),
			stdout: "8\n",
			stderr: "csplit: '/^4$/+2': line number out of range\n",
			code:   1,
		},
		{
			name:   "skip to match",
			args:   []string{"in", "%23%"},
			input:  numbers(1, 51),
			stdout: "84\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(23, 51)},
		},
		{
			name:   "skip to match sequence",
			args:   []string{"in", "%0$%", "%^4%"},
			input:  numbers(1, 51),
			stdout: "33\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(40, 51)},
		},
		{
			name:   "skip to match repeat",
			args:   []string{"in", "%0$%", "{1}", "/^4/"},
			input:  numbers(1, 51),
			stdout: "60\n33\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(20, 40),
				"xx01": numbers(40, 51),
			},
		},
		{
			name:   "skip with positive offset",
			args:   []string{"in", "%23%3"},
			input:  numbers(1, 51),
			stdout: "75\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(26, 51)},
		},
		{
			name:   "skip with negative offset",
			args:   []string{"in", "%23%-3"},
			input:  numbers(1, 51),
			stdout: "93\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(20, 51)},
		},
		{
			name:  "skip repeat always consumes everything",
			args:  []string{"in", "%0$%", "{*}"},
			input: numbers(1, 51),
			files: nil,
		},
		{
			name:   "mix",
			args:   []string{"in", "13", "%25%", "/0$/"},
			input:  numbers(1, 51),
			stdout: "27\n15\n63\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 13),
				"xx01": numbers(25, 30),
				"xx02": numbers(30, 51),
			},
		},
		{
			name:   "match not found after a split",
			args:   []string{"in", "/4/", "/nope/"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "6\n135\n",
			stderr: "csplit: '/nope/': match not found\n",
		},
		{
			name:   "match not found with -k keeps files",
			args:   []string{"in", "/4/", "/nope/", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "6\n135\n",
			stderr: "csplit: '/nope/': match not found\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 4),
				"xx01": numbers(4, 51),
			},
		},
		{
			name:   "match not found and repeat always is not an error",
			args:   []string{"in", "/nope/", "{*}"},
			input:  numbers(1, 51),
			stdout: "141\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(1, 51)},
		},
		{
			name:   "match not found on repetition",
			args:   []string{"in", "/9$/", "{50}"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "16\n29\n30\n30\n30\n6\n",
			stderr: "csplit: '/9$/': match not found on repetition 5\n",
		},
		{
			name:   "match not found on repetition with -k",
			args:   []string{"in", "/9$/", "{50}", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "16\n29\n30\n30\n30\n6\n",
			stderr: "csplit: '/9$/': match not found on repetition 5\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03", "xx04", "xx05"},
		},
		{
			name:   "match not found offsets",
			args:   []string{"in", "/nope/-5"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "141\n",
			stderr: "csplit: '/nope/-5': match not found\n",
		},
		{
			name:   "skip match not found",
			args:   []string{"in", "%nope%"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '%nope%': match not found\n",
		},
		{
			name:  "skip match not found repeat always",
			args:  []string{"in", "%nope%", "{*}"},
			input: numbers(1, 51),
			files: nil,
		},
		{
			// carried-over lines from the previous pattern are not matched
			// again: /10/ cannot match line 10 here.
			name:   "pattern does not rematch the carried line",
			args:   []string{"in", "10", "/10/"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "18\n123\n",
			stderr: "csplit: '/10/': match not found\n",
		},
		{
			name:   "negative offset reaching before the split",
			args:   []string{"in", "/20/-10", "/15/"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "18\n123\n",
			stderr: "csplit: '/15/': match not found\n",
		},
		{
			name:   "match offset underflow",
			args:   []string{"in", "/5/-10"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "0\n",
			stderr: "csplit: '/5/-10': line number out of range\n",
		},
		{
			name:   "match offset underflow with -k",
			args:   []string{"in", "/5/-10", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "0\n",
			stderr: "csplit: '/5/-10': line number out of range\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": ""},
		},
		{
			name:   "match offset underflow after a line split",
			args:   []string{"in", "10", "/12/-5", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "18\n0\n",
			stderr: "csplit: '/12/-5': line number out of range\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": "",
			},
		},
		{
			name:   "match offset overflow",
			args:   []string{"in", "/45/+10"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "141\n",
			stderr: "csplit: '/45/+10': line number out of range\n",
		},
		{
			name:   "match offset overflow with -k",
			args:   []string{"in", "/45/+10", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "141\n",
			stderr: "csplit: '/45/+10': line number out of range\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(1, 51)},
		},
		{
			name:   "skip offset underflow",
			args:   []string{"in", "%5%-10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '%5%-10': line number out of range\n",
		},
		{
			name:   "skip offset overflow",
			args:   []string{"in", "%45%+10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '%45%+10': line number out of range\n",
		},
		{
			name:   "negative offset minimum",
			args:   []string{"in", "/45/-2147483648"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "0\n",
			stderr: "csplit: '/45/-2147483648': line number out of range\n",
		},
	})
}

func TestCsplitLineNumberErrors(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:   "out of range",
			args:   []string{"in", "100"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "141\n",
			stderr: "csplit: '100': line number out of range\n",
		},
		{
			name:   "out of range with -k",
			args:   []string{"in", "100", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "141\n",
			stderr: "csplit: '100': line number out of range\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(1, 51)},
		},
		{
			name:   "out of range after a split",
			args:   []string{"in", "10", "100"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "18\n123\n",
			stderr: "csplit: '100': line number out of range\n",
		},
		{
			name:   "out of range on repetition",
			args:   []string{"in", "40", "{2}"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "108\n33\n",
			stderr: "csplit: '40': line number out of range on repetition 1\n",
		},
		{
			name:   "out of range on repetition with -k",
			args:   []string{"in", "40", "{*}", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "108\n33\n",
			stderr: "csplit: '40': line number out of range on repetition 1\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 40),
				"xx01": numbers(40, 51),
			},
		},
		{
			name:   "too small line number repeated",
			args:   []string{"in", "/20/", "10", "{*}", "-k"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "48\n0\n0\n30\n30\n30\n3\n",
			stderr: "csplit: '10': line number out of range on repetition 5\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03", "xx04", "xx05", "xx06"},
			body: map[string]string{
				"xx00": numbers(1, 20),
				"xx01": "",
				"xx02": "",
				"xx03": numbers(20, 30),
				"xx04": numbers(30, 40),
				"xx05": numbers(40, 50),
				"xx06": "50\n",
			},
		},
		{
			name:   "line number zero",
			args:   []string{"in", "0"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: 0: line number must be greater than zero\n",
		},
		{
			name:   "line numbers decrease",
			args:   []string{"in", "10", "5"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: line number '5' is smaller than preceding line number, 10\n",
		},
		{
			name:   "line numbers decrease after a match pattern",
			args:   []string{"in", "10", "/20/", "5"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: line number '5' is smaller than preceding line number, 10\n",
		},
	})
}

func TestCsplitOptions(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:  "quiet short",
			args:  []string{"-s", "in", "13"},
			input: numbers(1, 51),
			files: []string{"xx00", "xx01"},
		},
		{
			name:  "quiet long variants",
			args:  []string{"--quiet", "in", "13"},
			input: numbers(1, 51),
			files: []string{"xx00", "xx01"},
		},
		{
			name:  "silent",
			args:  []string{"--silent", "in", "13"},
			input: numbers(1, 51),
			files: []string{"xx00", "xx01"},
		},
		{
			name:   "prefix",
			args:   []string{"--prefix", "dog", "in", "13"},
			input:  numbers(1, 51),
			stdout: "27\n114\n",
			files:  []string{"dog00", "dog01"},
			body: map[string]string{
				"dog00": numbers(1, 13),
				"dog01": numbers(13, 51),
			},
		},
		{
			name:   "digits",
			args:   []string{"-n", "4", "in", "13"},
			input:  numbers(1, 51),
			stdout: "27\n114\n",
			files:  []string{"xx0000", "xx0001"},
		},
		{
			name:   "suffix format",
			args:   []string{"in", "10", "--suffix-format", "cst-%03d-"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xxcst-000-", "xxcst-001-"},
		},
		{
			name:   "suffix format hyphen leading as separate argument",
			args:   []string{"in", "10", "--suffix-format", "-%02d"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx-00", "xx-01"},
		},
		{
			name:   "suffix format zero precision",
			args:   []string{"in", "10", "--suffix-format", "%.0d"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx", "xx1"},
		},
		{
			name:   "suffix format precision and alternate form",
			args:   []string{"in", "10", "--suffix-format", "%#6.3x"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx   000", "xx 0x001"},
		},
		{
			name:   "suffix format zero flag with precision",
			args:   []string{"in", "10", "--suffix-format", "%0#6.3x"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx   000", "xx 0x001"},
		},
		{
			name:   "suffix format octal",
			args:   []string{"in", "10", "--suffix-format", "%03o"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx000", "xx001"},
		},
		{
			name:   "suffix format upper hex",
			args:   []string{"in", "10", "--suffix-format", "%03X"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx000", "xx001"},
		},
		{
			name:   "suffix format left adjusted",
			args:   []string{"in", "10", "--suffix-format", "%-4d-"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx0   -", "xx1   -"},
		},
		{
			name:   "alternate octal form",
			args:   []string{"in", "10", "--suffix-format", "%#10o"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx         0", "xx        01"},
		},
		{
			name:   "prefix and suffix format",
			args:   []string{"-f", "pre-", "-b", "cst-%03d-post", "in", "10"},
			input:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"pre-cst-000-post", "pre-cst-001-post"},
		},
		{
			name:   "elide empty files",
			args:   []string{"-z", "in", "/20/", "20"},
			input:  numbers(1, 51),
			stdout: "48\n93\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 20),
				"xx01": numbers(20, 51),
			},
		},
		{
			name:   "suppress matched line number",
			args:   []string{"--suppress-matched", "in", "10"},
			input:  numbers(1, 51),
			stdout: "18\n120\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": numbers(11, 51),
			},
		},
		{
			name:   "suppress matched final empty",
			args:   []string{"--suppress-matched", "-", "2", "4", "6"},
			stdin:  numbers(1, 7),
			stdout: "2\n2\n2\n0\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03"},
			body: map[string]string{
				"xx00": "1\n",
				"xx01": "3\n",
				"xx02": "5\n",
				"xx03": "",
			},
		},
		{
			name:   "suppress matched final empty elided",
			args:   []string{"--suppress-matched", "-z", "-", "2", "4", "6"},
			stdin:  numbers(1, 7),
			stdout: "2\n2\n2\n",
			files:  []string{"xx00", "xx01", "xx02"},
		},
		{
			name:   "suppress matched up to match",
			args:   []string{"--suppress-matched", "in", "/0$/", "{*}"},
			input:  numbers(1, 51),
			stdout: "18\n27\n27\n27\n27\n0\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03", "xx04", "xx05"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": numbers(11, 20),
				"xx02": numbers(21, 30),
				"xx03": numbers(31, 40),
				"xx04": numbers(41, 50),
				"xx05": "",
			},
		},
		{
			name:   "suppress matched up to match elided",
			args:   []string{"--suppress-matched", "-z", "in", "/0$/", "{*}"},
			input:  numbers(1, 51),
			stdout: "18\n27\n27\n27\n27\n",
			files:  []string{"xx00", "xx01", "xx02", "xx03", "xx04"},
		},
		{
			name:   "suppress matched positive offset",
			args:   []string{"--suppress-matched", "in", "/10/+4"},
			input:  numbers(1, 51),
			stdout: "30\n108\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 14),
				"xx01": numbers(15, 51),
			},
		},
		{
			name:   "suppress matched negative offset",
			args:   []string{"--suppress-matched", "in", "/10/-4"},
			input:  numbers(1, 51),
			stdout: "10\n129\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 6),
				"xx01": numbers(7, 51),
			},
		},
		{
			name:   "suppress matched skip",
			args:   []string{"--suppress-matched", "in", "%0$%"},
			input:  numbers(1, 51),
			stdout: "120\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(11, 51)},
		},
		{
			name:  "suppress matched skip positive offset elided",
			args:  []string{"-z", "-", "%a%1"},
			stdin: "a\n",
			files: nil,
		},
		{
			name:   "negative offset at start",
			args:   []string{"-", "/a/-1", "{*}"},
			stdin:  "\na\n",
			stdout: "0\n3\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "",
				"xx01": "\na\n",
			},
		},
		{
			name:   "negative offset at start elided",
			args:   []string{"-z", "-", "/a/-1", "{*}"},
			stdin:  "\na\n",
			stdout: "3\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": "\na\n"},
		},
		{
			name: "repeated options last one wins",
			args: []string{"in", "--suppress-matched", "--suppress-matched", "-kzsn",
				"2", "-szkn3", "-b", "%03d", "-b%03x", "-f", "xxy_", "-fxxz_", "/13/", "9", "{5}"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '9': line number out of range on repetition 5\n",
			files:  []string{"xxz_000", "xxz_001", "xxz_002", "xxz_003", "xxz_004", "xxz_005"},
			body: map[string]string{
				"xxz_000": numbers(1, 13),
				"xxz_001": numbers(15, 18),
				"xxz_002": numbers(19, 27),
				"xxz_003": numbers(28, 36),
				"xxz_004": numbers(37, 45),
			},
		},
	})
}

func TestCsplitFormatErrors(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:   "suffix format without conversion",
			args:   []string{"-b", "no conversion string", "in", "10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: incorrect conversion specification in suffix\n",
		},
		{
			name:   "suffix format bad conversion",
			args:   []string{"-b", "%042a", "in", "10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: incorrect conversion specification in suffix\n",
		},
		{
			name:   "suffix format too many conversions",
			args:   []string{"-b", "%02d-%-3x", "in", "10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: too many % conversion specifications in suffix\n",
		},
		{
			name:   "invalid digits",
			args:   []string{"-n", "bad", "in", "10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: invalid number: 'bad'\n",
		},
		{
			name:   "invalid digits negative",
			args:   []string{"-n", "-3", "in", "10"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: invalid number: '-3'\n",
		},
	})
}

func TestCsplitInputHandling(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:   "stdin",
			args:   []string{"-", "10"},
			stdin:  numbers(1, 51),
			stdout: "18\n123\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": numbers(10, 51),
			},
		},
		{
			name:   "stdin without trailing newline",
			args:   []string{"-", "2"},
			stdin:  "a\nb\nc\nd",
			stdout: "2\n5\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "a\n",
				"xx01": "b\nc\nd",
			},
		},
		{
			name:   "empty input",
			args:   []string{"-", "1"},
			code:   1,
			stdout: "0\n",
			stderr: "csplit: '1': line number out of range\n",
		},
		{
			name:   "empty input empty file",
			args:   []string{"-", "/a/"},
			code:   1,
			stdout: "0\n",
			stderr: "csplit: '/a/': match not found\n",
		},
		{
			name:   "missing input file",
			args:   []string{"missing", "10"},
			code:   1,
			stderr: "csplit: cannot open 'missing' for reading: No such file or directory\n",
		},
		{
			name:   "directory input",
			args:   []string{"in", "1"},
			mkdir:  true,
			code:   1,
			stderr: "csplit: read error: Is a directory\n",
		},
		{
			name:   "missing operand",
			code:   1,
			stderr: "csplit: missing operand\n",
		},
		{
			name:   "missing pattern operand",
			args:   []string{"in"},
			input:  numbers(1, 5),
			code:   1,
			stderr: "csplit: missing operand\n",
		},
		{
			name:   "unknown option",
			args:   []string{"--definitely-invalid"},
			code:   1,
			stderr: "csplit: unknown option --definitely-invalid\n",
		},
	})
}

func TestCsplitInvalidPatterns(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			name:   "garbage operand",
			args:   []string{"in", "abc"},
			input:  numbers(1, 5),
			code:   1,
			stderr: "csplit: 'abc': invalid pattern\n",
		},
		{
			name:   "repeat without a pattern",
			args:   []string{"in", "{2}"},
			input:  numbers(1, 5),
			code:   1,
			stderr: "csplit: '{2}': invalid pattern\n",
		},
		{
			name:   "non-ascii repeat",
			args:   []string{"in", "10", "{𝟚}"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '{𝟚}': invalid pattern\n",
		},
		{
			name:   "non-ascii offset",
			args:   []string{"in", "/9$/𝟚"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '/9$/𝟚': invalid pattern\n",
		},
		{
			name:   "unterminated regex",
			args:   []string{"in", "/abc"},
			input:  numbers(1, 5),
			code:   1,
			stderr: "csplit: '/abc': invalid pattern\n",
		},
		{
			name:   "bad regex",
			args:   []string{"in", "/a[b/"},
			input:  numbers(1, 5),
			code:   1,
			stderr: "csplit: '/a[b/': invalid pattern\n",
		},
		{
			// an empty regex is valid POSIX BRE and matches every line.
			name:   "empty up to regex",
			args:   []string{"in", "//"},
			input:  numbers(1, 4),
			stdout: "0\n6\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "",
				"xx01": numbers(1, 4),
			},
		},
		{
			name:   "empty skip regex",
			args:   []string{"in", "%%"},
			input:  numbers(1, 4),
			stdout: "6\n",
			files:  []string{"xx00"},
			body:   map[string]string{"xx00": numbers(1, 4)},
		},
		{
			name:   "unterminated percent delimiter",
			args:   []string{"in", "%abc"},
			input:  numbers(1, 4),
			code:   1,
			stderr: "csplit: '%abc': invalid pattern\n",
		},
	})
}

func TestCsplitHelp(t *testing.T) {
	r := runBuiltin(t, "csplit", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "--suffix-format")
	assertStdoutContains(t, r, "--elide-empty-files")

	r = runBuiltin(t, "csplit", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "(Misa)")
}

func TestCsplitCreateError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can open a mode-000 file for writing")
	}
	dir := t.TempDir()
	writeFile(t, dir, "in", "a\nb\nc\n")
	if err := os.WriteFile(filepath.Join(dir, "xx00"), nil, 0o400); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "csplit", "in", "2")
	assertExitCode(t, r, 1)
	assertStderr(t, r, "csplit: xx00: Permission denied\n")
}

func TestCsplitBoundaryRegex(t *testing.T) {
	// Anchors see one line at a time: ^ and $ bind to the line, not to the
	// whole input.
	runCsplitCases(t, []csplitCase{
		{
			name:   "anchored first line",
			args:   []string{"-", "/^b$/"},
			stdin:  "a\nb\nc\n",
			stdout: "2\n4\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "a\n",
				"xx01": "b\nc\n",
			},
		},
		{
			name:   "anchored last line",
			args:   []string{"-", "/^c$/"},
			stdin:  "a\nb\nc\n",
			stdout: "4\n2\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "a\nb\n",
				"xx01": "c\n",
			},
		},
		{
			name:   "bre leftmost longest",
			args:   []string{"-", "/a\\{2,3\\}/"},
			stdin:  "a\nbbb\naa\n",
			stdout: "6\n3\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "a\nbbb\n",
				"xx01": "aa\n",
			},
		},
	})
}

// TestCsplitUpstreamBoundaries pins behaviour that the newer upstream test
// suite checks explicitly (the vendored snapshot under tests/uutils/testdata
// predates those cases).
func TestCsplitUpstreamBoundaries(t *testing.T) {
	runCsplitCases(t, []csplitCase{
		{
			// the target line may not precede the start of the current split.
			name:   "negative offset before split start",
			args:   []string{"in", "/3$/-3"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "0\n",
			stderr: "csplit: '/3$/-3': line number out of range\n",
		},
		{
			// exactly the first line of the split: an empty split, not an error.
			name:   "negative offset at split start",
			args:   []string{"in", "/3$/-2"},
			input:  numbers(1, 51),
			stdout: "0\n141\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "",
				"xx01": numbers(1, 51),
			},
		},
		{
			name:   "negative offset at second split start",
			args:   []string{"in", "/10$/", "/12$/-2"},
			input:  numbers(1, 51),
			stdout: "18\n0\n123\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": "",
				"xx02": numbers(10, 51),
			},
		},
		{
			name:   "negative offset before second split start",
			args:   []string{"in", "/10$/", "/12$/-3"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "18\n0\n",
			stderr: "csplit: '/12$/-3': line number out of range\n",
		},
		{
			name:   "suppress matched negative offset at second split start",
			args:   []string{"--suppress-matched", "in", "/10$/", "/12$/-1"},
			input:  numbers(1, 51),
			stdout: "18\n0\n117\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": "",
				"xx02": numbers(12, 51),
			},
		},
		{
			name:   "suppress matched line number before current position",
			args:   []string{"in", "--suppress-matched", "/10$/", "5"},
			input:  numbers(1, 51),
			stdout: "18\n0\n117\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": numbers(1, 10),
				"xx01": "",
				"xx02": numbers(12, 51),
			},
		},
		{
			// no line 51 exists, so the split is complete.
			name:   "one past the end with suppress matched",
			args:   []string{"in", "--suppress-matched", "51"},
			input:  numbers(1, 51),
			stdout: "141\n0\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": numbers(1, 51),
				"xx01": "",
			},
		},
		{
			// line 50 was already dropped, so nothing is left for line 51.
			name:   "suppressed last line leaves nothing for one past the end",
			args:   []string{"in", "--suppress-matched", "50", "51"},
			input:  numbers(1, 51),
			code:   1,
			stdout: "138\n0\n",
			stderr: "csplit: '51': line number out of range\n",
		},
		{
			name:   "skip negative offset before split start",
			args:   []string{"in", "%3$%-3"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '%3$%-3': line number out of range\n",
		},
		{
			name:   "positive offset leaves a trailing empty split",
			args:   []string{"-", "/a/+1"},
			stdin:  "1\na\n",
			stdout: "4\n0\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "1\na\n",
				"xx01": "",
			},
		},
		{
			name:   "suppress matched leaves a trailing empty split",
			args:   []string{"--suppress-matched", "-", "2", "/a/"},
			stdin:  "1\n2\n3\n4\na\n",
			stdout: "2\n4\n0\n",
			files:  []string{"xx00", "xx01", "xx02"},
			body: map[string]string{
				"xx00": "1\n",
				"xx01": "3\n4\n",
				"xx02": "",
			},
		},
		{
			// uutils' repeat_everything: options repeat, the last one wins,
			// and the fifth "9" repetition runs past the end of the input.
			name: "every option repeated",
			args: []string{"in", "--suppress-matched", "--suppress-matched", "-kzsn",
				"2", "-szkn3", "-b", "%03d", "-b%03x", "-f", "xxy_", "-fxxz_",
				"/13/", "9", "{5}"},
			input:  numbers(1, 51),
			code:   1,
			stderr: "csplit: '9': line number out of range on repetition 5\n",
			files: []string{"xxz_000", "xxz_001", "xxz_002", "xxz_003", "xxz_004",
				"xxz_005"},
			body: map[string]string{
				"xxz_000": numbers(1, 13),
				"xxz_001": numbers(15, 18),
				"xxz_002": numbers(19, 27),
				"xxz_003": numbers(28, 36),
				"xxz_004": numbers(37, 45),
				"xxz_005": numbers(46, 51),
			},
		},
		{
			// $ is anchored to the end of the line, carriage return included.
			name:   "dollar does not ignore carriage return",
			args:   []string{"-", "/^---$/"},
			stdin:  "x\n---\r\ny\n",
			code:   1,
			stdout: "9\n",
			stderr: "csplit: '/^---$/': match not found\n",
		},
		{
			name:   "dot matches carriage return",
			args:   []string{"-", "/^---.$/"},
			stdin:  "x\n---\r\ny\n",
			stdout: "2\n7\n",
			files:  []string{"xx00", "xx01"},
			body: map[string]string{
				"xx00": "x\n",
				"xx01": "---\r\ny\n",
			},
		},
	})
}
