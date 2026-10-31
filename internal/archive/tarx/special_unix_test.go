//go:build unix

// Test derived from the unsupported-special-file requirement; Toybox reference
// tar.c and tar.test are attributed in NOTICE.
package tarx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSpecialFileSkipped(t *testing.T) {
	d := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(d, "fifo"), 0600); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(d, "ok"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	var failures int
	if err := Create(context.Background(), &b, []Source{{d, "."}}, CreateOptions{OnError: func(string, error) { failures++ }}); err != nil {
		t.Fatal(err)
	}
	if failures != 1 {
		t.Fatalf("got %d failures", failures)
	}
}
