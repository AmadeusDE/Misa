package builtins

import "testing"

func TestBasename(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"/usr/bin/foo"}, "foo\n"},
		{[]string{"/usr/bin/"}, "bin\n"},
		{[]string{"foo.go", ".go"}, "foo\n"},
		{[]string{"/usr/bin/foo", ".go"}, "foo\n"},
		{[]string{"-s", ".go", "foo.go"}, "foo\n"},
		{[]string{"-s", ".go", "foo.go", "bar.go"}, "foo\nbar\n"},
		{[]string{"/"}, "/\n"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			r := runBuiltin(t, "basename", tt.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestBasenameMissing(t *testing.T) {
	r := runBuiltin(t, "basename")
	assertExitCode(t, r, 1)
}

func TestDirname(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"/usr/bin/foo"}, "/usr/bin\n"},
		{[]string{"foo"}, ".\n"},
		{[]string{"/"}, "/\n"},
		{[]string{"/usr/bin/foo", "/etc/bar"}, "/usr/bin\n/etc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			r := runBuiltin(t, "dirname", tt.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestDirnameMissing(t *testing.T) {
	r := runBuiltin(t, "dirname")
	assertExitCode(t, r, 1)
}
