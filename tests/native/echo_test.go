package native

import "testing"

func TestEcho_Basic(t *testing.T) {
	r := run(t, "echo", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestEcho_MultipleArgs(t *testing.T) {
	r := run(t, "echo", "hello", "world")
	wantSuccess(t, r)
	wantOut(t, r, "hello world\n")
}

func TestEcho_Empty(t *testing.T) {
	r := run(t, "echo")
	wantSuccess(t, r)
	wantOut(t, r, "\n")
}

func TestEcho_NoNewline(t *testing.T) {
	r := run(t, "echo", "-n", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello")
}

func TestEcho_EscapeTab(t *testing.T) {
	r := run(t, "echo", "-e", `a\tb`)
	wantSuccess(t, r)
	wantOut(t, r, "a\tb\n")
}

func TestEcho_EscapeNewline(t *testing.T) {
	r := run(t, "echo", "-e", `a\nb`)
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\n")
}

func TestEcho_EscapeBackslash(t *testing.T) {
	r := run(t, "echo", "-e", `a\\b`)
	wantSuccess(t, r)
	wantOut(t, r, "a\\b\n")
}

func TestEcho_EscapeC(t *testing.T) {
	r := run(t, "echo", "-e", `hello\cworld`)
	wantSuccess(t, r)
	wantOut(t, r, "hello")
}

func TestEcho_EscapeOctal(t *testing.T) {
	r := run(t, "echo", "-e", `\0101`)
	wantSuccess(t, r)
	wantOut(t, r, "A\n")
}

func TestEcho_EscapeOctalAllButC(t *testing.T) {
	r := run(t, "echo", "-e", `\a\b\f\n\r\t\v\\\0123`)
	wantSuccess(t, r)
	wantOut(t, r, "\a\b\f\n\r\t\v\\S\n")
}

func TestEcho_EscapeHex(t *testing.T) {
	r := run(t, "echo", "-e", `\x41`)
	wantSuccess(t, r)
	wantOut(t, r, "A\n")
}

func TestEcho_DashDash(t *testing.T) {
	r := run(t, "echo", "--")
	wantSuccess(t, r)
	wantOut(t, r, "--\n")
}

func TestEcho_DashENotAlone(t *testing.T) {
	r := run(t, "echo", "-en", "hi")
	wantSuccess(t, r)
	wantOut(t, r, "hi")
}

func TestEcho_NotAFlag(t *testing.T) {
	r := run(t, "echo", "-x")
	wantSuccess(t, r)
	wantOut(t, r, "-x\n")
}
