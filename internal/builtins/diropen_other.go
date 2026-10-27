//go:build unix && !linux

package builtins

import "golang.org/x/sys/unix"

const dirOpenFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC
