// Behaviour adapted from toybox toys/other/usleep.c (0BSD, commit 6a0c0ef).
package builtins

import (
	"testing"
	"time"
)

func TestUsleepAcceptsMicroseconds(t *testing.T) {
	r := runBuiltin(t, "usleep", "0")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestUsleepActuallySleeps(t *testing.T) {
	const delay = 20 * time.Millisecond
	start := time.Now()
	r := runBuiltin(t, "usleep", "20000")
	assertExitCode(t, r, 0)
	if elapsed := time.Since(start); elapsed < delay {
		t.Errorf("usleep 20000 slept %v, want at least %v", elapsed, delay)
	}
}

func TestUsleepInvalidArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"missing", nil, "usleep: missing operand"},
		{"two operands", []string{"1", "2"}, `usleep: unexpected argument "2"`},
		{"not a number", []string{"abc"}, `usleep: invalid number "abc"`},
		{"fractional", []string{"1.5"}, `usleep: invalid number "1.5"`},
		{"negative", []string{"--", "-1"}, `usleep: invalid number "-1"`},
		{"overflow", []string{"99999999999999999999"}, `usleep: invalid number "99999999999999999999"`},
		{"too large for a duration", []string{"9223372036854775807"}, `usleep: time interval out of range "9223372036854775807"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "usleep", tc.args...)
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			if r.stderr != tc.wantErr+"\n" {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.wantErr+"\n")
			}
		})
	}
}

func TestUsleepOptionLikeOperand(t *testing.T) {
	// "-1" without "--" is an option, not an operand; either way it is a usage
	// error, never a silent zero-length sleep.
	r := runBuiltin(t, "usleep", "-1")
	assertExitCode(t, r, 1)
	if r.stderr == "" {
		t.Error("stderr is empty, want a usage error")
	}
}

func TestUsleepUnsupportedOption(t *testing.T) {
	r := runBuiltin(t, "usleep", "--fast")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "unknown option")
}

func TestUsleepHelp(t *testing.T) {
	r := runBuiltin(t, "usleep", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage:")

	r = runBuiltin(t, "usleep", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "0.1.0")
}
