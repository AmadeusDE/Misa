// Behaviour adapted from toybox uudecode.c (0BSD).
package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUUDecodeHeaderFile(t *testing.T) {
	skipOnWindows(t, "Windows keeps no permission bits")
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	ctx := &Context{Dir: dir, Stdin: strings.NewReader("preamble\nbegin 600 decoded name\n#86)C\nend\n"), Stdout: &out, Stderr: &stderr}
	if code := runUUDecode(ctx); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "decoded name"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "abc" {
		t.Fatalf("got %q", data)
	}
	info, err := os.Stat(filepath.Join(dir, "decoded name"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected stdout %q", &out)
	}
}

func TestUUDecodePadding(t *testing.T) {
	for _, line := range []string{"!80  ", "!80", "!80``"} {
		var out, stderr bytes.Buffer
		ctx := &Context{Args: []string{"-o", "-"}, Stdin: strings.NewReader("begin 644 test\n" + line + "\nend\n"), Stdout: &out, Stderr: &stderr}
		if code := runUUDecode(ctx); code != 0 || out.String() != "a" {
			t.Fatalf("%q: exit %d output %q stderr %s", line, code, &out, &stderr)
		}
	}
}

func TestUUDecodeMissingHeader(t *testing.T) {
	var out, stderr bytes.Buffer
	ctx := &Context{Args: []string{"-o", "-"}, Stdin: strings.NewReader("not encoded\n"), Stdout: &out, Stderr: &stderr}
	if code := runUUDecode(ctx); code == 0 || !strings.Contains(stderr.String(), "no begin line") {
		t.Fatalf("exit %d stderr %s", code, &stderr)
	}
}
