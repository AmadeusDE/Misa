package builtins

import (
	"path/filepath"
	"strings"
)

// findFullPath retains dot components, whose meaning can change across
// symlinks. filepath.Join would clean them before the OS resolves the path.
func findFullPath(base, path string) string {
	if base == "" || filepath.IsAbs(path) {
		return path
	}
	if strings.HasSuffix(base, string(filepath.Separator)) {
		return base + path
	}
	return base + string(filepath.Separator) + path
}
