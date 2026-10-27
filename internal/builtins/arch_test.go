package builtins

import (
	"runtime"
	"testing"
)

func TestArchRegistered(t *testing.T) {
	_, ok := Lookup("arch")
	if !ok {
		t.Fatal("arch not registered")
	}
}

func TestArchOutput(t *testing.T) {
	r := runBuiltin(t, "arch")
	assertExitCode(t, r, 0)
	got := trimTrailingNewline(r.stdout)
	want := runtime.GOARCH
	archMap := map[string]string{
		"amd64": "x86_64",
		"386":   "i686",
		"arm64": "aarch64",
		"arm":   "armv7l",
	}
	if mapped, ok := archMap[want]; ok {
		want = mapped
	}
	if got != want {
		t.Errorf("arch = %q, want %q", got, want)
	}
}

func TestArchHelp(t *testing.T) {
	r := runBuiltin(t, "arch", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "architecture")
}

func TestArchDoubleHyphen(t *testing.T) {
	r := runBuiltin(t, "arch", "--")
	assertExitCode(t, r, 0)
	got := trimTrailingNewline(r.stdout)
	want := runtime.GOARCH
	archMap := map[string]string{
		"amd64": "x86_64",
		"386":   "i686",
		"arm64": "aarch64",
		"arm":   "armv7l",
	}
	if mapped, ok := archMap[want]; ok {
		want = mapped
	}
	if got != want {
		t.Errorf("arch -- = %q, want %q", got, want)
	}
}

func trimTrailingNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}
