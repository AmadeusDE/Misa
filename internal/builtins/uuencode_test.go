// Behaviour adapted from toybox uuencode.c (0BSD).
package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUUEncodeFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input")
	if err := os.WriteFile(path, []byte("abc"), 0751); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0751); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	ctx := &Context{Dir: dir, Args: []string{"input", "encoded name"}, Stdout: &out, Stderr: &stderr}
	if code := runUUEncode(ctx); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	if got, want := out.String(), "begin 751 encoded name\n#86)C\nend\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUUEncodeRoundTrip(t *testing.T) {
	for _, base64 := range []bool{false, true} {
		for _, size := range []int{0, 1, 2, 3, 44, 45, 46, 56, 57, 58, 256} {
			input := make([]byte, size)
			for i := range input {
				input[i] = byte(i)
			}
			var encoded, decoded, stderr bytes.Buffer
			args := []string{"test"}
			if base64 {
				args = append([]string{"-m"}, args...)
			}
			enc := &Context{Args: args, Stdin: bytes.NewReader(input), Stdout: &encoded, Stderr: &stderr}
			if code := runUUEncode(enc); code != 0 {
				t.Fatalf("encode: %d: %s", code, &stderr)
			}
			dec := &Context{Args: []string{"-o", "-"}, Stdin: strings.NewReader(encoded.String()), Stdout: &decoded, Stderr: &stderr}
			if code := runUUDecode(dec); code != 0 {
				t.Fatalf("decode: %d: %s", code, &stderr)
			}
			if !bytes.Equal(decoded.Bytes(), input) {
				t.Fatalf("base64=%v size=%d: round trip differs", base64, size)
			}
		}
	}
}
