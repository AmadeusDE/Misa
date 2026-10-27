package gui

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/config"
)

func TestClipboardWriteOffIgnoresOSC52(t *testing.T) {
	off := false
	fb := newFakeBackend()
	a := &app{cfg: config.GUIConfig{ClipboardWrite: &off}, be: fb}
	a.handleOSC("52;c;aGVsbG8=")
	if fb.clip.writes != 0 {
		t.Fatalf("clipboard writes = %d", fb.clip.writes)
	}
}
