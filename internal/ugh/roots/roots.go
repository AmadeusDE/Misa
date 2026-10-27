// Package roots names the two directories an ugh instance owns: configuration
// (config.toml, allow.toml, credentials, model choices) and state (sessions,
// decision/cost/route logs, provider health). The zero value is plain ugh:
// $XDG_CONFIG_HOME/ugh and $XDG_STATE_HOME/ugh. A host that embeds ugh sets
// both explicitly so instances never read or write each other's files.
package roots

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Roots struct {
	Config string
	State  string
}

// Explicit reports whether the instance owns its directories, as opposed to
// sharing the user's ugh installation.
func (r Roots) Explicit() bool { return r.Config != "" || r.State != "" }

func (r Roots) ConfigDir() (string, error) {
	return resolve(r.Config, "XDG_CONFIG_HOME", ".config")
}

func (r Roots) StateDir() (string, error) {
	return resolve(r.State, "XDG_STATE_HOME", filepath.Join(".local", "state"))
}

func (r Roots) ConfigFile(name string) (string, error) {
	d, err := r.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

func (r Roots) StateFile(name string) (string, error) {
	d, err := r.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

func resolve(explicit, xdgVar, homeRel string) (string, error) {
	if explicit != "" {
		if !filepath.IsAbs(explicit) {
			return "", fmt.Errorf("roots: %q is not absolute", explicit)
		}
		return filepath.Clean(explicit), nil
	}
	base := os.Getenv(xdgVar)
	if !filepath.IsAbs(base) {
		home := os.Getenv("HOME")
		if !filepath.IsAbs(home) {
			return "", errors.New("roots: absolute HOME required")
		}
		base = filepath.Join(home, homeRel)
	}
	return filepath.Join(base, "ugh"), nil
}
