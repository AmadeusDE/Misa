package native

import "testing"

func TestExpr_Add(t *testing.T) {
	r := run(t, "expr", "2", "+", "3")
	wantSuccess(t, r)
	wantOut(t, r, "5\n")
}

func TestExpr_Sub(t *testing.T) {
	r := run(t, "expr", "10", "-", "3")
	wantSuccess(t, r)
	wantOut(t, r, "7\n")
}

func TestExpr_Mul(t *testing.T) {
	r := run(t, "expr", "4", "*", "5")
	wantSuccess(t, r)
	wantOut(t, r, "20\n")
}

func TestExpr_Div(t *testing.T) {
	r := run(t, "expr", "10", "/", "3")
	wantSuccess(t, r)
	wantOut(t, r, "3\n")
}

func TestExpr_Mod(t *testing.T) {
	r := run(t, "expr", "10", "%", "3")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestExpr_Compare(t *testing.T) {
	r := run(t, "expr", "5", ">", "3")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestExpr_CompareFalse(t *testing.T) {
	r := run(t, "expr", "3", ">", "5")
	wantCode(t, r, 1)
	wantOut(t, r, "0\n")
}

func TestExpr_Equal(t *testing.T) {
	r := run(t, "expr", "5", "=", "5")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestExpr_NotEqual(t *testing.T) {
	r := run(t, "expr", "5", "!=", "3")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestExpr_StringLength(t *testing.T) {
	r := run(t, "expr", "length", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "5\n")
}

func TestExpr_MultibyteLength(t *testing.T) {
	r := run(t, "expr", "length", "🦀🚀🎯")
	wantSuccess(t, r)
	wantOut(t, r, "3\n")
}

func TestExpr_Substr(t *testing.T) {
	r := run(t, "expr", "substr", "hello", "2", "3")
	wantSuccess(t, r)
	wantOut(t, r, "ell\n")
}

func TestExpr_Match(t *testing.T) {
	r := run(t, "expr", "match", "hello", "hel")
	wantSuccess(t, r)
	wantOut(t, r, "3\n")
}

func TestExpr_Index(t *testing.T) {
	r := run(t, "expr", "index", "hello", "le")
	wantSuccess(t, r)
	wantOut(t, r, "2\n")
}

func TestExpr_Or(t *testing.T) {
	r := run(t, "expr", "0", "|", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestExpr_And(t *testing.T) {
	r := run(t, "expr", "hello", "&", "world")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestExpr_DivByZero(t *testing.T) {
	r := run(t, "expr", "1", "/", "0")
	wantCode(t, r, 2)
}

func TestExpr_Parens(t *testing.T) {
	r := run(t, "expr", "(", "2", "+", "3", ")", "*", "4")
	wantSuccess(t, r)
	wantOut(t, r, "20\n")
}

func TestExpr_UnaryPlus(t *testing.T) {
	r := run(t, "expr", "+", "5", "+", "+", "3")
	wantSuccess(t, r)
	wantOut(t, r, "8\n")
}

func TestExpr_SyntaxError_MissingArg(t *testing.T) {
	r := run(t, "expr", "2", "+")
	wantCode(t, r, 2)
	wantErrContains(t, r, "syntax error: missing argument after '+'")
}

func TestExpr_SyntaxError_Parens(t *testing.T) {
	r := run(t, "expr", "(", "2")
	wantCode(t, r, 2)
	wantErrContains(t, r, "syntax error: expecting ')' after '2'")

	r2 := run(t, "expr", "(", "2", "a")
	wantCode(t, r2, 2)
	wantErrContains(t, r2, "syntax error: expecting ')' instead of 'a'")
}

func TestExpr_BRE_QuantifierChaining(t *testing.T) {
	r := run(t, "expr", "aa", ":", "a\\{1\\}\\{1\\}")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestExpr_BRE_Backreference(t *testing.T) {
	r := run(t, "expr", "aabcd", ":", "\\(a\\)\\1bcd")
	wantSuccess(t, r)
	wantOut(t, r, "a\n")
}
