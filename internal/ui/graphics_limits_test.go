package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestPaneTerminalUsesGraphicsLimitsWithoutEnablingGraphics(t *testing.T) {
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	user, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(user, "misa")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "graphics.yaml"), []byte("kitty:\n  max_images: 7\n  max_payload: 1024\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command, ok := builtins.Lookup("true")
	if !ok {
		t.Fatal("missing true builtin")
	}
	m := &Model{dir: t.TempDir()}
	tm, _, err := m.startPlan(commandPlan{builtin: command, fields: []string{"true"}}, 2, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer tm.Close()
	limits := tm.GraphicsLimits()
	if limits.MaxImages != 7 || limits.MaxPayload != 1024 {
		t.Fatalf("configured limits not applied: %+v", limits)
	}
	if tm.GraphicsEnabled() {
		t.Fatal("config enabled graphics without a consumer")
	}
}
