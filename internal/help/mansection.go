package help

import (
	"os"
	"path/filepath"
)

// FindManPageIn is FindManPage restricted to one section ("1", "5", "3p", ...). An empty section is any.
func FindManPageIn(command, section string) (string, bool) {
	if section == "" {
		return FindManPage(command)
	}
	if !validPageName(command) || !validPageName(section) {
		return "", false
	}
	for _, ext := range []string{".gz", ""} {
		path := filepath.Join("/usr/share/man", "man"+section[:1], command+"."+section+ext)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}
