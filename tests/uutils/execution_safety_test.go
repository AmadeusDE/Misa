package uutils

import "testing"

func TestExternalExecutionRefused(t *testing.T) {
	for _, v := range []struct {
		cmd  string
		args []string
	}{
		{"env", []string{"echo", "hello"}}, {"nice", []string{"echo"}}, {"timeout", []string{"1", "echo"}}, {"sort", []string{"--compress-program=gzip"}},
	} {
		if !isSandboxUnsafe(v.cmd, v.args) {
			t.Errorf("%s %q permits host execution", v.cmd, v.args)
		}
	}
}

func TestWrapperValidationOnly(t *testing.T) {
	for _, v := range []struct {
		cmd  string
		args []string
	}{
		{"env", []string{"--help"}}, {"env", []string{"--definitely-invalid"}}, {"env", []string{"-i", "=xyz"}},
		{"nice", []string{"--invalid"}}, {"nice", []string{"-n", "-2+4", "true"}},
		{"timeout", []string{"--definitely-invalid"}}, {"timeout", []string{"", ""}},
	} {
		if isSandboxUnsafe(v.cmd, v.args) {
			t.Errorf("safe validation rejected: %s %q", v.cmd, v.args)
		}
	}
}
