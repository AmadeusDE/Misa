package vi

import (
	"strings"
	"testing"
)

func TestShellCommandsUseInvocationEnvironment(t *testing.T) {
	t.Setenv("MISA_VI_ENV_TEST", "process")
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{"explicit", []string{"MISA_VI_ENV_TEST=invocation"}, "invocation"},
		{"empty", []string{}, "unset"},
		{"inherited", nil, "process"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := append([]string{}, tc.env...)
			if tc.env == nil {
				env = nil
			}
			e := newWithEnv("", t.TempDir(), env)
			if len(env) > 0 {
				env[0] = "MISA_VI_ENV_TEST=mutated"
			}
			cmd := `printf '%s' "${MISA_VI_ENV_TEST-unset}"`
			e.shellCmd(cmd)
			if e.statusMsg != tc.want {
				t.Errorf("shell: %q", e.statusMsg)
			}
			e.readCmd("!" + cmd)
			if e.lines[len(e.lines)-1] != tc.want {
				t.Errorf("read: %q", e.lines)
			}
			e.filterRange(0, 0, cmd)
			if strings.TrimSpace(e.lines[0]) != tc.want {
				t.Errorf("filter: %q", e.lines)
			}
		})
	}
}
