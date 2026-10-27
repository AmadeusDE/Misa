package gui

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/config"
)

func TestClipboardWriteOffIgnoresOSC52(t *testing.T) {
	off := false
	a := &app{cfg: config.GUIConfig{ClipboardWrite: &off}}
	a.clip.manager, a.clip.device = 1, 1 // a copy would reach the (nil) connection
	a.handleOSC("52;c;aGVsbG8=")
}
