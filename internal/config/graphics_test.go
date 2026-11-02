package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

func TestDefaultGraphicsUsesKittyDefaults(t *testing.T) {
	if got, want := DefaultGraphics().Kitty.Limits(), kittygfx.DefaultLimits(); got != want {
		t.Fatalf("DefaultGraphics().Kitty.Limits() = %+v, want %+v", got, want)
	}
}

func TestGraphicsValidateZeroUsesDefaultsWithoutWarning(t *testing.T) {
	c := GraphicsConfig{}
	if w := c.Validate(); len(w) != 0 {
		t.Fatalf("Validate() warnings = %v, want none", w)
	}
	if got, want := c.Kitty.Limits(), kittygfx.DefaultLimits(); got != want {
		t.Fatalf("Limits() = %+v, want %+v", got, want)
	}
}

func TestGraphicsValidateNegativeUsesDefaults(t *testing.T) {
	c := GraphicsConfig{Kitty: KittyConfig{
		MaxWidth: -1, MaxHeight: -2, MaxImageMiB: -3, MaxTerminalMiB: -4,
		MaxImages: -5, MaxPlacements: -6, MaxInFlightMiB: -7, MaxPayload: -8,
	}}
	w := c.Validate()
	if len(w) != 8 {
		t.Fatalf("Validate() warnings = %v, want one per field", w)
	}
	for _, name := range []string{"max_width", "max_height", "max_image_mib", "max_terminal_mib", "max_images", "max_placements", "max_in_flight_mib", "max_payload"} {
		if !hasWarning(w, name) {
			t.Errorf("Validate() warnings %v do not mention %q", w, name)
		}
	}
	if got, want := c.Kitty.Limits(), kittygfx.DefaultLimits(); got != want {
		t.Fatalf("Limits() = %+v, want %+v", got, want)
	}
}

func TestKittyLimitsClampOutOfRange(t *testing.T) {
	tests := []struct {
		name string
		in   KittyConfig
		want func(*kittygfx.Limits)
	}{
		{"max_width", KittyConfig{MaxWidth: 1 << 21}, func(l *kittygfx.Limits) { l.MaxWidth = 1 << 20 }},
		{"max_height", KittyConfig{MaxHeight: 1 << 21}, func(l *kittygfx.Limits) { l.MaxHeight = 1 << 20 }},
		{"max_image_mib", KittyConfig{MaxImageMiB: 1 << 20}, func(l *kittygfx.Limits) { l.MaxImageBytes = 65536 << 20 }},
		{"max_terminal_mib", KittyConfig{MaxTerminalMiB: 1 << 20}, func(l *kittygfx.Limits) { l.MaxTerminalBytes = 65536 << 20 }},
		{"max_images", KittyConfig{MaxImages: 1 << 30}, func(l *kittygfx.Limits) { l.MaxImages = 1_000_000 }},
		{"max_placements", KittyConfig{MaxPlacements: 1 << 30}, func(l *kittygfx.Limits) { l.MaxPlacements = 1_000_000 }},
		{"max_in_flight_mib", KittyConfig{MaxInFlightMiB: 1 << 20}, func(l *kittygfx.Limits) { l.MaxInFlightBytes = 65536 << 20 }},
		{"max_payload", KittyConfig{MaxPayload: 1 << 40}, func(l *kittygfx.Limits) { l.MaxPayload = 16 << 20 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.Limits()
			want := kittygfx.DefaultLimits()
			tt.want(&want)
			if got != want {
				t.Fatalf("Limits() = %+v, want %+v", got, want)
			}
			c := GraphicsConfig{Kitty: tt.in}
			if w := c.Validate(); !hasWarning(w, tt.name) {
				t.Errorf("Validate() warnings = %v, want one about %q", w, tt.name)
			}
		})
	}
}

