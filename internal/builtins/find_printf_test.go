package builtins

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type findPrintfBadWriter struct{}

func (findPrintfBadWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestFindPrintfValidation(t *testing.T) {
	for _, format := range []string{"%p %f %h %P %s %m %M %u %g %t %T@ %y %d %%\\n", "literal\\tvalue"} {
		if _, err := findPrintf(format); err != nil {
			t.Errorf("%q: %v", format, err)
		}
	}
	for _, format := range []string{"%", "%Q", "%T", "\\"} {
		if _, err := findPrintf(format); err == nil {
			t.Errorf("accepted %q", format)
		}
	}
}
func TestFindPrintfOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("abc"), 0640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Context{Stdout: &out}
	e := &findEntry{ctx: ctx, path: "./file", root: ".", info: info, depth: 1}
	n, err := findPrintf("%p|%f|%h|%P|%s|%m|%M|%y|%d|%%\\n")
	if err != nil || !n(e) {
		t.Fatalf("%v %v", err, e.err)
	}
	if got, want := out.String(), "./file|file|.|file|3|640|-rw-r-----|f|1|%\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	ctx.Stdout = findPrintfBadWriter{}
	if n(e) || e.err == nil || !strings.Contains(e.err.Error(), "write failed") {
		t.Fatal("write error was not propagated")
	}
}

func TestFindPrintfNegativeTimestamp(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f")
	if err := os.WriteFile(f, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(-1, 500000000)
	if err := os.Chtimes(f, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	e := &findEntry{ctx: &Context{Stdout: &out}, info: info}
	n, err := findPrintf("%T@")
	if err != nil || !n(e) {
		t.Fatalf("%v %v", err, e.err)
	}
	if out.String() != "-0.5000000000" {
		t.Fatalf("got %q", out.String())
	}
}
