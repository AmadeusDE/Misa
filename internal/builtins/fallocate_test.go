package builtins

import (
	"strings"
	"testing"
)

func TestAtolx(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"0", 0}, {"123", 123}, {" 12 ", 12}, {"1k", 1024}, {"2K", 2048}, {"1m", 1 << 20},
		{"1M", 1 << 20}, {"3g", 3 << 30}, {"1t", 1 << 40}, {"1p", 1 << 50}, {"1e", 1 << 60},
		{"4w", 8}, {"2b", 1024}, {"8c", 8}, {"1kd", 1000}, {"2md", 2000000},
		{"0x10", 16}, {"-1", -1}, {"+7", 7},
	} {
		got, err := atolx(tc.in)
		if err != nil {
			t.Errorf("atolx(%q) failed: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("atolx(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
	for _, in := range []string{"", "abc", "12x", "1k5", "1q", "0x", "--1", "12 34"} {
		if got, err := atolx(in); err == nil {
			t.Errorf("atolx(%q) = %d, want an error", in, got)
		}
	}
}

func TestFallocateUsageErrors(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing length", []string{"file"}, "-l LEN is required"},
		{"missing operand", []string{"-l", "10"}, "missing operand"},
		{"extra operand", []string{"-l", "10", "a", "b"}, "extra operand 'b'"},
		{"bad length", []string{"-l", "abc", "file"}, "not an integer: abc"},
		{"bad offset", []string{"-o", "xyz", "-l", "10", "file"}, "not an integer: xyz"},
		{"unknown option", []string{"-q", "-l", "10", "file"}, "unknown option -q"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinIn(t, dir, "fallocate", tc.args...)
			assertExitCode(t, r, 1)
			if !strings.Contains(r.stderr, tc.want) {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.want)
			}
		})
	}
}

func TestFallocateHelp(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "fallocate", "--help")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "Usage:") || !strings.Contains(r.stdout, "--length") {
		t.Errorf("--help = %q", r.stdout)
	}
}
