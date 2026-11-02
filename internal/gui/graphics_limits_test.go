package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGUITerminalUsesGraphicsLimitsWithoutEnablingGraphics(t *testing.T) {
	if runtime.GOOS == "android" {
		t.Skip("starting a PTY child via os.Args[0] is not supported on the Android emulator")
	}
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("COLORTERM", "")
	user, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(user, "misa")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "graphics.yaml"), []byte("kitty:\n  max_images: 9\n  max_payload: 2048\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, newFakeBackend())
	a.argv = []string{os.Args[0], "-test.run=^$"}
	a.startTerminal(2, 8)
	if a.term == nil {
		t.Fatal("terminal did not start")
	}
	defer a.term.Close()
	limits := a.term.GraphicsLimits()
	if limits.MaxImages != 9 || limits.MaxPayload != 2048 {
		t.Fatalf("configured limits not applied: %+v", limits)
	}
	if a.term.GraphicsEnabled() {
		t.Fatal("config enabled graphics without a compositor")
	}
}
