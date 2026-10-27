package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultVMImage is the cloud image a code VM is prepared from unless [vm] image names another.
const DefaultVMImage = "https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2"

// VM configures the throwaway machines code tasks run their tools in.
type VM struct {
	// Image is a qcow2 cloud image with cloud-init, a URL or a local path.
	Image string `toml:"image"`
	// Packages are installed with pacman when the base is prepared.
	Packages []string `toml:"packages"`
	// Setup are shell commands run as root, in order, after the packages.
	Setup      []string `toml:"setup"`
	DiskGB     int      `toml:"disk_gb"`
	MemoryMB   int      `toml:"memory_mb"`
	CPUs       int      `toml:"cpus"`
	Network    *bool    `toml:"network"`
	MaxRunning int      `toml:"max_running"`
	// Helper is the ughbot binary copied into each VM; empty is the running one.
	Helper string `toml:"helper"`
}

func (v *VM) applyDefaults() {
	if v.Image == "" {
		v.Image = DefaultVMImage
	}
	if v.Packages == nil {
		v.Packages = []string{"git", "base-devel", "go", "ripgrep"}
	}
	if v.DiskGB == 0 {
		v.DiskGB = 40
	}
	if v.MemoryMB == 0 {
		v.MemoryMB = 4096
	}
	if v.CPUs == 0 {
		v.CPUs = 4
	}
	if v.MaxRunning == 0 {
		v.MaxRunning = 2
	}
	if v.Network == nil {
		on := true
		v.Network = &on
	}
	v.Helper = ExpandHome(v.Helper)
	if !strings.Contains(v.Image, "://") {
		v.Image = ExpandHome(v.Image)
	}
}

func (v VM) check(add func(string, ...any)) {
	if v.DiskGB < 4 || v.MemoryMB < 512 || v.CPUs < 1 || v.MaxRunning < 1 {
		add("vm: disk_gb ≥ 4, memory_mb ≥ 512, cpus ≥ 1 and max_running ≥ 1 are required")
	}
}

// VMDir holds the prepared base image, keys and running machines.
func (c *Config) VMDir() string { return filepath.Join(c.StateDir, "vm") }

// ReposDir holds the repositories cloned for bots once a person approved them.
func (c *Config) ReposDir() string { return filepath.Join(c.StateDir, "repos") }

// CheckCodeDir is the check a code directory in a bot's definition gets, for one granted later.
func (c *Config) CheckCodeDir(bot, dir string) error {
	var problems []string
	c.checkCodeDirs(bot, []string{dir}, func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) })
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// checkCodeDirs refuses directories whose sharing would hand a VM this host's secrets: the
// home directory or anything above it, and anything in or around ughbot's state (except a
// clone under its repos dir), its config or ~/.ssh.
func (c *Config) checkCodeDirs(bot string, dirs []string, add func(string, ...any)) {
	for _, d := range duplicates(dirs) {
		add("bot %q: duplicate code entry %q", bot, d)
	}
	for _, d := range dirs {
		if !filepath.IsAbs(d) {
			add("bot %q: code directory %q must be an absolute path", bot, d)
			continue
		}
		if within(home(), d) {
			add("bot %q: code directory %q would share the whole home directory with the VM", bot, d)
			continue
		}
		clone := within(d, c.ReposDir()) && d != c.ReposDir()
		for _, secret := range []string{c.StateDir, c.ConfigDir, filepath.Join(home(), ".ssh")} {
			if clone && secret == c.StateDir {
				continue
			}
			if within(secret, d) || within(d, secret) {
				add("bot %q: code directory %q would share %s with the VM", bot, d, secret)
				break
			}
		}
	}
}

// within reports whether path is dir or lies under it.
func within(path, dir string) bool {
	r, err := filepath.Rel(dir, path)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

// CodeDir is the entry of dirs that holds path, if any.
func CodeDir(dirs []string, path string) (string, error) {
	path = filepath.Clean(ExpandHome(path))
	for _, d := range dirs {
		if within(path, d) {
			return d, nil
		}
	}
	return "", fmt.Errorf("%s is not in a code directory (%s)", path, strings.Join(dirs, ", "))
}
