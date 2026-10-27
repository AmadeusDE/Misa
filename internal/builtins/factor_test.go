package builtins

import "testing"

func TestFactorPrime(t *testing.T) {
	r := runBuiltin(t, "factor", "7")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "7: 7\n")
}

func TestFactorComposite(t *testing.T) {
	r := runBuiltin(t, "factor", "12")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "12: 2 2 3\n")
}

func TestFactorOne(t *testing.T) {
	r := runBuiltin(t, "factor", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1:\n")
}

func TestFactorTwo(t *testing.T) {
	r := runBuiltin(t, "factor", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "2: 2\n")
}

func TestFactorLarge(t *testing.T) {
	r := runBuiltin(t, "factor", "100")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "100: 2 2 5 5\n")
}

func TestFactorMultiple(t *testing.T) {
	r := runBuiltin(t, "factor", "6", "15")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "6: 2 3\n15: 3 5\n")
}

func TestFactorStdin(t *testing.T) {
	r := runBuiltinStdin(t, "factor", "24\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "24: 2 2 2 3\n")
}

func TestFactorInvalid(t *testing.T) {
	r := runBuiltin(t, "factor", "abc")
	assertExitCode(t, r, 1)
}

func TestFactorPower(t *testing.T) {
	r := runBuiltin(t, "factor", "128")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "128: 2 2 2 2 2 2 2\n")
}

func TestFactorLargePrime(t *testing.T) {
	r := runBuiltin(t, "factor", "997")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "997: 997\n")
}

func TestFactorLeadingWhitespace(t *testing.T) {
	r := runBuiltin(t, "factor", "  0")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "0:\n")
}

func TestFactorPlusPrefix(t *testing.T) {
	r := runBuiltin(t, "factor", "+1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1:\n")
}

func TestFactorWhitespacePlusPrefix(t *testing.T) {
	r := runBuiltin(t, "factor", " +2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "2: 2\n")
}

func TestFactorExponents(t *testing.T) {
	r := runBuiltin(t, "factor", "-h", "128")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "128: 2^7\n")
}

func TestFactorExponentsLong(t *testing.T) {
	r := runBuiltin(t, "factor", "--exponents", "12")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "12: 2^2 3\n")
}

func TestFactorExponentsMixed(t *testing.T) {
	r := runBuiltin(t, "factor", "-hh", "1234", "10240")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1234: 2 617\n10240: 2^11 5\n")
}

func TestFactorNegativeArgTip(t *testing.T) {
	r := runBuiltin(t, "factor", "-1")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Usage: factor")
}

func TestFactorNullTrim(t *testing.T) {
	r := runBuiltinStdin(t, "factor", "42\x00\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "42: 2 3 7\n")
}
