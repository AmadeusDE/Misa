package builtins

import "testing"

func TestExprAdd(t *testing.T) {
	r := runBuiltin(t, "expr", "1", "+", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestExprSubtract(t *testing.T) {
	r := runBuiltin(t, "expr", "10", "-", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "7\n")
}

func TestExprMultiply(t *testing.T) {
	r := runBuiltin(t, "expr", "3", "*", "4")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "12\n")
}

func TestExprDivide(t *testing.T) {
	r := runBuiltin(t, "expr", "10", "/", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestExprModulo(t *testing.T) {
	r := runBuiltin(t, "expr", "10", "%", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}

func TestExprDivZero(t *testing.T) {
	r := runBuiltin(t, "expr", "5", "/", "0")
	assertExitCode(t, r, 2)
}

func TestExprCompareEqual(t *testing.T) {
	r := runBuiltin(t, "expr", "5", "=", "5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}

func TestExprCompareNotEqual(t *testing.T) {
	r := runBuiltin(t, "expr", "5", "!=", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}

func TestExprCompareLess(t *testing.T) {
	r := runBuiltin(t, "expr", "3", "<", "5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}

func TestExprCompareGreater(t *testing.T) {
	r := runBuiltin(t, "expr", "5", ">", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}

func TestExprCompareFalse(t *testing.T) {
	r := runBuiltin(t, "expr", "3", ">", "5")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "0\n")
}

func TestExprLength(t *testing.T) {
	r := runBuiltin(t, "expr", "length", "hello")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "5\n")
}

func TestExprSubstr(t *testing.T) {
	r := runBuiltin(t, "expr", "substr", "hello", "2", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "ell\n")
}

func TestExprMatch(t *testing.T) {
	r := runBuiltin(t, "expr", "match", "hello123", "[a-z]*")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "5\n")
}

func TestExprOr(t *testing.T) {
	r := runBuiltin(t, "expr", "hello", "|", "world")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestExprOrZero(t *testing.T) {
	r := runBuiltin(t, "expr", "0", "|", "world")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "world\n")
}

func TestExprZeroResult(t *testing.T) {
	r := runBuiltin(t, "expr", "0", "+", "0")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "0\n")
}

func TestExprMissing(t *testing.T) {
	r := runBuiltin(t, "expr")
	assertExitCode(t, r, 2)
}

func TestExprStringCompare(t *testing.T) {
	r := runBuiltin(t, "expr", "abc", "<", "def")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}
