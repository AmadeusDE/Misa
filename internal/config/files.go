package config

import (
	"os"
	"path/filepath"
)

// FindFile is findConfigFile for packages that keep their own config file:
// next to the executable, then ~/.config/misa, never the working directory.
func FindFile(name string) string { return findConfigFile(name) }

// UserFile is where a config file is created when none exists yet.
func UserFile(name string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return name
	}
	return filepath.Join(dir, "misa", name)
}
