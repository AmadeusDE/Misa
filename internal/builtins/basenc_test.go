// Behaviour adapted from uutils coreutils basenc (MIT).
package builtins

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestBasenc(t *testing.T) {
	for _, tc := range []struct {
		name, input, output, stderr string
		args                        []string
		code                        int
	}{
		{name: "base64", args: []string{"--base64"}, input: "to>be?", output: "dG8+YmU/\n"},
		{name: "base64url", args: []string{"--base64url"}, input: "to>be?", output: "dG8-YmU_\n"},
		{name: "base32", args: []string{"--base32"}, input: "abc", output: "MFRGG===\n"},
		{name: "base32hex", args: []string{"--base32hex"}, input: "abc", output: "C5H66===\n"},
		{name: "base16", args: []string{"--base16"}, input: "Hello!", output: "48656C6C6F21\n"},
		{name: "msbf", args: []string{"--base2msbf"}, input: "msbf", output: "01101101011100110110001001100110\n"},
		{name: "lsbf", args: []string{"--base2lsbf"}, input: "lsbf", output: "00110110110011100100011001100110\n"},
		{name: "z85", args: []string{"--z85"}, input: "Hello, World", output: "nm=QNz.92jz/PV8\n"},
		{name: "base58", args: []string{"--base58"}, input: "Hello, World!", output: "72k1xXWG59fYdzSNoA\n"},
		{name: "missing_encoding", code: 1, stderr: "basenc: missing encoding type\n"},
		{name: "last_encoding", args: []string{"--base64", "--base32", "--base64", "-w0"}, input: "abc", output: "YWJj"},
		{name: "wrap", args: []string{"--base16", "-w", "4"}, input: "abcde", output: "6162\n6364\n65\n"},
		{name: "wrap_exact", args: []string{"--base16", "-w2"}, input: "ab", output: "61\n62\n"},
		{name: "wrap_invalid", args: []string{"--base16", "-w", "x"}, code: 1, stderr: "basenc: invalid wrap size: 'x'\n"},
		{name: "wrap_negative", args: []string{"--base16", "-w-1"}, code: 1, stderr: "basenc: invalid wrap size: '-1'\n"},
		{name: "base16_lowercase", args: []string{"--base16", "-di"}, input: "6a:6B", output: "jk"},
		{name: "base16_prefix", args: []string{"--base16", "-d"}, input: "6162!63", output: "ab", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "base16_truncated", args: []string{"--base16", "-d"}, input: "616", output: "a", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "base32_prefix", args: []string{"--base32", "-d"}, input: "MFRGGZDF=", output: "abcde", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "base32hex_prefix", args: []string{"--base32hex", "-d"}, input: "VNC0FKD5W", output: "\xfd\xd8\x07\xd1\xa5", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "base32hex_truncated", args: []string{"--base32hex", "-d"}, input: "CPNMUO", output: "foo", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "base32_autopad", args: []string{"--base32", "-d"}, input: "MFRGGZDF\nMFRGG", output: "abcdeabc"},
		{name: "base64_prefix", args: []string{"--base64", "-d"}, input: "YWJj!ZA==", output: "abc", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "base64_autopad", args: []string{"--base64", "-D"}, input: "YWI", output: "ab"},
		{name: "base64_concatenated", args: []string{"--base64", "-d"}, input: "YQ==Yg==", output: "ab"},
		{name: "base64_ignore", args: []string{"--base64", "--decode", "--ignore"}, input: "Y!W Jj\n", output: "abc"},
		{name: "binary_prefix", args: []string{"--base2msbf", "-d"}, input: "011000011", output: "a", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "z85_length", args: []string{"--z85"}, input: "123", code: 1, stderr: "basenc: error: invalid input (length must be multiple of 4 characters)\n"},
		{name: "z85_overflow", args: []string{"--z85", "-d"}, input: "##########", code: 1, stderr: "basenc: error: invalid input\n"},
		{name: "z85_decode", args: []string{"--z85", "-d"}, input: "f!$Kwh8WxM", output: "12345678"},
		{name: "base58_leading_zero", args: []string{"--base58"}, input: "\x00\x00\x01", output: "112\n"},
		{name: "base58_invalid", args: []string{"--base58", "-d"}, input: "0", code: 1, stderr: "basenc: error: invalid input\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "basenc", tc.input, tc.args...)
			assertExitCode(t, r, tc.code)
			assertStdout(t, r, tc.output)
			if r.stderr != tc.stderr {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.stderr)
			}
		})
	}
}

func TestBasencRoundTrip(t *testing.T) {
	for _, encoding := range []string{"base64", "base64url", "base32", "base32hex", "base16", "base2msbf", "base2lsbf", "z85", "base58"} {
		t.Run(encoding, func(t *testing.T) {
			for _, input := range []string{"", "\x00\x00\x00\x00", "\x00\x01\xfe\xff", strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 52)} {
				enc := runBuiltinStdin(t, "basenc", input, "--"+encoding)
				assertExitCode(t, enc, 0)
				dec := runBuiltinStdin(t, "basenc", enc.stdout, "--"+encoding, "-d")
				assertExitCode(t, dec, 0)
				assertStdout(t, dec, input)
			}
		})
	}
}

func TestBasencLargeBase58(t *testing.T) {
	input := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 50)
	r := runBuiltinStdin(t, "basenc", input, "--base58")
	assertExitCode(t, r, 0)
	if !strings.HasSuffix(r.stdout, "ZNRRacEnhrY83ZEYkpwWVZNFK5DFRasr\nw693NsNGtiQ9fYAj\n") {
		t.Fatal("large base58 output suffix mismatch")
	}
}

func TestBasencFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "input", "foo")
	r := runBuiltinIn(t, dir, "basenc", "input", "--base64")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "Zm9v\n")
}

type basencFailWriter struct{}

func (basencFailWriter) Write([]byte) (int, error) { return 0, errors.New("No space left on device") }

func TestBasencWriteError(t *testing.T) {
	var stderr bytes.Buffer
	ctx := &Context{Args: []string{"--base16"}, Stdin: strings.NewReader("Hello, World!"), Stdout: basencFailWriter{}, Stderr: &stderr}
	cmd, ok := Lookup("basenc")
	if !ok {
		t.Fatal("basenc not registered")
	}
	if code := cmd.Run(ctx); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if got := stderr.String(); got != "basenc: No space left on device\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestBasencStreamingDecode(t *testing.T) {
	reader, writer := io.Pipe()
	var stdout, stderr bytes.Buffer
	ctx := &Context{Args: []string{"--base32", "-d"}, Stdin: reader, Stdout: &stdout, Stderr: &stderr}
	go func() {
		for _, part := range []string{"MFR", "GGZDF", "MFR", "GG"} {
			_, _ = io.WriteString(writer, part)
		}
		writer.Close()
	}()
	defer reader.Close()
	cmd, ok := Lookup("basenc")
	if !ok {
		t.Fatal("basenc not registered")
	}
	if code := cmd.Run(ctx); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.String() != "abcdeabc" {
		t.Errorf("stdout = %q", stdout.String())
	}
}
