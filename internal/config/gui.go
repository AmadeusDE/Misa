package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// GUIConfig configures the Wayland window frontend (misa --gui), gui.yaml.
type GUIConfig struct {
	// Font is a fontconfig pattern ("JetBrains Mono") or a font file path.
	Font     string  `yaml:"font,omitempty"`
	FontSize float64 `yaml:"font_size,omitempty"`
	Padding  *int    `yaml:"padding,omitempty"`
	Columns  int     `yaml:"columns,omitempty"`
	Rows     int     `yaml:"rows,omitempty"`
	// Foreground, Background and Cursor are hex colors; Palette is the 16
	// ANSI colors, black through bright white.
	Foreground string   `yaml:"foreground,omitempty"`
	Background string   `yaml:"background,omitempty"`
	Cursor     string   `yaml:"cursor,omitempty"`
	Palette    []string `yaml:"palette,omitempty"`
	// Opacity of the default background, 0 (clear) to 1 (solid); 0.55 when
	// unset. Text and colored backgrounds stay solid.
	Opacity *float64 `yaml:"opacity,omitempty"`
	// ClipboardWrite lets programs set the clipboard with OSC 52 (default
	// on, as in xterm); false ignores those requests.
	ClipboardWrite *bool `yaml:"clipboard_write,omitempty"`
}

func LoadGUI() GUIConfig {
	path := findConfigFile("gui.yaml")
	if path == "" {
		return GUIConfig{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return GUIConfig{}
	}
	var cfg GUIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "misa: gui config parse error in %s: %v\n", path, err)
		return GUIConfig{}
	}
	return cfg
}
