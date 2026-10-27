package native

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func runNlBuiltin(t *testing.T, stdin string, args ...string) (code int, stdout string, stderr string) {
	t.Helper()
	cmd, ok := builtins.Lookup("nl")
	if !ok {
		t.Fatalf("builtin nl not registered")
	}
	var stdoutBuf, stderrBuf strings.Builder
	ctx := &builtins.Context{
		Dir:    t.TempDir(),
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Stdin:  strings.NewReader(stdin),
	}
	code = cmd.Run(ctx)
	return code, stdoutBuf.String(), stderrBuf.String()
}

func TestNlOverflow(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		args       []string
		wantExitOK bool
		output     string
	}{
		{
			name:       "nl with INTMAX_MAX and INTMAX_MIN, 2 lines (should succeed)",
			input:      "a\nb\n",
			args:       []string{"-v9223372036854775807", "-i-9223372036854775808"},
			wantExitOK: true,
			output:     "9223372036854775807\ta\n    -1\tb\n",
		},
		{
			name:       "nl with INTMAX_MAX and INTMAX_MIN, 3 lines (should fail)",
			input:      "a\nb\nc\n",
			args:       []string{"-v9223372036854775807", "-i-9223372036854775808"},
			wantExitOK: false,
			output:     "9223372036854775807\ta\n    -1\tb\n",
		},
		{
			name:       "nl with INTMAX_OFLOW (should fail at parse time)",
			input:      "",
			args:       []string{"-v9223372036854775808"},
			wantExitOK: false,
			output:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runNlBuiltin(t, tt.input, tt.args...)

			if tt.wantExitOK && code != 0 {
				t.Errorf("expected success but got exit code %d, stderr: %s", code, stderr)
			}
			if !tt.wantExitOK && code == 0 {
				t.Errorf("expected error but got success")
			}

			if tt.output != "" && !strings.Contains(stdout, strings.TrimSuffix(tt.output, "\n")) {
				t.Errorf("output mismatch:\nwant substring: %q\ngot:  %q", tt.output, stdout)
			}
		})
	}
}

func TestNlBasic(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		args   []string
		output string
	}{
		{
			name:   "nl basic numbering",
			input:  "a\nb\nc\n",
			args:   []string{},
			output: "     1\ta\n     2\tb\n     3\tc\n",
		},
		{
			name:   "nl with -v",
			input:  "a\nb\n",
			args:   []string{"-v5"},
			output: "     5\ta\n     6\tb\n",
		},
		{
			name:   "nl with -i",
			input:  "a\nb\nc\n",
			args:   []string{"-i2"},
			output: "     1\ta\n     3\tb\n     5\tc\n",
		},
		{
			name:   "nl with -n ln (left justified)",
			input:  "a\nb\n",
			args:   []string{"-n", "ln"},
			output: "1     \ta\n2     \tb\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runNlBuiltin(t, tt.input, tt.args...)
			if code != 0 {
				t.Errorf("unexpected error: %s", stderr)
			}

			if stdout != tt.output {
				t.Errorf("output mismatch:\nwant: %q\ngot:  %q", tt.output, stdout)
			}
		})
	}
}
