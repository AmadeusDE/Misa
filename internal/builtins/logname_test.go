package builtins

import "testing"

func TestLognameOutput(t *testing.T) {
	r := runBuiltin(t, "logname")
	assertExitCode(t, r, 0)
	if r.stdout == "" {
		t.Error("expected non-empty output")
	}
}

func TestLognameRegistered(t *testing.T) {
	_, ok := Lookup("logname")
	if !ok {
		t.Fatal("logname not registered")
	}
}
