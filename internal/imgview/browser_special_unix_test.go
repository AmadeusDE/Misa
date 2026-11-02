//go:build unix

package imgview

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestBrowserRejectsFIFO(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pipe.png")
	if err := unix.Mkfifo(p, 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := CheckImageFile(p, Limits{}); ok || err == nil {
		t.Fatalf("FIFO error: %v %v", ok, err)
	}
	if IsImageFile(p, Limits{}) {
		t.Fatal("FIFO accepted")
	}
}
