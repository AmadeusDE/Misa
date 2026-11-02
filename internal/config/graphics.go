package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

// GraphicsConfig is graphics.yaml, the terminal graphics settings shared by
// every frontend.
type GraphicsConfig struct {
	Kitty KittyConfig `yaml:"kitty"`
}

// KittyConfig overrides the Kitty graphics limits. A zero or negative field
// means "use the default"; Validate replaces it and clamps the upper bounds.
// MiB fields are binary megabytes.
type KittyConfig struct {
	MaxWidth       int64 `yaml:"max_width"`
	MaxHeight      int64 `yaml:"max_height"`
	MaxImageMiB    int64 `yaml:"max_image_mib"`
	MaxTerminalMiB int64 `yaml:"max_terminal_mib"`
	MaxImages      int64 `yaml:"max_images"`
	MaxPlacements  int64 `yaml:"max_placements"`
	MaxInFlightMiB int64 `yaml:"max_in_flight_mib"`
	MaxPayload     int64 `yaml:"max_payload"`
}

const (
	maxKittyPixels  = 1 << 20
	maxKittyMiB     = 65536
	maxKittyCount   = 1_000_000
	maxKittyPayload = 16 << 20
)

// DefaultGraphics returns the graphics configuration Misa uses without a file.
func DefaultGraphics() GraphicsConfig {
	return GraphicsConfig{Kitty: defaultKitty()}
}

func defaultKitty() KittyConfig {
	l := kittygfx.DefaultLimits()
	return KittyConfig{
		MaxWidth:       int64(l.MaxWidth),
		MaxHeight:      int64(l.MaxHeight),
		MaxImageMiB:    ceilMiB(l.MaxImageBytes),
		MaxTerminalMiB: ceilMiB(l.MaxTerminalBytes),
		MaxImages:      int64(l.MaxImages),
		MaxPlacements:  int64(l.MaxPlacements),
		MaxInFlightMiB: ceilMiB(l.MaxInFlightBytes),
		MaxPayload:     int64(l.MaxPayload),
	}
}

// ceilMiB rounds up so a default is never smaller than the kittygfx one.
func ceilMiB(bytes int64) int64 { return (bytes + (1 << 20) - 1) >> 20 }

// Validate normalizes zero and negative fields to their defaults, clamps
// values above the safe maximum, and returns the changes as diagnostics.
func (c *GraphicsConfig) Validate() []string {
	return c.Kitty.normalize()
}

func (k *KittyConfig) normalize() []string {
	d := defaultKitty()
	var warnings []string
	for _, f := range []struct {
		name  string
		value *int64
		def   int64
		max   int64
	}{
		{"max_width", &k.MaxWidth, d.MaxWidth, maxKittyPixels},
		{"max_height", &k.MaxHeight, d.MaxHeight, maxKittyPixels},
		{"max_image_mib", &k.MaxImageMiB, d.MaxImageMiB, maxKittyMiB},
		{"max_terminal_mib", &k.MaxTerminalMiB, d.MaxTerminalMiB, maxKittyMiB},
		{"max_images", &k.MaxImages, d.MaxImages, maxKittyCount},
		{"max_placements", &k.MaxPlacements, d.MaxPlacements, maxKittyCount},
		{"max_in_flight_mib", &k.MaxInFlightMiB, d.MaxInFlightMiB, maxKittyMiB},
		{"max_payload", &k.MaxPayload, d.MaxPayload, maxKittyPayload},
	} {
		old := *f.value
		switch {
		case old < 0:
			*f.value = f.def
			warnings = append(warnings, fmt.Sprintf("%s: %d is negative; using the default %d", f.name, old, f.def))
		case old == 0:
			*f.value = f.def
		case old > f.max:
			*f.value = f.max
			warnings = append(warnings, fmt.Sprintf("%s: %d is above the maximum %d; using %d", f.name, old, f.max, f.max))
		}
	}
	return warnings
}

// Limits returns the validated kittygfx limits, so a KittyConfig built in
// code gets the same defaults and clamps as one loaded from a file.
func (k KittyConfig) Limits() kittygfx.Limits {
	k.normalize()
	return kittygfx.Limits{
		MaxWidth:         int(k.MaxWidth),
		MaxHeight:        int(k.MaxHeight),
		MaxImageBytes:    k.MaxImageMiB << 20,
		MaxTerminalBytes: k.MaxTerminalMiB << 20,
		MaxImages:        int(k.MaxImages),
		MaxPlacements:    int(k.MaxPlacements),
		MaxInFlightBytes: k.MaxInFlightMiB << 20,
		MaxPayload:       int(k.MaxPayload),
	}
}

// LoadGraphics loads the first graphics.yaml in the standard search path.
// Missing fields keep their defaults.
func LoadGraphics() GraphicsConfig {
	c := DefaultGraphics()
	path := findConfigFile("graphics.yaml")
	if path == "" {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		fmt.Fprintf(os.Stderr, "misa: graphics config parse error in %s: %v\n", path, err)
		return DefaultGraphics()
	}
	for _, warning := range c.Validate() {
		fmt.Fprintf(os.Stderr, "misa: graphics config %s: %s\n", path, warning)
	}
	return c
}
