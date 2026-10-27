package builtins

import "testing"

func TestTtyOutput(t *testing.T) {
	r := runBuiltin(t, "tty")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "not a tty\n")
}

func TestTtyRegistered(t *testing.T) {
	_, ok := Lookup("tty")
	if !ok {
		t.Fatal("tty not registered")
	}
}
