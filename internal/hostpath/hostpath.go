// Package hostpath is the one place that decides where Misa keeps its files on this host, who the user is, and how
// completion splits a typed path. Unix: config in os.UserConfigDir()/misa (XDG_CONFIG_HOME aware), data in
// ~/.local/share/misa. Windows: config in %APPDATA%\misa, data in %LOCALAPPDATA%\misa. The working directory is
// never searched.
package hostpath

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const app = "misa"

// ConfigDir is the per-user configuration directory, or "" when the host has no home.
func ConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, app)
}

// DataDir is the per-user data and history directory, or "" when the host has no home.
func DataDir() string {
	home, _ := os.UserHomeDir()
	return dataDir(runtime.GOOS, os.Getenv, home)
}

func dataDir(goos string, getenv func(string) string, home string) string {
	if goos == "windows" {
		if d := getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, app)
		}
		if home == "" {
			return ""
		}
		return filepath.Join(home, "AppData", "Local", app)
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", app)
}

// HistoryFile is Misa's own zsh-format history, or "" when the host has no home.
func HistoryFile() string { return inDataDir("history") }

// TldrDir is where downloaded tldr pages are kept.
func TldrDir() string { return inDataDir("tldr") }

func inDataDir(name string) string {
	d := DataDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, name)
}

// SearchDirs lists, in precedence order, the directories searched for a config file or asset directory:
// next to the executable, then the user configuration directory.
func SearchDirs(sub string) []string {
	var dirs []string
	if execPath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(execPath), sub))
	}
	if d := ConfigDir(); d != "" {
		dirs = append(dirs, filepath.Join(d, sub))
	}
	return dirs
}

// FindFile returns the first existing config file called name in SearchDirs, or "".
func FindFile(name string) string {
	for _, p := range SearchDirs(name) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Username is the login name: USER then LOGNAME on Unix; USERNAME then the account name on Windows.
func Username() string {
	return username(runtime.GOOS, os.Getenv, func() string {
		if u, err := user.Current(); err == nil {
			return u.Username
		}
		return ""
	})
}

func username(goos string, getenv func(string) string, current func() string) string {
	names := []string{"USER", "LOGNAME"}
	if goos == "windows" {
		names = []string{"USERNAME", "USER"}
	}
	for _, n := range names {
		if v := getenv(n); v != "" {
			return v
		}
	}
	if goos != "windows" {
		return ""
	}
	return accountName(goos, current())
}

// AccountName is the bare login name of an account name from os/user, which on Windows is DOMAIN\name.
func AccountName(name string) string { return accountName(runtime.GOOS, name) }

func accountName(goos, name string) string {
	if goos != "windows" {
		return name
	}
	return name[strings.LastIndexByte(name, '\\')+1:]
}

func isSep(c byte, windows bool) bool { return c == '/' || (windows && c == '\\') }

// SplitPath cuts p after its last separator into the directory part (separator kept) and the base. With windows
// set, `\` also separates and a bare drive ("C:") is a directory part.
func SplitPath(p string, windows bool) (dir, base string) {
	i := len(p) - 1
	for i >= 0 && !isSep(p[i], windows) {
		i--
	}
	if i < 0 && windows && len(p) >= 2 && isDrive(p) {
		return p[:2], p[2:]
	}
	return p[:i+1], p[i+1:]
}

func isDrive(p string) bool {
	c := p[0] | 0x20
	return c >= 'a' && c <= 'z' && p[1] == ':'
}

// IsAbs reports whether p names a place regardless of the working directory. With windows set that is a drive
// root (`C:\`, `C:/`) or a UNC path; a bare drive (`C:`) and a rooted path without a drive are not absolute.
func IsAbs(p string, windows bool) bool {
	if !windows {
		return strings.HasPrefix(p, "/")
	}
	if len(p) >= 3 && isDrive(p) && isSep(p[2], true) {
		return true
	}
	return len(p) >= 2 && isSep(p[0], true) && isSep(p[1], true)
}

// Separator is the separator completion appends to a directory: `\` only when windows is set and the typed path
// already used it.
func Separator(typed string, windows bool) string {
	if windows && strings.Contains(typed, `\`) {
		return `\`
	}
	return "/"
}
