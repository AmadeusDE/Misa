package mweb

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/AmadeusDE/Misa/internal/config"
)

// StateDir holds the device store: $XDG_STATE_HOME/misa/webui, else ~/.local/state/misa/webui.
func StateDir() string {
	// The web server and --web-admin are process-level entrypoints.
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

// invocationDirs resolves the builtin's paths without consulting process state.
func invocationDirs(getenv func(string) string) (state, configDir string) {
	home := getenv("HOME")
	stateHome := getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	state = filepath.Join(stateHome, "misa", "webui")
	// Match os.UserConfigDir, including its platform-specific fallback.
	switch runtime.GOOS {
	case "darwin", "ios":
		if home == "" {
			return state, "."
		}
		configDir = filepath.Join(home, "Library", "Application Support", "misa")
	case "windows":
		if dir := getenv("AppData"); dir != "" {
			configDir = filepath.Join(dir, "misa")
		} else {
			configDir = "."
		}
	case "plan9":
		if home == "" {
			configDir = "."
		} else {
			configDir = filepath.Join(home, "lib", "misa")
		}
	default:
		dir := getenv("XDG_CONFIG_HOME")
		if dir == "" && home != "" {
			dir = filepath.Join(home, ".config")
		}
		if dir == "" {
			configDir = "."
		} else {
			configDir = filepath.Join(dir, "misa")
		}
	}
	return state, configDir
}
