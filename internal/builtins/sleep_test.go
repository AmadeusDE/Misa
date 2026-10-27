package builtins

import "testing"

func TestSleepZero(t *testing.T) {
	r := runBuiltin(t, "sleep", "0")
	assertExitCode(t, r, 0)
}

func TestSleepInvalid(t *testing.T) {
	r := runBuiltin(t, "sleep", "abc")
	assertExitCode(t, r, 1)
}

func TestSleepMissing(t *testing.T) {
	r := runBuiltin(t, "sleep")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "the following required arguments were not provided")
}

func TestSleepSuffix(t *testing.T) {
	r := runBuiltin(t, "sleep", "0s")
	assertExitCode(t, r, 0)
}

func TestSleepMultiple(t *testing.T) {
	r := runBuiltin(t, "sleep", "0", "0")
	assertExitCode(t, r, 0)
}

func TestParseSleepDuration(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"0", true},
		{"1", true},
		{"0.5", true},
		{"1s", true},
		{"1m", true},
		{"1h", true},
		{"1d", true},
		{"abc", false},
		{"", false},
		{"-1", false},
	}
	for _, tt := range tests {
		_, err := parseSleepDuration(tt.input)
		if (err == nil) != tt.ok {
			t.Errorf("parseSleepDuration(%q): got err=%v, want ok=%v", tt.input, err, tt.ok)
		}
	}
}
