package mweb

import (
	"os"
	"path/filepath"

	"github.com/AmadeusDE/Misa/internal/config"
)

// StateDir holds the device store: $XDG_STATE_HOME/misa/webui, else ~/.local/state/misa/webui.
func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "misa", "webui")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "misa", "webui")
}

// ConfigDir holds authorized_keys and the CLI device key (the directory of config.yaml).
func ConfigDir() string { return filepath.Dir(config.DefaultPath()) }

func authorizedKeysPath() string { return filepath.Join(ConfigDir(), "authorized_keys") }

func deviceKeyPath() string { return filepath.Join(ConfigDir(), "web-device.key") }

// securePrivateDir creates dir if needed and makes it 0700.
func securePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