func TestKittyLimitsAllowRaisingDefaults(t *testing.T) {
	c := GraphicsConfig{Kitty: KittyConfig{
		MaxWidth: 1 << 20, MaxHeight: 1 << 20, MaxImageMiB: 65536, MaxTerminalMiB: 65536,
		MaxImages: 1_000_000, MaxPlacements: 1_000_000, MaxInFlightMiB: 65536, MaxPayload: 16 << 20,
	}}
	if w := c.Validate(); len(w) != 0 {
		t.Fatalf("Validate() warnings = %v, want none", w)
	}
	want := kittygfx.Limits{
		MaxWidth: 1 << 20, MaxHeight: 1 << 20, MaxImageBytes: 65536 << 20,
		MaxTerminalBytes: 65536 << 20, MaxImages: 1_000_000, MaxPlacements: 1_000_000,
		MaxInFlightBytes: 65536 << 20, MaxPayload: 16 << 20,
	}
	if got := c.Kitty.Limits(); got != want {
		t.Fatalf("Limits() = %+v, want %+v", got, want)
	}
}

func TestKittyLimitsAboveDefaultWithinRange(t *testing.T) {
	got := (KittyConfig{MaxImageMiB: 1024, MaxPayload: 64 << 10}).Limits()
	if got.MaxImageBytes != 1024<<20 {
		t.Errorf("MaxImageBytes = %d, want %d", got.MaxImageBytes, 1024<<20)
	}
	if got.MaxPayload != 64<<10 {
		t.Errorf("MaxPayload = %d, want %d", got.MaxPayload, 64<<10)
	}
}

func TestLoadGraphicsUserConfig(t *testing.T) {
	dir := hermeticConfigDir(t)
	writeGraphicsConfig(t, dir, `
kitty:
  max_width: 8192
  max_height: 4096
  max_image_mib: 1024
  max_terminal_mib: 2048
  max_images: 4
  max_placements: 8
  max_in_flight_mib: 64
  max_payload: 65536
`)
	want := kittygfx.Limits{
		MaxWidth: 8192, MaxHeight: 4096, MaxImageBytes: 1024 << 20, MaxTerminalBytes: 2048 << 20,
		MaxImages: 4, MaxPlacements: 8, MaxInFlightBytes: 64 << 20, MaxPayload: 65536,
	}
	if got := LoadGraphics().Kitty.Limits(); got != want {
		t.Fatalf("LoadGraphics().Kitty.Limits() = %+v, want %+v", got, want)
	}
}

func TestLoadGraphicsKeepsDefaultsForMissingFields(t *testing.T) {
	dir := hermeticConfigDir(t)
	writeGraphicsConfig(t, dir, "kitty:\n  max_images: 4\n")
	want := kittygfx.DefaultLimits()
	want.MaxImages = 4
	if got := LoadGraphics().Kitty.Limits(); got != want {
		t.Fatalf("LoadGraphics().Kitty.Limits() = %+v, want %+v", got, want)
	}
}

func TestLoadGraphicsNegativeUsesDefault(t *testing.T) {
	dir := hermeticConfigDir(t)
	writeGraphicsConfig(t, dir, "kitty:\n  max_images: -3\n  max_payload: -1\n")
	if got, want := LoadGraphics().Kitty.Limits(), kittygfx.DefaultLimits(); got != want {
		t.Fatalf("LoadGraphics().Kitty.Limits() = %+v, want %+v", got, want)
	}
}

func TestLoadGraphicsInvalidYAMLUsesDefaults(t *testing.T) {
	dir := hermeticConfigDir(t)
	writeGraphicsConfig(t, dir, "kitty: [oops\n")
	if got, want := LoadGraphics().Kitty.Limits(), kittygfx.DefaultLimits(); got != want {
		t.Fatalf("LoadGraphics().Kitty.Limits() = %+v, want %+v", got, want)
	}
}

func TestLoadGraphicsIgnoresWorkingDirectory(t *testing.T) {
	hermeticConfigDir(t)
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.WriteFile(filepath.Join(cwd, "graphics.yaml"), []byte("kitty:\n  max_images: 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := LoadGraphics().Kitty.Limits(), kittygfx.DefaultLimits(); got != want {
		t.Fatalf("LoadGraphics().Kitty.Limits() = %+v, want %+v", got, want)
	}
}

func hasWarning(warnings []string, name string) bool {
	for _, w := range warnings {
		if strings.Contains(w, name) {
			return true
		}
	}
	return false
}

// hermeticConfigDir points the user config search at an empty temporary
// directory and returns the misa directory inside it.
func hermeticConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	return filepath.Join(dir, "misa")
}

func writeGraphicsConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "graphics.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
