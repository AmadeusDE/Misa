package builtins

import (
	"bytes"
	"testing"
)

func TestXXDDumpAndReverse(t *testing.T) {
	var out bytes.Buffer
	xxdDump(&out, []byte("hello"), xxdOptions{cols: 4, group: 2})
	if got, want := out.String(), "00000000: 6865 6c6c  hell\n00000004: 6f         o\n"; got != want {
		t.Fatalf("dump = %q, want %q", got, want)
	}
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if code := xxdReverse(ctx, []byte("00000000: 6865 6c6c  he\n00000004: 6f         o\n"), xxdOptions{cols: 16, group: 2}); code != 0 {
		t.Fatalf("reverse exit = %d", code)
	}
	if got := ctx.Stdout.(*bytes.Buffer).String(); got != "hello" {
		t.Fatalf("reverse = %q", got)
	}
}

func TestXXDPlain(t *testing.T) {
	var out bytes.Buffer
	xxdDump(&out, []byte{0, 0xff}, xxdOptions{cols: 30, plain: true})
	if got := out.String(); got != "00ff\n" {
		t.Fatalf("plain = %q", got)
	}
}

func TestXXDReverseRejectsHostileOffsets(t *testing.T) {
	for _, in := range []string{"00000004: 6f\n-0000002: 41\n", "7fffffffffffffff: 41\n", "ffffffff: 41\n"} {
		ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
		if code := xxdReverse(ctx, []byte(in), xxdOptions{cols: 16, group: 2}); code == 0 {
			t.Errorf("%q: exit 0, want an error", in)
		}
	}
	ctx := &Context{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if code := xxdReverse(ctx, []byte("00000002: 41\n"), xxdOptions{cols: 16, group: 2}); code != 0 || ctx.Stdout.(*bytes.Buffer).String() != "\x00\x00A" {
		t.Errorf("small gap: exit %d, out %q", code, ctx.Stdout.(*bytes.Buffer).String())
	}
}
