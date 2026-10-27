package builtins

import (
	"os"
	"strings"
	"testing"
)

func TestEnvDefault(t *testing.T) {
	r := runBuiltin(t, "env")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "=") {
		t.Errorf("expected KEY=VALUE output, got %q", r.stdout)
	}
}

func TestEnvContainsPath(t *testing.T) {
	r := runBuiltin(t, "env")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "PATH=")
}

func TestEnvUnset(t *testing.T) {
	key := "MISA_TEST_ENV_VAR_12345"
	os.Setenv(key, "hello")
	defer os.Unsetenv(key)

	r := runBuiltin(t, "env", "-u", key)
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, key+"=") {
		t.Errorf("expected %s to be unset, but found it in output", key)
	}
}

func TestEnvIgnore(t *testing.T) {
	r := runBuiltin(t, "env", "-i")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestEnvNull(t *testing.T) {
	r := runBuiltin(t, "env", "-0")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "\x00") {
		t.Error("expected null-separated output with -0")
	}
}
