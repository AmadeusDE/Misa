package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Entry struct {
	Title string  `yaml:"title"`
	Desc  string  `yaml:"desc"`
	Cmd   string  `yaml:"cmd,omitempty"`
	Items []Entry `yaml:"items,omitempty"`
}

const DefaultInfo = "version,time,terminal|user,cwd|ram,swap|uptime,kernel|git|localip"

type AsciiConfig struct {
	// Distro art: auto-detected from /etc/os-release by default.
	// Set to a name to force a specific logo (e.g. "arch", "cachyos").
	// Comma-separated or YAML list picks one at random.
	Distro string `yaml:"distro,omitempty"`

	// Anime art: "random" (default) picks one at random.
	// Set to a name (e.g. "rei", "asuka") to force one.
	// Comma-separated or YAML list picks one at random.
	Anime string `yaml:"anime,omitempty"`

	// Source for distro art: "pfetch", "neofetch", "fastfetch", or "dynamic_fallback" (default)
	AsciiType string `yaml:"ascii_type,omitempty"`

	// Custom directory to load additional distro ASCII files from
	AsciiDir string `yaml:"ascii_dir,omitempty"`

	// Theme for distro art colorization (e.g. "transgender", "rainbow", "default")
	// Comma-separated or YAML list picks one at random.
	Theme string `yaml:"theme,omitempty"`

	// Color distribution mode for distro art themes: "line" (default) or "char"
	ColorMode string `yaml:"color_mode,omitempty"`

	// Path to custom presets.json for color palettes
	PalettesFile string `yaml:"palettes_file,omitempty"`

	// Max lines to display for ASCII art (crops taller art). 0 = no limit.
	MaxHeight int `yaml:"max_height,omitempty"`
}

type StatusColors struct {
	Version  string `yaml:"version,omitempty"`
	Time     string `yaml:"time,omitempty"`
	Terminal string `yaml:"terminal,omitempty"`
	CWD      string `yaml:"cwd,omitempty"`
	RAM      string `yaml:"ram,omitempty"`
	Swap     string `yaml:"swap,omitempty"`
	Uptime   string `yaml:"uptime,omitempty"`
	Kernel   string `yaml:"kernel,omitempty"`
	Distro   string `yaml:"distro,omitempty"`
	Anime    string `yaml:"anime,omitempty"`
	Git      string `yaml:"git,omitempty"`
	LocalIP  string `yaml:"localip,omitempty"`
	User     string `yaml:"user,omitempty"`
}

type UIColors struct {
	Prompt      string `yaml:"prompt,omitempty"`
	PromptArrow string `yaml:"prompt_arrow,omitempty"`
	PromptOK    string `yaml:"prompt_ok,omitempty"`
	PromptErr   string `yaml:"prompt_err,omitempty"`
	Divider     string `yaml:"divider,omitempty"`
	Ghost       string `yaml:"ghost,omitempty"`
	Status      string `yaml:"status,omitempty"`
	Output      string `yaml:"output,omitempty"`
}

type StatusbarConfig struct {
	Info string `yaml:"info,omitempty"`
}

// WebConfig is the `web:` block read by `misa --web`; flags override it.
type WebConfig struct {
	Addr      string   `yaml:"addr,omitempty"`      // host:port, for development
	Interface string   `yaml:"interface,omitempty"` // e.g. wg0, for production
	Port      int      `yaml:"port,omitempty"`      // with interface, default 8787
	Command   []string `yaml:"command,omitempty"`   // program the browser terminal runs; default misa itself
}

type Config struct {
	Menu             []Entry           `yaml:"menu"`
	Ascii            AsciiConfig       `yaml:"ascii"`
	Colors           StatusColors      `yaml:"colors"`
	UI               UIColors          `yaml:"ui"`
	Statusbar        StatusbarConfig   `yaml:"statusbar"`
	Info             string            `yaml:"info,omitempty"`
	StartInShell     bool              `yaml:"start_in_shell,omitempty"`
	DisableMinishell bool              `yaml:"disable_minishell,omitempty"`
	HistorySearch    *bool             `yaml:"history_search,omitempty"`
	Aliases          map[string]string `yaml:"aliases,omitempty"`
	Autorun          []string          `yaml:"autorun,omitempty"`
	BuiltinMode      string            `yaml:"builtin_mode,omitempty"`
	JobControl       *bool             `yaml:"job_control,omitempty"`
	Web              WebConfig         `yaml:"web,omitempty"`
}

func BoolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func BoolPtr(b bool) *bool {
	return &b
}

func ActivePath() string {
	return findConfig()
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(dir, "misa", "config.yaml")
}

func EnsureExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(exampleConfig), 0o644)
}

const exampleConfig = `# Misa configuration — see config.example.yaml for full docs.
menu:
  - title: Apps
    desc: Terminal Applications
    items:
      - title: Lazygit
        desc: Launch lazygit in the current directory
        cmd: lazygit

ascii:
  # distro: auto
  anime: random
  # theme: default
  # max_height: 16
`

func findConfig() string { return findConfigFile("config.yaml") }

func Save(cfg Config) error {
	path := findConfig()
	if path == "" {
		path = DefaultPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Load() Config {
	path := findConfig()
	if path == "" {
		return defaults()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return defaults()
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "misa: config parse error in %s: %v\n", path, err)
		return defaults()
	}
	if len(cfg.Menu) == 0 {
		cfg.Menu = defaults().Menu
	}
	return cfg
}

func defaults() Config {
	return Config{
		Menu: []Entry{
			{
				Title: "Apps",
				Desc:  "Terminal Applications",
				Items: []Entry{
					{Title: "Antigravity-cli", Desc: "Launch agy in the current directory", Cmd: "agy"},
					{Title: "Claude-Code", Desc: "Launch claude in the current directory", Cmd: "claude"},
					{Title: "Lazygit", Desc: "Launch lazygit in the current directory", Cmd: "lazygit"},
				},
			},
		},
	}
}
