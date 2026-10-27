// Package config loads and validates ughbot.toml. It knows only "bot"; personas are plain config.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Spend struct {
	Daily   float64 `toml:"daily" json:"daily,omitempty"`
	Monthly float64 `toml:"monthly" json:"monthly,omitempty"`
}

type Bot struct {
	Name   string `toml:"-" json:"name"`
	Model  string `toml:"model" json:"model"`
	Prompt string `toml:"prompt" json:"prompt,omitempty"`
	// PromptText is the system prompt itself; it wins over the Prompt file. Bots made in the UI use it.
	PromptText   string       `toml:"prompt_text" json:"prompt_text,omitempty"`
	CompactAt    int          `toml:"compact_at" json:"compact_at,omitempty"`
	Spend        Spend        `toml:"spend" json:"spend"`
	Capabilities Capabilities `toml:"capabilities" json:"capabilities"`
	Workdir      string       `toml:"workdir" json:"workdir,omitempty"`
}

type Listen struct {
	Addr string `toml:"addr"`
	TLS  bool   `toml:"tls"`
}

// Web configures the browser UI. When enabled one of ListenInterface (production, e.g. a
// WireGuard interface) or ListenAddr (local testing) must be set.
type Web struct {
	Enabled         bool   `toml:"enabled"`
	ListenInterface string `toml:"listen_interface"`
	ListenAddr      string `toml:"listen_addr"`
	Port            int    `toml:"port"`
	// Command is the program the browser terminal runs (argv). Empty runs the ughbot TUI;
	// ["misa"] gives an enrolled device a full shell on this host.
	Command []string `toml:"command"`
}

type Config struct {
	StateDir  string         `toml:"state_dir"`
	ConfigDir string         `toml:"config_dir"`
	Socket    string         `toml:"socket"`
	Listen    Listen         `toml:"listen"`
	Web       Web            `toml:"web"`
	VM        VM             `toml:"vm"`
	Bots      map[string]Bot `toml:"bots"`
}

// Load reads and validates the file at path; relative prompt paths resolve against its directory.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if c.ConfigDir == DefaultConfigDir() {
		c.ConfigDir = filepath.Dir(path)
	}
	c.resolvePrompts()
	return c, nil
}

// Parse decodes TOML, applies defaults and validates; it does not touch the filesystem beyond $HOME.
func Parse(data []byte) (*Config, error) {
	var c Config
	md, err := toml.Decode(string(data), &c)
	if err != nil {
		return nil, err
	}
	if un := md.Undecoded(); len(un) > 0 {
		keys := make([]string, len(un))
		for i, k := range un {
			keys[i] = k.String()
		}
		return nil, fmt.Errorf("unknown keys: %s", strings.Join(keys, ", "))
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// BotNames lists configured bots in sorted order.
func (c *Config) BotNames() []string {
	names := make([]string, 0, len(c.Bots))
	for n := range c.Bots {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
