//go:build linux

package builtins

import "golang.org/x/sys/unix"

// dirOpenFlags opens a directory only to walk from it: O_PATH needs no read
// permission, matching what chdir would allow.
const dirOpenFlags = unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC
