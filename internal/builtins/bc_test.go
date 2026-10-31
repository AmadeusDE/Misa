package builtins

import (
	"bytes"
	"strings"
	"testing"
)

// End-to-end tests for the bc builtin: bytes in, bytes out, exit status.
// Arithmetic precision itself is covered by bc_num_test.go and bc_scale_test.go.

func TestBcRegistered(t *testing.T) {
	cmd, ok := Lookup("bc")
	if !ok {
		t.Fatal("bc not registered")
	}
	if cmd.Run == nil {
		t.Fatal("bc has no run function")
	}
}

func TestBcStdinPrograms(t *testing.T) {
	tests := []struct {
		name string
		prog string
		want string
		code int
	}{
		{"add", "1+1\n", "2\n", 0},
		{"assignment-is-not-printed", "x = 5\nx\n", "5\n", 0},
		{"paren-assignment-prints", "(x = 5)\n", "5\n", 0},
		{"multi-statement-line", "1; 2\n", "1\n2\n", 0},
		{"no-trailing-newline", "1+1", "2\n", 0},
		{"decimals", "1.5 + 2.5\n", "4.0\n", 0},
		{"leading-dot-output", "scale=2\n1/2\n", ".50\n", 0},
		{"zero-prints-bare", "0.000\n", "0\n", 0},
		{"negative-zero", "0 - 0.0\n", "0\n", 0},
		{"last-dot", "1+1\n.\n", "2\n2\n", 0},
		{"line-continuation", "1 + \\\n1\n", "2\n", 0},
		{"comment", "/* hello */ 1 + 1\n", "2\n", 0},
		{"hash-comment", "1 + 1 # trailing\n", "2\n", 0},
		{"multiline-block-from-pipe", "define f(n) {\n  return (n * 2)\n}\nf(21)\n", "42\n", 0},
		{"if-else", "if (1) 1 else 2\nif (0) 3 else 4\n", "1\n4\n", 0},
		{"while-break", "i = 0\nwhile (1) { i = i + 1; if (i == 3) break }\ni\n", "3\n", 0},
		{"for-continue", "for (i = 0; i < 4; i++) { if (i == 2) continue; i }\n", "0\n1\n3\n", 0},
		{"for-empty-parts", "i = 0\nfor (;;) { i = i + 1; if (i > 2) break }\ni\n", "3\n", 0},
		{"print-string", "print \"a\\tb\\n\"\n", "a\tb\n", 0},
		// A string statement prints verbatim: no escapes, no newline.
		{"string-statement", "\"x\\ty\"\n", `x\ty`, 0},
		{"print-no-newline", "print 1, 2\n3\n", "123\n", 0},
		{"quit-stops-parsing", "1+1\nquit\n2+2\n", "2\n", 0},
		{"halt-stops-run", "1+1\nhalt\n2+2\n", "2\n", 0},
		{"ibase-hex", "ibase=16\nFF\n", "255\n", 0},
		{"obase-hex", "obase=16\n255\n", "FF\n", 0},
		{"obase-binary-fraction", "obase=2\n.1\n", ".0001\n", 0},
		{"obase-groups", "obase=100\n12345\n", " 01 23 45\n", 0},
		{"single-digit-letters", "ibase=2\nA\n", "10\n", 0},
		{"uppercase-digits-clamp", "ibase=2\n11A\n", "7\n", 0},
		{"length-scale", "length(123.456)\nscale(123.456)\n", "6\n3\n", 0},
		{"length-zero", "length(0)\n", "0\n", 0},
		{"sqrt", "scale=5\nsqrt(2)\n", "1.41421\n", 0},
		{"abs", "abs(-4.5)\n", "4.5\n", 0},
		{"increment-postfix", "x = 5\nx++\nx\n++x\n", "5\n6\n7\n", 0},
		{"decrement-prefix", "x = 5\n--x\nx\n", "4\n4\n", 0},
		{"arrays", "a[2] = 42\na[2]\na[1]\n", "42\n0\n", 0},
		{"compound-assign", "x = 2\nx *= 3\nx\n", "6\n", 0},
		{"comparisons", "1 < 2\n2 <= 1\n1 == 1\n1 != 1\n", "1\n0\n1\n0\n", 0},
		{"logical", "!(0)\n!(3)\n1 && 0\n0 || 2\n", "1\n0\n0\n1\n", 0},
		{"negative-power-precedence", "-2^2\n2^3^2\n", "-4\n512\n", 0},
		{"void-function", "define void v(x) { print \"v=\", x, \"\\n\"; return }\nv(5)\n", "v=5\n", 0},
		{"recursion", "define fact(n) { if (n <= 1) return (1); return (n * fact(n-1)) }\nfact(10)\n", "3628800\n", 0},
		{"limits", "limits\n", "BC_BASE_MAX", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "bc", tc.prog)
			assertExitCode(t, r, tc.code)
			if tc.name == "limits" {
				assertStdoutContains(t, r, tc.want)
				return
			}
			if r.stdout != tc.want {
				t.Errorf("stdout = %q, want %q (stderr %q)", r.stdout, tc.want, r.stderr)
			}
		})
	}
}

func TestBcErrors(t *testing.T) {
	tests := []struct {
		name string
		prog string
		want string
	}{
		{"divide-by-zero", "1/0\n", "divide by zero"},
		{"bad-ibase", "ibase=1\n", "bad ibase"},
		{"bad-obase", "obase=1\n", "bad obase"},
		{"bad-scale", "scale=100001\n", "bad scale"},
		{"non-integer-exponent", "2^1.5\n", "non integer number"},
		{"negative-sqrt", "sqrt(-1)\n", "negative number"},
		{"undefined-function", "f(2)\n", "undefined function: f()"},
		{"parameter-count", "define f(a) { return (a) }\nf(1,2)\n", "mismatched parameters; need 1, have 2"},
		{"break-outside-loop", "break\n", "break outside a loop"},
		{"continue-outside-loop", "continue\n", "continue outside a loop"},
		{"unterminated-string", "print \"abc\n", "string end could not be found"},
		{"unterminated-comment", "/* abc\n", "comment end could not be found"},
		{"syntax", "1 +\n", "bad expression"},
		{"read-unsupported", "read()\n", "read() is not supported"},
		{"array-parameters", "define f(a[]) { return (1) }\n", "array parameters are not supported"},
		{"void-return-value", "define void f() { return (1) }\n", "cannot return a value from a void function"},
		{"return-outside-function", "return (1)\n", "not in a function"},
		{"huge-exponent", "2^10000001\n", "too large"},
		{"huge-number", strings.Repeat("9", 1000001) + "\n", "too large"},
		{"call-depth", "define f(n) { return (f(n)) }\nf(1)\n", "call stack depth exceeded"},
		{"bad-character", "1 @ 2\n", "bad character"},
		{"lowercase-in-number", "ibase=16\n1a\n", "unexpected"},
		{"second-auto", "define f() { auto a\nauto b\nreturn (1) }\nf()\n", "no auto variable found"},
		{"increment-parenthesized", "x=1\n(x)++\n", "operand must be a variable"},
		{"unclosed-block", "define f() {\n  return (1)\n", "end of input"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "bc", tc.prog)
			assertExitCode(t, r, 1)
			if !strings.Contains(r.stderr, tc.want) {
				t.Errorf("stderr = %q, want it to contain %q", r.stderr, tc.want)
			}
			if !strings.HasPrefix(r.stderr, "bc: ") {
				t.Errorf("stderr = %q, want a bc: prefix", r.stderr)
			}
		})
	}
}

func TestBcFilesThenStdin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.bc", "x = 10\n")
	writeFile(t, dir, "b.bc", "x + 1\n")
	r := runBuiltinStdinIn(t, dir, "bc", "x + 2\n", "a.bc", "b.bc")
	assertExitCode(t, r, 0)
	if r.stdout != "11\n12\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "11\n12\n")
	}
}

func TestBcDashOperandIsStdin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.bc", "1\n")
	r := runBuiltinStdinIn(t, dir, "bc", "2\n", "a.bc", "-")
	assertExitCode(t, r, 0)
	if r.stdout != "1\n2\n" {
		t.Errorf("stdout = %q", r.stdout)
	}
}

func TestBcMissingFile(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinStdinIn(t, dir, "bc", "1\n", "nope.bc")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "bc: nope.bc: No such file or directory")
	if r.stdout != "" {
		t.Errorf("stdout = %q, want none", r.stdout)
	}
}

func TestBcFileErrorStopsBeforeStdin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.bc", "1/0\n")
	r := runBuiltinStdinIn(t, dir, "bc", "1+1\n", "bad.bc")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "divide by zero")
}

func TestBcOptionErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-l"}, "not implemented"},
		{[]string{"-s"}, "not implemented"},
		{[]string{"-w"}, "not implemented"},
		{[]string{"-Z"}, "unknown option"},
	}
	for _, tc := range tests {
		r := runBuiltinStdin(t, "bc", "", tc.args...)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, tc.want)
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want none", tc.args, r.stdout)
		}
	}
}

func TestBcHelpAndVersion(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "usage: bc")

	r = runBuiltinStdin(t, "bc", "", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "bc (Misa)")
}

func TestBcInteractiveBanner(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "1\n", "-i")
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, "bc (Misa)") {
		t.Errorf("stdout = %q, want the banner first", r.stdout)
	}
	r = runBuiltinStdin(t, "bc", "1\n", "-i", "-q")
	assertExitCode(t, r, 0)
	if r.stdout != "1\n" {
		t.Errorf("stdout = %q, want no banner", r.stdout)
	}
}

func TestBcOutputWrapping(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "scale=100\n1/3\n")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), r.stdout)
	}
	if !strings.HasSuffix(lines[0], "\\") {
		t.Errorf("first line %q does not end in a continuation", lines[0])
	}
	if len(lines[0]) != 69 {
		t.Errorf("first line length = %d, want 69 (68 chars plus backslash)", len(lines[0]))
	}

	// BC_LINE_LENGTH changes the wrap column.
	r = runBcWithEnv(t, "BC_LINE_LENGTH=20", "scale=30\n1/3\n")
	lines = strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	for _, line := range lines {
		if len(line) > 20 {
			t.Errorf("line %q is longer than the 20 column limit", line)
		}
	}
}

func TestBcCancelStopsInfiniteLoop(t *testing.T) {
	cancel := make(chan struct{})
	done := make(chan runResult, 1)
	go func() {
		done <- runBcWithCancel(t, cancel, "while (1) x = x + 1\n")
	}()
	close(cancel)
	r := <-done
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "interrupted")
}

func TestBcCancelStopsInfiniteFunction(t *testing.T) {
	cancel := make(chan struct{})
	close(cancel)
	r := runBcWithCancel(t, cancel, "define f(n) { return (f(n+1)) }\nf(1)\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "interrupted")
}

func TestBcBrokenStdoutStops(t *testing.T) {
	cmd, _ := Lookup("bc")
	var stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Args:   nil,
		Stdout: failingWriter{},
		Stderr: &stderr,
		Stdin:  strings.NewReader("while (1) 1\n"),
	}
	if code := cmd.Run(ctx); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

// runBcWithEnv runs bc with one extra environment variable.
func runBcWithEnv(t *testing.T, env, stdin string) runResult {
	t.Helper()
	cmd, ok := Lookup("bc")
	if !ok {
		t.Fatal("bc not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Env:    []string{env},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// runBcWithCancel runs bc with a cancellation channel that the test can close.
func runBcWithCancel(t *testing.T, cancel <-chan struct{}, stdin string) runResult {
	t.Helper()
	cmd, ok := Lookup("bc")
	if !ok {
		t.Fatal("bc not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Cancel: cancel,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestBcVoidValueInExpression(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "define void v() { print \"x\" }\n1 + v()\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot use a void value in an expression")

	r = runBuiltinStdin(t, "bc", "define void v() { print \"x\" }\nprint v()\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot use a void value in an expression")
}

func TestBcLastAndPrint(t *testing.T) {
	// print updates last; a bare assignment neither prints nor updates last.
	r := runBuiltinStdin(t, "bc", "1+1\nprint 7\nlast\n")
	assertExitCode(t, r, 0)
	if r.stdout != "2\n77\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "2\n77\n")
	}

	r = runBuiltinStdin(t, "bc", "5\nx = 42\nlast\n")
	assertExitCode(t, r, 0)
	if r.stdout != "5\n5\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "5\n5\n")
	}
}

func TestBcLastAssignment(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "1+1\nlast = 9\nlast\n")
	assertExitCode(t, r, 0)
	if r.stdout != "2\n9\n" {
		t.Errorf("stdout = %q", r.stdout)
	}
}

func TestBcIbaseAffectsLaterLiterals(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "ibase=16\n10\nibase=10\n10\n")
	assertExitCode(t, r, 0)
	// ibase=10 while ibase is 16 reads the literal 10 as 16, so ibase stays 16.
	if r.stdout != "16\n16\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "16\n16\n")
	}
}

func TestBcEmptyStrings(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "print \"\"\n\"\"\n")
	assertExitCode(t, r, 0)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want nothing", r.stdout)
	}
	r = runBuiltinStdin(t, "bc", "print \"a\", \"\", \"b\"\n")
	assertExitCode(t, r, 0)
	if r.stdout != "ab" {
		t.Errorf("stdout = %q, want %q", r.stdout, "ab")
	}
}

func TestBcEmptyStatementBodies(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "if (0) ;\n1\nfor (i = 0; i < 1; i++) ;\ni\n")
	assertExitCode(t, r, 0)
	if r.stdout != "1\n1\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "1\n1\n")
	}
}

func TestBcDefineInsideFunctionRejected(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "define f() { define g() { return (1) }; return (2) }\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "define inside a function")
}

func TestBcQuitDiscardsUnit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "q.bc", "1+1; quit\n")
	r := runBuiltinStdinIn(t, dir, "bc", "", "q.bc")
	assertExitCode(t, r, 0)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want nothing: quit is a compile-time command", r.stdout)
	}
}

func TestBcZeroKeepsScale(t *testing.T) {
	// A zero result still carries the scale the operation produced.
	r := runBuiltinStdin(t, "bc", "scale=5\n0/1\nscale(0/1)\n0.00\nscale(0.00)\n")
	assertExitCode(t, r, 0)
	if r.stdout != "0\n5\n0\n2\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "0\n5\n0\n2\n")
	}
}

func TestBcInteractiveContinuesAfterError(t *testing.T) {
	r := runBuiltinStdin(t, "bc", "1/0\n2+2\n", "-i", "-q")
	assertExitCode(t, r, 0)
	if r.stdout != "4\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "4\n")
	}
	assertStderrContains(t, r, "divide by zero")

	// Without -i the same program stops at the error.
	r = runBuiltinStdin(t, "bc", "1/0\n2+2\n")
	assertExitCode(t, r, 1)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want none", r.stdout)
	}
}

func TestBcDigitsAboveIbaseKeepTheirValue(t *testing.T) {
	// Only letters clamp to ibase-1; 0-9 keep their face value (Toybox).
	r := runBuiltinStdin(t, "bc", "ibase=2\n12\nibase=5\n77\nibase=2\n11A\n")
	assertExitCode(t, r, 0)
	if r.stdout != "4\n42\n7\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "4\n42\n7\n")
	}
}

func TestBcSqrtZeroUsesScaleSetting(t *testing.T) {
	// Toybox tests for zero before it bumps the scale, so sqrt(0.00) has the
	// scale setting rather than the operand's scale.
	r := runBuiltinStdin(t, "bc", "scale(0.00)\nscale(sqrt(0.00))\nscale=3\nscale(sqrt(0))\n")
	assertExitCode(t, r, 0)
	if r.stdout != "2\n0\n3\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "2\n0\n3\n")
	}
}

func TestBcSpecialAssignmentValue(t *testing.T) {
	// Assigning scale yields the right-hand side as written, not the
	// truncated integer that was stored.
	r := runBuiltinStdin(t, "bc", "(scale = 2.7)\nscale\n(scale += 1.5)\nscale\n")
	assertExitCode(t, r, 0)
	if r.stdout != "2.7\n2\n3.5\n3\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "2.7\n2\n3.5\n3\n")
	}
}

func TestBcParenthesizedAssignmentPrints(t *testing.T) {
	// A bare assignment is silent, but one that starts with "(" prints.
	r := runBuiltinStdin(t, "bc", "x=1\nx=2\n(x) = 3\nx\n")
	assertExitCode(t, r, 0)
	if r.stdout != "3\n3\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "3\n3\n")
	}
}

func TestBcBodyOnNextLine(t *testing.T) {
	// Newlines before a body are skipped (Toybox), so the statement on the
	// next line is the body.
	r := runBuiltinStdin(t, "bc", "if (1)\n2+2\nwhile (0)\n1\nfor (i = 0;\ni < 2;\ni++) i\nprint 1,\n2\n")
	assertExitCode(t, r, 0)
	if r.stdout != "4\n0\n1\n12" {
		t.Errorf("stdout = %q, want %q", r.stdout, "4\n0\n1\n12")
	}
}

func TestBcDynamicScoping(t *testing.T) {
	// Autos and parameters are visible to functions called from that frame,
	// as in Toybox and GNU bc.
	r := runBuiltinStdin(t, "bc", "define g(){return(a)}\ndefine f(){auto a\na=5\nreturn(g())}\na=1\nf()\n")
	assertExitCode(t, r, 0)
	if r.stdout != "5\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "5\n")
	}
	r = runBuiltinStdin(t, "bc", "define g(){a=9}\ndefine f(){auto a\na=5\ng()\nreturn(a)}\na=1\nprint f(), \" \", a, \"\\n\"\n")
	assertExitCode(t, r, 0)
	// The bare g() call inside f() prints its (zero) return value first.
	if r.stdout != "0\n9 1\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "0\n9 1\n")
	}
}

func TestBcZeroFractionInOtherBaseLosesScale(t *testing.T) {
	// Toybox's non-decimal parser returns a bare zero for a zero result.
	// Note ibase=A (10) rather than ibase=10, which would be read in base 16.
	r := runBuiltinStdin(t, "bc", "ibase=16\nscale(.0001)\nscale(0.0)\nibase=A\nscale(0.0)\n")
	assertExitCode(t, r, 0)
	if r.stdout != "0\n0\n1\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "0\n0\n1\n")
	}
}

func TestBcElseAcrossUnits(t *testing.T) {
	// A file (or -) is one unit, so an else may follow on its own line.
	dir := t.TempDir()
	writeFile(t, dir, "e.bc", "if (0) 1\nelse 2\n")
	r := runBuiltinStdinIn(t, dir, "bc", "", "e.bc")
	assertExitCode(t, r, 0)
	if r.stdout != "2\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "2\n")
	}

	// Standard input is parsed a line at a time, so there the else is a new
	// statement and fails. Documented in docs/plans/bc.md.
	r = runBuiltinStdin(t, "bc", "if (0) 1\nelse 2\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unexpected")
}
