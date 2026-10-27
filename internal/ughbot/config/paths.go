package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func xdg(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	return filepath.Join(home(), fallback)
}

func DefaultConfigDir() string { return filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), "ughbot") }
func DefaultStateDir() string  { return filepath.Join(xdg("XDG_STATE_HOME", ".local/state"), "ughbot") }

// DefaultSocket is $XDG_RUNTIME_DIR/ughbot.sock, falling back to the state directory.
func DefaultSocket() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "ughbot.sock")
	}
	return filepath.Join(DefaultStateDir(), "ughbot.sock")
}

// DefaultPath is the config file location.
func DefaultPath() string { return filepath.Join(DefaultConfigDir(), "ughbot.toml") }

// ExpandHome expands a leading ~ to the home directory.
func ExpandHome(p string) string {
	if p == "~" {
		return home()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home(), p[2:])
	}
	return p
}

func (c *Config) applyDefaults() {
	if c.StateDir == "" {
		c.StateDir = DefaultStateDir()
	}
	if c.ConfigDir == "" {
		c.ConfigDir = DefaultConfigDir()
	}
	if c.Socket == "" {
		c.Socket = DefaultSocket()
	}
	c.StateDir, c.ConfigDir, c.Socket = ExpandHome(c.StateDir), ExpandHome(c.ConfigDir), ExpandHome(c.Socket)
	c.VM.applyDefaults()
	for name, b := range c.Bots {
		b.Name = name
		c.Bots[name] = normalizeBot(b)
	}
}

func normalizeBot(b Bot) Bot {
	b.Workdir = ExpandHome(b.Workdir)
	b.Prompt = ExpandHome(b.Prompt)
	b.Capabilities.Code = slices.Clone(b.Capabilities.Code)
	for i, d := range b.Capabilities.Code {
		b.Capabilities.Code[i] = filepath.Clean(ExpandHome(d))
	}
	return b
}

// With returns a copy of the config whose bots include b (replacing a bot of that name),
// or without b when remove is set; err lists everything wrong with the resulting set,
// including bots that b names but that do not exist.
func (c *Config) With(b Bot, remove bool) (*Config, error) {
	n := *c
	n.Bots = make(map[string]Bot, len(c.Bots)+1)
	for k, v := range c.Bots {
		n.Bots[k] = v
	}
	if remove {
		delete(n.Bots, b.Name)
	} else {
		b = normalizeBot(b)
		if b.Prompt != "" && !filepath.IsAbs(b.Prompt) {
			b.Prompt = filepath.Join(c.ConfigDir, b.Prompt)
		}
		n.Bots[b.Name] = b
	}
	if err := n.Validate(); err != nil || remove {
		return &n, err
	}
	return &n, n.CheckReferences(b.Name)
}

func (c *Config) resolvePrompts() {
	for name, b := range c.Bots {
		if b.Prompt != "" && !filepath.IsAbs(b.Prompt) {
			b.Prompt = filepath.Join(c.ConfigDir, b.Prompt)
		}
		c.Bots[name] = b
	}
}

// DBPath is the SQLite file under the state directory.
func (c *Config) DBPath() string { return filepath.Join(c.StateDir, "ughbot.db") }

// SessionDir is where a bot's ugh sessions live.
func (c *Config) SessionDir(bot string) string { return filepath.Join(c.StateDir, "bots", bot) }
