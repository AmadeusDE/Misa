package uutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Conservatively treat non-option operands and path-bearing option values as
// filesystem paths for mutators only. Text-processing command inputs are not
// interpreted as paths. This is a private test tree, not a concurrency-safe
// sandbox against adversarial modifications.
func unsafeMutatorArgs(dir, cmd string, args []string) string {
	switch cmd {
	case "cp", "mv", "rm", "rmdir", "mkdir", "touch", "ln", "link", "unlink", "tee", "truncate", "chmod", "chown", "chgrp", "install", "mkfifo", "mknod", "shred", "dd", "split":
	default:
		return ""
	}
	options := true
	skipValue := false
	for _, arg := range args {
		if skipValue {
			skipValue = false
			continue
		}
		if options && ((cmd == "truncate" && (arg == "-s" || arg == "--size")) || (cmd == "touch" && (arg == "-d" || arg == "--date" || arg == "-t"))) {
			skipValue = true
			continue
		}
		if options && ((cmd == "truncate" && strings.HasPrefix(arg, "--size=")) || (cmd == "touch" && strings.HasPrefix(arg, "--date="))) {
			continue
		}
		if options && arg == "--" {
			options = false
			continue
		}
		path := arg
		if options && strings.HasPrefix(arg, "-") {
			if _, value, ok := strings.Cut(arg, "="); ok {
				path = value
			} else if strings.Contains(arg, "/") || strings.Contains(arg, "..") {
				return fmt.Sprintf("path-bearing option %q not permitted", arg)
			} else {
				continue
			}
		}
		// rm/unlink remove the last directory entry rather than following a
		// final symlink. A trailing slash or /. forces traversal instead.
		if (cmd == "rm" || cmd == "unlink") && path != "" && !strings.HasSuffix(path, "/") && !strings.HasSuffix(path, "/.") && !filepath.IsAbs(path) {
			full := filepath.Join(dir, path)
			info, err := os.Lstat(full)
			if err == nil && info.Mode()&os.ModeSymlink != 0 && isPathSafe(dir, filepath.Dir(path)) {
				continue
			}
		}
		if harmlessDevicePath(cmd, path) {
			continue
		}
		if !isPathSafe(dir, path) {
			return fmt.Sprintf("unsafe filesystem argument %q", arg)
		}
	}
	return ""
}

// harmlessDevicePath allows the standard stream devices and paths beneath
// /dev/null (which always fail with ENOTDIR) for commands that only read or
// write data through them. Removing or re-moding these is never allowed.
func harmlessDevicePath(cmd, path string) bool {
	switch cmd {
	case "cp", "tee", "dd":
	default:
		return false
	}
	switch path {
	case "/dev/null", "/dev/stdin", "/dev/stdout", "/dev/stderr", "/dev/fd/0", "/dev/fd/1", "/dev/fd/2":
		return true
	}
	return strings.HasPrefix(path, "/dev/null/")
}
